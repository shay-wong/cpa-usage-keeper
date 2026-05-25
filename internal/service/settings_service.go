package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cpa-usage-keeper/internal/backup"
	"cpa-usage-keeper/internal/config"
	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository"
	repodto "cpa-usage-keeper/internal/repository/dto"
	servicedto "cpa-usage-keeper/internal/service/dto"
	"gorm.io/gorm"
)

type DatabaseCleanupSettingsSnapshot struct {
	Settings                 entities.DatabaseCleanupSettings
	CurrentDatabaseSizeBytes *int64
}

type DatabaseSettingsProvider interface {
	GetDatabaseCleanupSettings(context.Context) (DatabaseCleanupSettingsSnapshot, error)
	UpdateDatabaseCleanupSettings(context.Context, servicedto.UpdateDatabaseCleanupSettingsInput) (DatabaseCleanupSettingsSnapshot, error)
	GetStorageInfo(context.Context) (servicedto.StorageInfo, error)
	CreateBackup(context.Context, servicedto.CreateBackupInput) (servicedto.BackupOperationResult, error)
	RestoreBackup(context.Context, servicedto.RestoreBackupInput) (servicedto.RestoreOperationResult, error)
}

var (
	ErrStorageSettingsInvalid     = errors.New("storage settings invalid")
	ErrStorageBackupDomainNeeded  = errors.New("storage backup domain required")
	ErrStorageRestoreDomainNeeded = errors.New("storage restore domain required")
	ErrStorageBackupNotFound      = errors.New("storage backup not found")
	ErrDatabaseBackupsUnsupported = errors.New("database backups unsupported")
)

type databaseSettingsService struct {
	db          *gorm.DB
	sqlDB       *sql.DB
	sqlDBErr    error
	sqlitePath  string
	backupDir   string
	databaseURL string
}

func NewDatabaseSettingsService(db *gorm.DB, sqlitePath string) DatabaseSettingsProvider {
	backupDir := filepath.Join(filepath.Dir(strings.TrimSpace(sqlitePath)), "backups")
	return NewDatabaseSettingsServiceWithBackupDir(db, sqlitePath, backupDir)
}

func NewDatabaseSettingsServiceWithBackupDir(db *gorm.DB, sqlitePath string, backupDir string) DatabaseSettingsProvider {
	return NewDatabaseSettingsServiceWithBackupDirAndDatabaseURL(db, sqlitePath, backupDir, "")
}

func NewDatabaseSettingsServiceWithBackupDirAndDatabaseURL(db *gorm.DB, sqlitePath string, backupDir string, databaseURL string) DatabaseSettingsProvider {
	var sqlDB *sql.DB
	var sqlDBErr error
	if db != nil {
		sqlDB, sqlDBErr = db.DB()
	}
	return &databaseSettingsService{db: db, sqlDB: sqlDB, sqlDBErr: sqlDBErr, sqlitePath: sqlitePath, backupDir: backupDir, databaseURL: strings.TrimSpace(databaseURL)}
}

func (s *databaseSettingsService) GetDatabaseCleanupSettings(context.Context) (DatabaseCleanupSettingsSnapshot, error) {
	settings, err := repository.GetDatabaseCleanupSettings(s.db)
	if err != nil {
		return DatabaseCleanupSettingsSnapshot{}, err
	}
	snapshot := DatabaseCleanupSettingsSnapshot{Settings: settings}
	sizeBytes, ok, err := repository.GetDatabaseSizeBytes(s.db, s.sqlitePath)
	if err != nil {
		return DatabaseCleanupSettingsSnapshot{}, err
	}
	if ok {
		snapshot.CurrentDatabaseSizeBytes = &sizeBytes
	}
	return snapshot, nil
}

func (s *databaseSettingsService) UpdateDatabaseCleanupSettings(ctx context.Context, input servicedto.UpdateDatabaseCleanupSettingsInput) (DatabaseCleanupSettingsSnapshot, error) {
	if err := validateStorageSettings(input); err != nil {
		return DatabaseCleanupSettingsSnapshot{}, err
	}
	if _, err := repository.UpsertDatabaseCleanupSettings(s.db, repodto.DatabaseCleanupSettingsInput(input)); err != nil {
		return DatabaseCleanupSettingsSnapshot{}, err
	}
	return s.GetDatabaseCleanupSettings(ctx)
}

func (s *databaseSettingsService) GetStorageInfo(context.Context) (servicedto.StorageInfo, error) {
	settings, err := repository.GetDatabaseCleanupSettings(s.db)
	if err != nil {
		return servicedto.StorageInfo{}, err
	}
	domains, err := repository.GetStorageDomainStats(s.db)
	if err != nil {
		return servicedto.StorageInfo{}, err
	}
	backups, err := s.listBackups()
	if err != nil {
		return servicedto.StorageInfo{}, err
	}
	backupSizeBytes, err := backup.DirectorySize(s.backupDir)
	if err != nil {
		return servicedto.StorageInfo{}, err
	}
	info := servicedto.StorageInfo{
		Settings:                   settingsToInput(settings),
		BackupTotalSizeBytes:       backupSizeBytes,
		BackupCount:                len(backups),
		DatabaseBackupsSupported:   s.databaseBackupsSupported(),
		SQLiteFileBackupsSupported: s.sqliteFileBackupsSupported(),
		Domains:                    mapStorageDomains(domains),
		Backups:                    backups,
	}
	if sizeBytes, ok, err := repository.GetDatabaseSizeBytes(s.db, s.sqlitePath); err != nil {
		return servicedto.StorageInfo{}, err
	} else if ok {
		info.CurrentDatabaseSizeBytes = &sizeBytes
	}
	return info, nil
}

func (s *databaseSettingsService) databaseBackupsSupported() bool {
	return s != nil && (s.sqliteFileBackupsSupported() || s.postgresBackupsSupported())
}

func (s *databaseSettingsService) sqliteFileBackupsSupported() bool {
	return s != nil && s.db != nil && s.db.Dialector.Name() == config.DatabaseDriverSQLite
}

func (s *databaseSettingsService) postgresBackupsSupported() bool {
	return s != nil && s.db != nil && s.db.Dialector.Name() == config.DatabaseDriverPostgres && strings.TrimSpace(s.databaseURL) != "" && backup.PostgresToolsAvailable()
}

func (s *databaseSettingsService) CreateBackup(ctx context.Context, input servicedto.CreateBackupInput) (servicedto.BackupOperationResult, error) {
	settings, err := repository.GetDatabaseCleanupSettings(s.db)
	if err != nil {
		return servicedto.BackupOperationResult{}, err
	}
	options, err := storageBackupOptions(input, settings)
	if err != nil {
		return servicedto.BackupOperationResult{}, err
	}

	var path string
	switch {
	case s.sqliteFileBackupsSupported():
		if s.sqlDBErr != nil {
			return servicedto.BackupOperationResult{}, fmt.Errorf("load sql database handle: %w", s.sqlDBErr)
		}
		if s.sqlDB == nil {
			return servicedto.BackupOperationResult{}, fmt.Errorf("database is required")
		}
		path, err = backup.NewWriter(s.backupDir).WriteDatabaseWithOptions(ctx, s.sqlDB, time.Now(), options)
	case s.postgresBackupsSupported():
		path, err = backup.NewPostgresWriter(s.backupDir, s.databaseURL).WriteDatabaseWithOptions(ctx, time.Now(), options)
	default:
		return servicedto.BackupOperationResult{}, fmt.Errorf("%w: database backups are not available for current database driver", ErrDatabaseBackupsUnsupported)
	}
	if err != nil {
		return servicedto.BackupOperationResult{}, err
	}
	if settings.MaxBackupCount > 0 {
		if _, err := backup.CleanupByMaxCount(s.backupDir, settings.MaxBackupCount); err != nil {
			return servicedto.BackupOperationResult{}, err
		}
	}
	file, err := backupFileInfo(path)
	if err != nil {
		return servicedto.BackupOperationResult{}, err
	}
	return servicedto.BackupOperationResult{Backup: file}, nil
}

func storageBackupOptions(input servicedto.CreateBackupInput, settings entities.DatabaseCleanupSettings) (backup.Options, error) {
	requestLogs := input.RequestLogs
	usageLogs := input.UsageLogs
	usageIdentities := input.UsageIdentities
	apiKeys := input.APIKeys
	redisInbox := input.RedisInbox
	modelPrices := input.ModelPrices
	if !storageBackupDomainSelected(requestLogs, usageLogs, usageIdentities, apiKeys, redisInbox, modelPrices) {
		requestLogs = settings.BackupRequestLogs
		usageLogs = settings.BackupUsageLogs
		usageIdentities = settings.BackupUsageIdentities
		apiKeys = settings.BackupAPIKeys
		redisInbox = settings.BackupRedisInbox
		modelPrices = settings.BackupModelPrices
	}
	if !storageBackupDomainSelected(requestLogs, usageLogs, usageIdentities, apiKeys, redisInbox, modelPrices) {
		return backup.Options{}, fmt.Errorf("%w: at least one backup domain is required", ErrStorageBackupDomainNeeded)
	}
	return backup.Options{RequestLogs: requestLogs, UsageLogs: usageLogs, UsageIdentities: usageIdentities, APIKeys: apiKeys, RedisInbox: redisInbox, ModelPrices: modelPrices}, nil
}

func storageBackupDomainSelected(values ...bool) bool {
	for _, value := range values {
		if value {
			return true
		}
	}
	return false
}

func (s *databaseSettingsService) createRestoreSafetyBackup(ctx context.Context) error {
	switch {
	case s.sqliteFileBackupsSupported():
		if s.sqlDBErr != nil {
			return fmt.Errorf("load sql database handle: %w", s.sqlDBErr)
		}
		if s.sqlDB == nil {
			return fmt.Errorf("database is required")
		}
		_, err := backup.NewWriter(s.backupDir).WriteRestoreSafetyBackup(ctx, s.sqlDB, time.Now())
		return err
	case s.postgresBackupsSupported():
		_, err := backup.NewPostgresWriter(s.backupDir, s.databaseURL).WriteRestoreSafetyBackup(ctx, time.Now())
		return err
	default:
		return fmt.Errorf("%w: database backups are not available for current database driver", ErrDatabaseBackupsUnsupported)
	}
}

func (s *databaseSettingsService) RestoreBackup(ctx context.Context, input servicedto.RestoreBackupInput) (servicedto.RestoreOperationResult, error) {
	path, err := s.backupPathByID(input.ID)
	if err != nil {
		return servicedto.RestoreOperationResult{}, err
	}
	selection := repository.StorageDomainSelection{
		RequestLogs:     input.RequestLogs,
		UsageLogs:       input.UsageLogs,
		UsageIdentities: input.UsageIdentities,
		APIKeys:         input.APIKeys,
		RedisInbox:      input.RedisInbox,
		ModelPrices:     input.ModelPrices,
	}
	if !selection.AnyEnabled() {
		return servicedto.RestoreOperationResult{}, fmt.Errorf("%w: at least one restore domain is required", ErrStorageRestoreDomainNeeded)
	}
	if !input.SkipSafetyBackup {
		if err := s.createRestoreSafetyBackup(ctx); err != nil {
			return servicedto.RestoreOperationResult{}, fmt.Errorf("create safety backup before restore: %w", err)
		}
	}
	switch {
	case s.sqliteFileBackupsSupported():
		err = repository.RestoreStorageDomains(ctx, s.db, path, selection, time.Now())
	case s.postgresBackupsSupported():
		err = s.restorePostgresBackup(ctx, path, selection)
	default:
		err = fmt.Errorf("%w: database restore is not available for current database driver", ErrDatabaseBackupsUnsupported)
	}
	if err != nil {
		return servicedto.RestoreOperationResult{}, err
	}
	return servicedto.RestoreOperationResult{
		RestoredRequestLogs:     input.RequestLogs,
		RestoredUsageLogs:       input.UsageLogs,
		RestoredUsageIdentities: input.UsageIdentities,
		RestoredAPIKeys:         input.APIKeys,
		RestoredRedisInbox:      input.RedisInbox,
		RestoredModelPrices:     input.ModelPrices,
	}, nil
}

func (s *databaseSettingsService) restorePostgresBackup(ctx context.Context, path string, selection repository.StorageDomainSelection) error {
	options := backup.Options{
		RequestLogs:     selection.RequestLogs,
		UsageLogs:       selection.UsageLogs,
		UsageIdentities: selection.UsageIdentities,
		APIKeys:         selection.APIKeys,
		RedisInbox:      selection.RedisInbox,
		ModelPrices:     selection.ModelPrices,
	}
	if len(backup.PostgresTablesForOptions(options)) == 0 {
		return fmt.Errorf("%w: at least one restore domain is required", ErrStorageRestoreDomainNeeded)
	}
	if err := backup.RestorePostgresDatabase(ctx, s.databaseURL, path, options); err != nil {
		return err
	}
	if selection.UsageLogs {
		return repository.RebuildUsageDerivedStats(ctx, s.db, time.Now())
	}
	return nil
}

func settingsToInput(settings entities.DatabaseCleanupSettings) servicedto.UpdateDatabaseCleanupSettingsInput {
	return servicedto.UpdateDatabaseCleanupSettingsInput{
		RecordRequestDetails:    settings.RecordRequestDetails,
		CleanupRequestLogs:      settings.CleanupRequestLogs,
		CleanupUsageLogs:        settings.CleanupUsageLogs,
		RequestLogRetentionDays: settings.RequestLogRetentionDays,
		UsageLogRetentionDays:   settings.UsageLogRetentionDays,
		MaxDatabaseSizeMB:       settings.MaxDatabaseSizeMB,
		BackupRequestLogs:       settings.BackupRequestLogs,
		BackupUsageLogs:         settings.BackupUsageLogs,
		BackupUsageIdentities:   settings.BackupUsageIdentities,
		BackupAPIKeys:           settings.BackupAPIKeys,
		BackupRedisInbox:        settings.BackupRedisInbox,
		BackupModelPrices:       settings.BackupModelPrices,
		BackupHour:              settings.BackupHour,
		BackupMinute:            settings.BackupMinute,
		MaxBackupCount:          settings.MaxBackupCount,
	}
}

func mapStorageDomains(domains []repository.StorageDomainStats) []servicedto.StorageDomainInfo {
	items := make([]servicedto.StorageDomainInfo, 0, len(domains))
	for _, domain := range domains {
		items = append(items, servicedto.StorageDomainInfo{
			Key:         domain.Key,
			Label:       domain.Label,
			Description: domain.Description,
			TableNames:  domain.TableNames,
			Rows:        domain.Rows,
			SizeBytes:   domain.SizeBytes,
		})
	}
	return items
}

func (s *databaseSettingsService) listBackups() ([]servicedto.BackupFileInfo, error) {
	paths, err := backup.ListFiles(s.backupDir)
	if err != nil {
		return nil, err
	}
	paths = s.filterBackupPathsForCurrentDriver(paths)
	items := make([]servicedto.BackupFileInfo, 0, len(paths))
	for _, path := range paths {
		item, err := backupFileInfo(path)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt > items[j].CreatedAt })
	return items, nil
}

func (s *databaseSettingsService) backupPathByID(id string) (string, error) {
	paths, err := backup.ListFiles(s.backupDir)
	if err != nil {
		return "", err
	}
	for _, path := range s.filterBackupPathsForCurrentDriver(paths) {
		if backupID(path) == id {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w: backup not found", ErrStorageBackupNotFound)
}

func (s *databaseSettingsService) filterBackupPathsForCurrentDriver(paths []string) []string {
	extension := s.currentBackupFileExtension()
	if extension == "" {
		return nil
	}
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.EqualFold(filepath.Ext(path), extension) {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

func (s *databaseSettingsService) currentBackupFileExtension() string {
	if s.sqliteFileBackupsSupported() {
		return ".db"
	}
	if s.postgresBackupsSupported() {
		return ".dump"
	}
	return ""
}

func validateStorageSettings(input servicedto.UpdateDatabaseCleanupSettingsInput) error {
	if input.RequestLogRetentionDays < 0 {
		return fmt.Errorf("%w: request log retention days must be non-negative", ErrStorageSettingsInvalid)
	}
	if input.UsageLogRetentionDays < 0 {
		return fmt.Errorf("%w: usage log retention days must be non-negative", ErrStorageSettingsInvalid)
	}
	if input.MaxDatabaseSizeMB < 0 {
		return fmt.Errorf("%w: max database size MB must be non-negative", ErrStorageSettingsInvalid)
	}
	if input.BackupHour < 0 || input.BackupHour > 23 {
		return fmt.Errorf("%w: backup hour must be between 0 and 23", ErrStorageSettingsInvalid)
	}
	if input.BackupMinute < 0 || input.BackupMinute > 59 {
		return fmt.Errorf("%w: backup minute must be between 0 and 59", ErrStorageSettingsInvalid)
	}
	if input.MaxBackupCount < 0 {
		return fmt.Errorf("%w: max backup count must be non-negative", ErrStorageSettingsInvalid)
	}
	return nil
}

func backupFileInfo(path string) (servicedto.BackupFileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return servicedto.BackupFileInfo{}, err
	}
	return servicedto.BackupFileInfo{
		ID:        backupID(path),
		FileName:  filepath.Base(path),
		SizeBytes: info.Size(),
		CreatedAt: info.ModTime().Format(time.RFC3339),
	}, nil
}

func backupID(path string) string {
	return strings.TrimSuffix(filepath.Base(filepath.Dir(path))+"/"+filepath.Base(path), filepath.Ext(path))
}
