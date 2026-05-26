package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"cpa-usage-keeper/internal/backup"
	"cpa-usage-keeper/internal/service"
	"github.com/sirupsen/logrus"
)

type DatabaseBackupWriter interface {
	WriteDatabase(context.Context, time.Time) (string, error)
	WriteDatabaseWithOptions(context.Context, time.Time, backup.Options) (string, error)
}

type DatabaseBackupCleaner interface {
	Cleanup(retentionDays int, now time.Time) (int, error)
	CleanupByMaxCount(maxCount int) (int, error)
}

type DatabaseBackupSettingsProvider interface {
	GetDatabaseCleanupSettings(context.Context) (service.DatabaseCleanupSettingsSnapshot, error)
}

type DatabaseBackupHistory interface {
	LastBackupAt() (time.Time, bool, error)
}

type databaseBackupStore struct {
	db     *sql.DB
	dir    string
	writer *backup.Writer
}

func newDatabaseBackupStore(db *sql.DB, dir string) *databaseBackupStore {
	return &databaseBackupStore{db: db, dir: dir, writer: backup.NewWriter(dir)}
}

func (s *databaseBackupStore) WriteDatabase(ctx context.Context, backupAt time.Time) (string, error) {
	return s.writer.WriteDatabase(ctx, s.db, backupAt)
}

func (s *databaseBackupStore) WriteDatabaseWithOptions(ctx context.Context, backupAt time.Time, options backup.Options) (string, error) {
	return s.writer.WriteDatabaseWithOptions(ctx, s.db, backupAt, options)
}

func (s *databaseBackupStore) Cleanup(retentionDays int, now time.Time) (int, error) {
	return s.writer.Cleanup(retentionDays, now)
}

func (s *databaseBackupStore) CleanupByMaxCount(maxCount int) (int, error) {
	return backup.CleanupByMaxCount(s.dir, maxCount)
}

func (s *databaseBackupStore) LastBackupAt() (time.Time, bool, error) {
	return lastDatabaseBackupAt(s.dir)
}

func lastDatabaseBackupAt(dir string) (time.Time, bool, error) {
	files, err := backup.ListCleanupEligibleFiles(dir)
	if err != nil {
		return time.Time{}, false, err
	}
	var latest time.Time
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			return time.Time{}, false, err
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest, !latest.IsZero(), nil
}

type postgresBackupStore struct {
	dir    string
	writer *backup.PostgresWriter
}

func newPostgresBackupStore(dir string, databaseURL string) *postgresBackupStore {
	return &postgresBackupStore{dir: dir, writer: backup.NewPostgresWriter(dir, databaseURL)}
}

func (s *postgresBackupStore) WriteDatabase(ctx context.Context, backupAt time.Time) (string, error) {
	return s.writer.WriteDatabase(ctx, backupAt)
}

func (s *postgresBackupStore) WriteDatabaseWithOptions(ctx context.Context, backupAt time.Time, options backup.Options) (string, error) {
	return s.writer.WriteDatabaseWithOptions(ctx, backupAt, options)
}

func (s *postgresBackupStore) Cleanup(retentionDays int, now time.Time) (int, error) {
	return backup.Cleanup(s.dir, retentionDays, now)
}

func (s *postgresBackupStore) CleanupByMaxCount(maxCount int) (int, error) {
	return backup.CleanupByMaxCount(s.dir, maxCount)
}

func (s *postgresBackupStore) LastBackupAt() (time.Time, bool, error) {
	return lastDatabaseBackupAt(s.dir)
}

type DatabaseBackupRunner struct {
	writer        DatabaseBackupWriter
	cleaner       DatabaseBackupCleaner
	history       DatabaseBackupHistory
	settings      DatabaseBackupSettingsProvider
	interval      time.Duration
	retentionDays int
	lastBackupAt  time.Time
	retryDelay    time.Duration
	retryAttempts int
	pendingRetry  bool
	now           func() time.Time
	sleep         func(context.Context, time.Duration) bool

	mu      sync.Mutex
	running bool
}

func NewDatabaseBackupRunner(writer DatabaseBackupWriter, cleaner DatabaseBackupCleaner, interval time.Duration, retentionDays int) *DatabaseBackupRunner {
	return NewDatabaseBackupRunnerWithSettings(writer, cleaner, nil, interval, retentionDays)
}

func NewDatabaseBackupRunnerWithSettings(writer DatabaseBackupWriter, cleaner DatabaseBackupCleaner, settings DatabaseBackupSettingsProvider, interval time.Duration, retentionDays int) *DatabaseBackupRunner {
	history, _ := writer.(DatabaseBackupHistory)
	return &DatabaseBackupRunner{
		writer:        writer,
		cleaner:       cleaner,
		history:       history,
		settings:      settings,
		interval:      interval,
		retentionDays: retentionDays,
		retryDelay:    15 * time.Minute,
		now:           time.Now,
		sleep:         maintenanceSleepContext,
	}
}

func (r *DatabaseBackupRunner) Run(ctx context.Context) error {
	if err := r.validate(); err != nil {
		return err
	}
	logrus.Info("database backup task started")
	r.setRunning(true)
	defer r.setRunning(false)

	for {
		now := r.now()
		delay := r.nextDelay(ctx, now)
		if delay < 0 {
			delay = 0
		}
		if !r.sleep(ctx, delay) {
			return nil
		}
		backupAt := r.now()
		settings := r.currentSettings(ctx)
		if _, err := r.writer.WriteDatabaseWithOptions(ctx, backupAt, backup.Options{
			RequestLogs:     settings.Settings.BackupRequestLogs,
			UsageLogs:       settings.Settings.BackupUsageLogs,
			UsageIdentities: settings.Settings.BackupUsageIdentities,
			APIKeys:         settings.Settings.BackupAPIKeys,
			RedisInbox:      settings.Settings.BackupRedisInbox,
			ModelPrices:     settings.Settings.BackupModelPrices,
		}); err != nil {
			logrus.WithError(err).Error("database backup failed")
			r.retryAttempts++
			r.pendingRetry = r.retryAttempts <= 3
		} else {
			r.lastBackupAt = backupAt
			r.retryAttempts = 0
			r.pendingRetry = false
		}
		r.cleanup(backupAt, settings.Settings.MaxBackupCount)
	}
}

func (r *DatabaseBackupRunner) nextDelay(ctx context.Context, now time.Time) time.Duration {
	if r.pendingRetry {
		return r.retryDelay
	}
	settings := r.currentSettings(ctx)
	if r.usesDailySchedule() {
		return r.nextDailyDelay(now, settings.Settings.BackupHour, settings.Settings.BackupMinute)
	}
	lastBackupAt, ok := r.lastBackupAtFromHistory()
	if !ok {
		return 0
	}
	return lastBackupAt.Add(r.interval).Sub(now)
}

func (r *DatabaseBackupRunner) cleanup(now time.Time, maxBackupCount int) {
	if r.cleaner == nil {
		return
	}
	if r.retentionDays > 0 {
		if _, err := r.cleaner.Cleanup(r.retentionDays, now); err != nil {
			logrus.WithError(err).Error("database backup cleanup failed")
		}
	}
	if maxBackupCount > 0 {
		if _, err := r.cleaner.CleanupByMaxCount(maxBackupCount); err != nil {
			logrus.WithError(err).Error("database backup max count cleanup failed")
		}
	}
}

func (r *DatabaseBackupRunner) currentSettings(ctx context.Context) service.DatabaseCleanupSettingsSnapshot {
	settings := service.DatabaseCleanupSettingsSnapshot{}
	settings.Settings.BackupUsageLogs = true
	settings.Settings.BackupUsageIdentities = true
	settings.Settings.BackupAPIKeys = true
	settings.Settings.BackupModelPrices = true
	settings.Settings.BackupHour = 4
	settings.Settings.MaxBackupCount = 1
	if r.settings == nil {
		return settings
	}
	snapshot, err := r.settings.GetDatabaseCleanupSettings(ctx)
	if err != nil {
		logrus.WithError(err).Error("load database backup settings failed")
		return settings
	}
	if snapshot.Settings.BackupHour < 0 || snapshot.Settings.BackupHour > 23 {
		snapshot.Settings.BackupHour = 4
	}
	if snapshot.Settings.BackupMinute < 0 || snapshot.Settings.BackupMinute > 59 {
		snapshot.Settings.BackupMinute = 0
	}
	return snapshot
}

func (r *DatabaseBackupRunner) nextDailyDelay(now time.Time, hour int, minute int) time.Duration {
	localNow := now.In(time.Local)
	nextBackupAt := nextDailyBackupAtAt(localNow, hour, minute)
	if lastBackupAt, ok := r.lastBackupAtFromHistory(); ok {
		lastLocalBackup := lastBackupAt.In(time.Local)
		lastBackupDay := time.Date(lastLocalBackup.Year(), lastLocalBackup.Month(), lastLocalBackup.Day(), hour, minute, 0, 0, time.Local)
		candidate := lastBackupDay.AddDate(0, 0, r.dailyScheduleDays())
		if localNow.Before(candidate) {
			nextBackupAt = candidate
		}
	}
	return nextBackupAt.Sub(localNow)
}

func (r *DatabaseBackupRunner) usesDailySchedule() bool {
	return r.interval >= 24*time.Hour && r.interval%(24*time.Hour) == 0
}

func (r *DatabaseBackupRunner) dailyScheduleDays() int {
	return int(r.interval / (24 * time.Hour))
}

func (r *DatabaseBackupRunner) lastBackupAtFromHistory() (time.Time, bool) {
	lastBackupAt := r.lastBackupAt
	if r.history != nil {
		storedBackupAt, ok, err := r.history.LastBackupAt()
		if err != nil {
			logrus.WithError(err).Error("load last database backup time failed")
		} else if ok && storedBackupAt.After(lastBackupAt) {
			lastBackupAt = storedBackupAt
		}
	}
	return lastBackupAt, !lastBackupAt.IsZero()
}

func nextDailyBackupAt(now time.Time) time.Time {
	return nextDailyBackupAtAt(now, 4, 0)
}

func nextDailyBackupAtAt(now time.Time, hour int, minute int) time.Time {
	localNow := now.In(time.Local)
	backupAt := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hour, minute, 0, 0, time.Local)
	if !localNow.Before(backupAt) {
		backupAt = backupAt.AddDate(0, 0, 1)
	}
	return backupAt
}

func (r *DatabaseBackupRunner) validate() error {
	if r == nil {
		return fmt.Errorf("database backup runner is nil")
	}
	if r.writer == nil {
		return fmt.Errorf("database backup writer is nil")
	}
	if r.interval <= 0 {
		return fmt.Errorf("database backup interval must be positive")
	}
	if r.retryDelay <= 0 {
		r.retryDelay = 15 * time.Minute
	}
	if r.now == nil {
		r.now = time.Now
	}
	if r.sleep == nil {
		r.sleep = maintenanceSleepContext
	}
	return nil
}

func (r *DatabaseBackupRunner) setRunning(running bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = running
}
