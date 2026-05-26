package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository/dto"
	"cpa-usage-keeper/internal/timeutil"
	"gorm.io/gorm"
)

const (
	// databaseCleanupSettingsSingletonID 固定数据库清理配置只有一行，避免 UI 保存产生多份配置。
	databaseCleanupSettingsSingletonID int64 = 1
	// usageRequestDetailSizeCleanupBatchMax 限制单轮按库大小清理的最大删除批次，避免一次构造过大的 SQLite IN 条件。
	usageRequestDetailSizeCleanupBatchMax = 500
	// usageRequestDetailSizeCleanupBatchDivisor 控制按库大小清理时每轮最多删除约 10% 详情缓存。
	usageRequestDetailSizeCleanupBatchDivisor = 10
	// databaseSizeCleanupMaxIterations 防止 VACUUM 无法继续收缩文件时按大小清理无限循环。
	databaseSizeCleanupMaxIterations = 20
)

type restoreDomainTable string

const (
	restoreDomainTableRequestDetails  restoreDomainTable = "usage_request_details"
	restoreDomainTableUsageEvents     restoreDomainTable = "usage_events"
	restoreDomainTableUsageIdentities restoreDomainTable = "usage_identities"
	restoreDomainTableAPIKeys         restoreDomainTable = "cpa_api_keys"
	restoreDomainTableRedisInbox      restoreDomainTable = "redis_usage_inboxes"
	restoreDomainTableModelPrices     restoreDomainTable = "model_price_settings"
)

func GetDatabaseCleanupSettings(db *gorm.DB) (entities.DatabaseCleanupSettings, error) {
	if db == nil {
		return entities.DatabaseCleanupSettings{}, fmt.Errorf("database is nil")
	}
	var settings entities.DatabaseCleanupSettings
	if err := db.First(&settings, databaseCleanupSettingsSingletonID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultDatabaseCleanupSettings(), nil
		}
		return entities.DatabaseCleanupSettings{}, fmt.Errorf("get database cleanup settings: %w", err)
	}
	return settings, nil
}

func UpsertDatabaseCleanupSettings(db *gorm.DB, input dto.DatabaseCleanupSettingsInput) (entities.DatabaseCleanupSettings, error) {
	if db == nil {
		return entities.DatabaseCleanupSettings{}, fmt.Errorf("database is nil")
	}
	settings := entities.DatabaseCleanupSettings{
		ID:                      databaseCleanupSettingsSingletonID,
		RecordRequestDetails:    input.RecordRequestDetails,
		CleanupRequestLogs:      input.CleanupRequestLogs,
		CleanupUsageLogs:        input.CleanupUsageLogs,
		RequestLogRetentionDays: input.RequestLogRetentionDays,
		UsageLogRetentionDays:   input.UsageLogRetentionDays,
		MaxDatabaseSizeMB:       input.MaxDatabaseSizeMB,
		BackupRequestLogs:       input.BackupRequestLogs,
		BackupUsageLogs:         input.BackupUsageLogs,
		BackupUsageIdentities:   input.BackupUsageIdentities,
		BackupAPIKeys:           input.BackupAPIKeys,
		BackupRedisInbox:        input.BackupRedisInbox,
		BackupModelPrices:       input.BackupModelPrices,
		BackupHour:              input.BackupHour,
		BackupMinute:            input.BackupMinute,
		MaxBackupCount:          input.MaxBackupCount,
	}
	now := timeutil.FormatStorageTime(time.Now())
	// GORM default tag 首次插入会把 false/0 交给数据库默认值，这里必须显式写入所有用户值。
	if err := db.Exec(`
		INSERT INTO database_cleanup_settings (
			id,
			record_request_details,
			cleanup_request_logs,
			cleanup_usage_logs,
			request_log_retention_days,
			usage_log_retention_days,
			max_database_size_mb,
			backup_request_logs,
			backup_usage_logs,
			backup_usage_identities,
			backup_api_keys,
			backup_redis_inbox,
			backup_model_prices,
			backup_hour,
			backup_minute,
			max_backup_count,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			record_request_details = excluded.record_request_details,
			cleanup_request_logs = excluded.cleanup_request_logs,
			cleanup_usage_logs = excluded.cleanup_usage_logs,
			request_log_retention_days = excluded.request_log_retention_days,
			usage_log_retention_days = excluded.usage_log_retention_days,
			max_database_size_mb = excluded.max_database_size_mb,
			backup_request_logs = excluded.backup_request_logs,
			backup_usage_logs = excluded.backup_usage_logs,
			backup_usage_identities = excluded.backup_usage_identities,
			backup_api_keys = excluded.backup_api_keys,
			backup_redis_inbox = excluded.backup_redis_inbox,
			backup_model_prices = excluded.backup_model_prices,
			backup_hour = excluded.backup_hour,
			backup_minute = excluded.backup_minute,
			max_backup_count = excluded.max_backup_count,
			updated_at = excluded.updated_at
	`,
		databaseCleanupSettingsSingletonID,
		settings.RecordRequestDetails,
		settings.CleanupRequestLogs,
		settings.CleanupUsageLogs,
		settings.RequestLogRetentionDays,
		settings.UsageLogRetentionDays,
		settings.MaxDatabaseSizeMB,
		settings.BackupRequestLogs,
		settings.BackupUsageLogs,
		settings.BackupUsageIdentities,
		settings.BackupAPIKeys,
		settings.BackupRedisInbox,
		settings.BackupModelPrices,
		settings.BackupHour,
		settings.BackupMinute,
		settings.MaxBackupCount,
		now,
		now,
	).Error; err != nil {
		return entities.DatabaseCleanupSettings{}, fmt.Errorf("upsert database cleanup settings: %w", err)
	}
	return GetDatabaseCleanupSettings(db)
}

func normalizeDatabaseCleanupInput(settings dto.DatabaseCleanupSettingsInput) dto.DatabaseCleanupSettingsInput {
	if !settings.CleanupRequestLogs && !settings.CleanupUsageLogs && settings.RequestLogRetentionDays == 0 && settings.UsageLogRetentionDays == 0 && settings.MaxDatabaseSizeMB == 0 {
		settings.CleanupRequestLogs = true
	}
	if (settings.RequestLogRetentionDays > 0 || settings.MaxDatabaseSizeMB > 0) && !settings.CleanupUsageLogs {
		settings.CleanupRequestLogs = true
	}
	return settings
}

func CleanupUsageRequestDetails(db *gorm.DB, settings dto.DatabaseCleanupSettingsInput, now time.Time, sqlitePath string) (dto.UsageRequestDetailCleanupResult, error) {
	settings = normalizeDatabaseCleanupInput(settings)
	if db == nil {
		return dto.UsageRequestDetailCleanupResult{}, fmt.Errorf("database is nil")
	}
	var result dto.UsageRequestDetailCleanupResult
	if !settings.CleanupRequestLogs {
		return result, nil
	}
	if settings.RequestLogRetentionDays > 0 {
		deleted, err := cleanupUsageRequestDetailsByRetention(db, settings.RequestLogRetentionDays, now)
		if err != nil {
			return result, err
		}
		result.RetentionDeleted = deleted
	}
	if settings.MaxDatabaseSizeMB > 0 {
		deleted, err := cleanupUsageRequestDetailsByDatabaseSize(db, settings.MaxDatabaseSizeMB, sqlitePath)
		if err != nil {
			return result, err
		}
		result.SizeDeleted = deleted
	}
	return result, nil
}

func CleanupUsageEvents(ctx context.Context, db *gorm.DB, settings dto.DatabaseCleanupSettingsInput, now time.Time, sqlitePath string) (dto.UsageEventCleanupResult, error) {
	settings = normalizeDatabaseCleanupInput(settings)
	if db == nil {
		return dto.UsageEventCleanupResult{}, fmt.Errorf("database is nil")
	}
	var result dto.UsageEventCleanupResult
	if !settings.CleanupUsageLogs {
		return result, nil
	}
	if settings.UsageLogRetentionDays > 0 {
		deleted, err := cleanupUsageEventsByRetention(db, settings.UsageLogRetentionDays, now)
		if err != nil {
			return result, err
		}
		result.RetentionDeleted = deleted
	}
	if settings.MaxDatabaseSizeMB > 0 {
		deleted, err := cleanupUsageEventsByDatabaseSize(db, settings.MaxDatabaseSizeMB, sqlitePath)
		if err != nil {
			return result, err
		}
		result.SizeDeleted = deleted
	}
	if result.RetentionDeleted > 0 || result.SizeDeleted > 0 {
		if err := RebuildUsageDerivedStats(ctx, db, now); err != nil {
			return result, err
		}
	}
	return result, nil
}

func defaultDatabaseCleanupSettings() entities.DatabaseCleanupSettings {
	return entities.DatabaseCleanupSettings{
		ID:                    databaseCleanupSettingsSingletonID,
		RecordRequestDetails:  true,
		CleanupRequestLogs:    true,
		BackupUsageLogs:       true,
		BackupUsageIdentities: true,
		BackupAPIKeys:         true,
		BackupModelPrices:     true,
		BackupHour:            4,
		MaxBackupCount:        1,
	}
}

func cleanupUsageEventsByRetention(db *gorm.DB, retentionDays int, now time.Time) (int64, error) {
	cutoff := timeutil.NormalizeStorageTime(now).AddDate(0, 0, -retentionDays)
	result := db.Where("timestamp < ?", timeutil.FormatStorageTime(cutoff)).Delete(&entities.UsageEvent{})
	if result.Error != nil {
		return 0, fmt.Errorf("cleanup usage events by retention: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func cleanupUsageEventsByDatabaseSize(db *gorm.DB, maxDatabaseSizeMB int, sqlitePath string) (int64, error) {
	maxBytes := int64(maxDatabaseSizeMB) * 1024 * 1024
	var deleted int64
	for iteration := 0; iteration < databaseSizeCleanupMaxIterations; iteration++ {
		sizeBytes, ok, err := GetDatabaseSizeBytes(db, sqlitePath)
		if err != nil || !ok {
			return deleted, err
		}
		if sizeBytes <= maxBytes {
			return deleted, nil
		}
		batchSize, err := usageEventCleanupBatchSize(db)
		if err != nil || batchSize == 0 {
			return deleted, err
		}
		batchDeleted, err := deleteOldestUsageEvents(db, batchSize)
		if err != nil || batchDeleted == 0 {
			return deleted, err
		}
		deleted += batchDeleted
		if err := Vacuum(db); err != nil {
			return deleted, err
		}
	}
	return deleted, fmt.Errorf("usage event size cleanup reached iteration limit")
}

func usageEventCleanupBatchSize(db *gorm.DB) (int, error) {
	var total int64
	if err := db.Model(&entities.UsageEvent{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count usage events for size cleanup: %w", err)
	}
	if total == 0 {
		return 0, nil
	}
	batchSize := int(total / usageRequestDetailSizeCleanupBatchDivisor)
	if batchSize < 1 {
		return 1, nil
	}
	return min(batchSize, usageRequestDetailSizeCleanupBatchMax), nil
}

func deleteOldestUsageEvents(db *gorm.DB, batchSize int) (int64, error) {
	var ids []int64
	if err := db.Model(&entities.UsageEvent{}).Order("timestamp ASC, id ASC").Limit(batchSize).Pluck("id", &ids).Error; err != nil {
		return 0, fmt.Errorf("select old usage events for size cleanup: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := db.Where("id IN ?", ids).Delete(&entities.UsageEvent{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete old usage events for size cleanup: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func RebuildUsageDerivedStats(ctx context.Context, db *gorm.DB, now time.Time) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := resetUsageDerivedStats(tx); err != nil {
			return err
		}
		if err := AggregateUsageIdentityStats(ctx, tx, now); err != nil {
			return fmt.Errorf("rebuild usage identity stats: %w", err)
		}
		if err := AggregateUsageOverviewStats(ctx, tx, now); err != nil {
			return fmt.Errorf("rebuild usage overview stats: %w", err)
		}
		return nil
	})
}

func resetUsageDerivedStats(tx *gorm.DB) error {
	for _, model := range []any{
		&entities.UsageOverviewHourlyStat{},
		&entities.UsageOverviewDailyStat{},
		&entities.UsageOverviewHealthStat{},
		&entities.UsageOverviewAggregationCheckpoint{},
	} {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(model).Error; err != nil {
			return fmt.Errorf("clear usage derived stats: %w", err)
		}
	}
	updates := map[string]any{
		"total_requests":                 0,
		"success_count":                  0,
		"failure_count":                  0,
		"input_tokens":                   0,
		"output_tokens":                  0,
		"reasoning_tokens":               0,
		"cached_tokens":                  0,
		"total_tokens":                   0,
		"last_aggregated_usage_event_id": 0,
		"first_used_at":                  nil,
		"last_used_at":                   nil,
		"stats_updated_at":               nil,
	}
	if err := tx.Model(&entities.UsageIdentity{}).Where("1 = 1").Updates(updates).Error; err != nil {
		return fmt.Errorf("reset usage identity stats: %w", err)
	}
	return nil
}

func cleanupUsageRequestDetailsByRetention(db *gorm.DB, retentionDays int, now time.Time) (int64, error) {
	cutoff := timeutil.NormalizeStorageTime(now).AddDate(0, 0, -retentionDays)
	result := db.Where("fetched_at < ?", timeutil.FormatStorageTime(cutoff)).Delete(&entities.UsageRequestDetail{})
	if result.Error != nil {
		return 0, fmt.Errorf("cleanup usage request details by retention: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func cleanupUsageRequestDetailsByDatabaseSize(db *gorm.DB, maxDatabaseSizeMB int, sqlitePath string) (int64, error) {
	maxBytes := int64(maxDatabaseSizeMB) * 1024 * 1024
	var deleted int64
	for iteration := 0; iteration < databaseSizeCleanupMaxIterations; iteration++ {
		sizeBytes, ok, err := GetDatabaseSizeBytes(db, sqlitePath)
		if err != nil || !ok {
			return deleted, err
		}
		if sizeBytes <= maxBytes {
			return deleted, nil
		}
		batchSize, err := usageRequestDetailCleanupBatchSize(db)
		if err != nil || batchSize == 0 {
			return deleted, err
		}
		batchDeleted, err := deleteOldestUsageRequestDetails(db, batchSize)
		if err != nil || batchDeleted == 0 {
			return deleted, err
		}
		deleted += batchDeleted
		if err := Vacuum(db); err != nil {
			return deleted, err
		}
	}
	return deleted, fmt.Errorf("usage request detail size cleanup reached iteration limit")
}

// GetDatabaseSizeBytes 返回当前数据库大小；SQLite 走文件大小，PostgreSQL 走 pg_database_size。
func GetDatabaseSizeBytes(db *gorm.DB, sqlitePath string) (int64, bool, error) {
	if db == nil {
		return 0, false, fmt.Errorf("database is nil")
	}
	switch db.Dialector.Name() {
	case "sqlite":
		return sqliteDatabaseSizeBytes(sqlitePath)
	case "postgres":
		var sizeBytes int64
		if err := db.Raw("SELECT pg_database_size(current_database())").Scan(&sizeBytes).Error; err != nil {
			return 0, false, fmt.Errorf("get postgres database size: %w", err)
		}
		return sizeBytes, true, nil
	default:
		return 0, false, fmt.Errorf("unsupported database driver %q", db.Dialector.Name())
	}
}

// GetSQLiteDatabaseSizeBytes 返回当前 SQLite 文件大小；内存库或无路径时 ok=false。
func GetSQLiteDatabaseSizeBytes(sqlitePath string) (int64, bool, error) {
	return sqliteDatabaseSizeBytes(sqlitePath)
}

func usageRequestDetailCleanupBatchSize(db *gorm.DB) (int, error) {
	var total int64
	if err := db.Model(&entities.UsageRequestDetail{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count usage request details for size cleanup: %w", err)
	}
	if total == 0 {
		return 0, nil
	}
	batchSize := int(total / usageRequestDetailSizeCleanupBatchDivisor)
	if batchSize < 1 {
		return 1, nil
	}
	return min(batchSize, usageRequestDetailSizeCleanupBatchMax), nil
}

func deleteOldestUsageRequestDetails(db *gorm.DB, batchSize int) (int64, error) {
	var ids []int64
	if err := db.Model(&entities.UsageRequestDetail{}).Order("fetched_at ASC, id ASC").Limit(batchSize).Pluck("id", &ids).Error; err != nil {
		return 0, fmt.Errorf("select old usage request details for size cleanup: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := db.Where("id IN ?", ids).Delete(&entities.UsageRequestDetail{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete old usage request details for size cleanup: %w", result.Error)
	}
	return result.RowsAffected, nil
}

type StorageDomainSelection struct {
	RequestLogs     bool
	UsageLogs       bool
	UsageIdentities bool
	APIKeys         bool
	RedisInbox      bool
	ModelPrices     bool
}

func (selection StorageDomainSelection) AnyEnabled() bool {
	return selection.RequestLogs || selection.UsageLogs || selection.UsageIdentities || selection.APIKeys || selection.RedisInbox || selection.ModelPrices
}

type StorageDomainStats struct {
	Key         string
	Label       string
	Description string
	TableNames  []string
	Rows        int64
	SizeBytes   int64
}

func GetStorageDomainStats(db *gorm.DB) ([]StorageDomainStats, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	requestRows, err := countTableRows(db, &entities.UsageRequestDetail{})
	if err != nil {
		return nil, err
	}
	usageRows, err := countTableRows(db, &entities.UsageEvent{})
	if err != nil {
		return nil, err
	}
	usageIdentityRows, err := countTableRows(db, &entities.UsageIdentity{})
	if err != nil {
		return nil, err
	}
	apiKeyRows, err := countTableRows(db, &entities.CPAAPIKey{})
	if err != nil {
		return nil, err
	}
	redisRows, err := countTableRows(db, &entities.RedisUsageInbox{})
	if err != nil {
		return nil, err
	}
	overviewRows, err := countOverviewRows(db)
	if err != nil {
		return nil, err
	}
	modelPriceRows, err := countTableRows(db, &entities.ModelPriceSetting{})
	if err != nil {
		return nil, err
	}
	return []StorageDomainStats{
		{Key: "request_logs", Label: "请求日志", Description: "请求详情日志缓存，可关闭记录或按保留期清理。", TableNames: []string{"usage_request_details"}, Rows: requestRows, SizeBytes: requestRows * 4096},
		{Key: "usage_logs", Label: "用量日志", Description: "原始用量事件和可重建的统计缓存。", TableNames: []string{"usage_events", "usage_overview_hourly_stats", "usage_overview_daily_stats", "usage_overview_health_stats", "usage_overview_aggregation_checkpoints"}, Rows: usageRows, SizeBytes: (usageRows + overviewRows) * 4096},
		{Key: "usage_identities", Label: "凭证与身份", Description: "Auth File 和 AI Provider 凭证身份及其累计统计。", TableNames: []string{"usage_identities"}, Rows: usageIdentityRows, SizeBytes: usageIdentityRows * 4096},
		{Key: "api_keys", Label: "API Key", Description: "CPA 管理接口同步的 API Key 与别名。", TableNames: []string{"cpa_api_keys"}, Rows: apiKeyRows, SizeBytes: apiKeyRows * 4096},
		{Key: "redis_inbox", Label: "Redis 队列缓存", Description: "本地暂存的 Redis 原始消息，维护任务自动清理。", TableNames: []string{"redis_usage_inboxes"}, Rows: redisRows, SizeBytes: redisRows * 4096},
		{Key: "model_prices", Label: "模型价格", Description: "按模型计算成本使用的价格配置。", TableNames: []string{"model_price_settings"}, Rows: modelPriceRows, SizeBytes: modelPriceRows * 4096},
	}, nil
}

func countTableRows(db *gorm.DB, model any) (int64, error) {
	var count int64
	if err := db.Model(model).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count storage rows: %w", err)
	}
	return count, nil
}

func countOverviewRows(db *gorm.DB) (int64, error) {
	var total int64
	for _, model := range []any{&entities.UsageOverviewHourlyStat{}, &entities.UsageOverviewDailyStat{}, &entities.UsageOverviewHealthStat{}, &entities.UsageOverviewAggregationCheckpoint{}} {
		count, err := countTableRows(db, model)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func RestoreStorageDomains(ctx context.Context, db *gorm.DB, backupPath string, selection StorageDomainSelection, now time.Time) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if strings.TrimSpace(backupPath) == "" {
		return fmt.Errorf("backup path is required")
	}
	if !selection.AnyEnabled() {
		return fmt.Errorf("at least one restore domain is required")
	}
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("check backup file: %w", err)
	}
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("ATTACH DATABASE ? AS restore_backup", backupPath).Error; err != nil {
			return fmt.Errorf("attach backup database: %w", err)
		}
		attached := true
		detach := func() error {
			if !attached {
				return nil
			}
			attached = false
			if err := tx.Exec("DETACH DATABASE restore_backup").Error; err != nil {
				return fmt.Errorf("detach backup database: %w", err)
			}
			return nil
		}
		restoreErr := restoreSelectedTables(tx, selection)
		var rebuildErr error
		if restoreErr == nil && selection.UsageLogs {
			rebuildErr = RebuildUsageDerivedStats(ctx, tx, now)
		}
		detachErr := detach()
		if restoreErr != nil {
			if detachErr != nil {
				return fmt.Errorf("%w; %v", restoreErr, detachErr)
			}
			return restoreErr
		}
		if rebuildErr != nil {
			if detachErr != nil {
				return fmt.Errorf("%w; %v", rebuildErr, detachErr)
			}
			return rebuildErr
		}
		return detachErr
	}); err != nil {
		return err
	}
	return nil
}

func restoreSelectedTables(tx *gorm.DB, selection StorageDomainSelection) error {
	ordered := []struct {
		enabled bool
		table   restoreDomainTable
	}{
		{selection.RequestLogs, restoreDomainTableRequestDetails},
		{selection.UsageIdentities, restoreDomainTableUsageIdentities},
		{selection.UsageLogs, restoreDomainTableUsageEvents},
		{selection.APIKeys, restoreDomainTableAPIKeys},
		{selection.RedisInbox, restoreDomainTableRedisInbox},
		{selection.ModelPrices, restoreDomainTableModelPrices},
	}
	for _, item := range ordered {
		if !item.enabled {
			continue
		}
		if err := restoreTable(tx, item.table); err != nil {
			return err
		}
	}
	return nil
}

func restoreTable(tx *gorm.DB, table restoreDomainTable) error {
	tableName, err := restoreTableName(table)
	if err != nil {
		return err
	}
	if err := tx.Exec("DELETE FROM " + tableName).Error; err != nil {
		return fmt.Errorf("clear %s before restore: %w", tableName, err)
	}
	if err := tx.Exec("INSERT INTO " + tableName + " SELECT * FROM restore_backup." + tableName).Error; err != nil {
		return fmt.Errorf("restore %s: %w", tableName, err)
	}
	return nil
}

func restoreTableName(table restoreDomainTable) (string, error) {
	switch table {
	case restoreDomainTableRequestDetails,
		restoreDomainTableUsageEvents,
		restoreDomainTableUsageIdentities,
		restoreDomainTableAPIKeys,
		restoreDomainTableRedisInbox,
		restoreDomainTableModelPrices:
		return string(table), nil
	default:
		return "", fmt.Errorf("unsupported restore table %q", table)
	}
}

func sqliteDatabaseSizeBytes(sqlitePath string) (int64, bool, error) {
	trimmed := strings.TrimSpace(sqlitePath)
	if before, _, ok := strings.Cut(trimmed, "?"); ok {
		trimmed = before
	}
	if trimmed == "" || trimmed == ":memory:" {
		return 0, false, nil
	}
	info, err := os.Stat(trimmed)
	if err != nil {
		return 0, false, fmt.Errorf("check sqlite database size: %w", err)
	}
	return info.Size(), true, nil
}
