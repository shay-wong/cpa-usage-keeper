package backup

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

	"github.com/mattn/go-sqlite3"
)

const (
	// databaseBackupFilePrefix 标记普通自动/手动备份，按最大备份数纳入清理计数。
	databaseBackupFilePrefix = "database_"
	// restoreSafetyBackupFilePrefix 标记恢复前安全备份，避免最大备份数清理误删回滚点。
	restoreSafetyBackupFilePrefix = "restore_safety_"
)

type Options struct {
	RequestLogs     bool
	UsageLogs       bool
	UsageIdentities bool
	APIKeys         bool
	RedisInbox      bool
	ModelPrices     bool
}

func (options Options) withDefaults() Options {
	if !options.anyDomainEnabled() {
		return Options{RequestLogs: true, UsageLogs: true, UsageIdentities: true, APIKeys: true, RedisInbox: true, ModelPrices: true}
	}
	return options
}

func (options Options) anyDomainEnabled() bool {
	return options.RequestLogs || options.UsageLogs || options.UsageIdentities || options.APIKeys || options.RedisInbox || options.ModelPrices
}

type Writer struct {
	dir string
}

func NewWriter(dir string) *Writer {
	return &Writer{dir: strings.TrimSpace(dir)}
}

func (w *Writer) WriteDatabase(ctx context.Context, db *sql.DB, backupAt time.Time) (string, error) {
	return w.writeDatabaseWithPrefix(ctx, db, backupAt, Options{RequestLogs: true, UsageLogs: true, UsageIdentities: true, APIKeys: true, RedisInbox: true, ModelPrices: true}, databaseBackupFilePrefix)
}

func (w *Writer) WriteDatabaseWithOptions(ctx context.Context, db *sql.DB, backupAt time.Time, options Options) (string, error) {
	return w.writeDatabaseWithPrefix(ctx, db, backupAt, options, databaseBackupFilePrefix)
}

// WriteRestoreSafetyBackup 创建恢复前回滚点，文件名避开最大备份数清理范围。
func (w *Writer) WriteRestoreSafetyBackup(ctx context.Context, db *sql.DB, backupAt time.Time) (string, error) {
	return w.writeDatabaseWithPrefix(ctx, db, backupAt, Options{RequestLogs: true, UsageLogs: true, UsageIdentities: true, APIKeys: true, RedisInbox: true, ModelPrices: true}, restoreSafetyBackupFilePrefix)
}

func (w *Writer) writeDatabaseWithPrefix(ctx context.Context, db *sql.DB, backupAt time.Time, options Options, filePrefix string) (string, error) {
	if w == nil {
		return "", fmt.Errorf("backup writer is nil")
	}
	if w.dir == "" {
		return "", fmt.Errorf("backup directory is required")
	}
	if db == nil {
		return "", fmt.Errorf("database is required")
	}
	options = options.withDefaults()

	stamp := backupAt.In(time.Local)
	if stamp.IsZero() {
		stamp = time.Now().In(time.Local)
	}
	dayDir := filepath.Join(w.dir, stamp.Format("2006-01-02"))
	if err := os.MkdirAll(dayDir, 0o700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	if err := os.Chmod(dayDir, 0o700); err != nil {
		return "", fmt.Errorf("restrict backup directory permissions: %w", err)
	}

	fileName := fmt.Sprintf("%s%s.db", filePrefix, stamp.Format("20060102T150405.000000000"))
	fullPath := filepath.Join(dayDir, fileName)
	tempPath := fullPath + ".tmp"
	_ = os.Remove(tempPath)
	if err := copySQLiteDatabase(ctx, db, tempPath); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	if err := applyBackupOptions(tempPath, options); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	if err := os.Chmod(tempPath, 0o600); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("restrict backup file permissions: %w", err)
	}
	if err := os.Rename(tempPath, fullPath); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("finalize backup file: %w", err)
	}
	return fullPath, nil
}

func copySQLiteDatabase(ctx context.Context, sourceDB *sql.DB, destPath string) error {
	sourceConn, err := sourceDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open source database connection: %w", err)
	}
	defer sourceConn.Close()

	destDB, err := sql.Open("sqlite3", destPath)
	if err != nil {
		return fmt.Errorf("open backup database: %w", err)
	}
	defer destDB.Close()
	destConn, err := destDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open backup database connection: %w", err)
	}
	defer destConn.Close()

	return sourceConn.Raw(func(sourceDriverConn any) error {
		sourceSQLite, ok := sourceDriverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("source database connection is not sqlite3")
		}
		return destConn.Raw(func(destDriverConn any) error {
			destSQLite, ok := destDriverConn.(*sqlite3.SQLiteConn)
			if !ok {
				return fmt.Errorf("backup database connection is not sqlite3")
			}
			backup, err := destSQLite.Backup("main", sourceSQLite, "main")
			if err != nil {
				return fmt.Errorf("start sqlite backup: %w", err)
			}
			var backupErr error
			for {
				if err := ctx.Err(); err != nil {
					backupErr = err
					break
				}
				done, err := backup.Step(100)
				if err != nil {
					backupErr = fmt.Errorf("copy sqlite backup: %w", err)
					break
				}
				if done {
					break
				}
			}
			if err := backup.Close(); err != nil {
				backupErr = errors.Join(backupErr, fmt.Errorf("close sqlite backup: %w", err))
			}
			return backupErr
		})
	})
}

func applyBackupOptions(path string, options Options) error {
	backupDB, err := sql.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("open backup database for filtering: %w", err)
	}
	defer backupDB.Close()
	if !options.RequestLogs {
		if _, err := backupDB.Exec(`DELETE FROM usage_request_details`); err != nil {
			return fmt.Errorf("remove request logs from backup: %w", err)
		}
	}
	if !options.UsageLogs {
		for _, table := range []string{
			"usage_events",
			"usage_overview_hourly_stats",
			"usage_overview_daily_stats",
			"usage_overview_health_stats",
			"usage_overview_aggregation_checkpoints",
		} {
			if _, err := backupDB.Exec(`DELETE FROM ` + table); err != nil {
				return fmt.Errorf("remove usage logs from backup: %w", err)
			}
		}
		if _, err := backupDB.Exec(`
			UPDATE usage_identities
			SET total_requests = 0,
				success_count = 0,
				failure_count = 0,
				input_tokens = 0,
				output_tokens = 0,
				reasoning_tokens = 0,
				cached_tokens = 0,
				total_tokens = 0,
				last_aggregated_usage_event_id = 0,
				first_used_at = NULL,
				last_used_at = NULL,
				stats_updated_at = NULL
		`); err != nil {
			return fmt.Errorf("reset usage identity stats in backup: %w", err)
		}
	}
	if !options.UsageIdentities {
		if _, err := backupDB.Exec(`DELETE FROM usage_identities`); err != nil {
			return fmt.Errorf("remove usage identities from backup: %w", err)
		}
	}
	if !options.APIKeys {
		if _, err := backupDB.Exec(`DELETE FROM cpa_api_keys`); err != nil {
			return fmt.Errorf("remove API keys from backup: %w", err)
		}
	}
	if !options.RedisInbox {
		if _, err := backupDB.Exec(`DELETE FROM redis_usage_inboxes`); err != nil {
			return fmt.Errorf("remove Redis inbox from backup: %w", err)
		}
	}
	if !options.ModelPrices {
		if _, err := backupDB.Exec(`DELETE FROM model_price_settings`); err != nil {
			return fmt.Errorf("remove model prices from backup: %w", err)
		}
	}
	if _, err := backupDB.Exec(`VACUUM`); err != nil {
		return fmt.Errorf("compact filtered backup: %w", err)
	}
	return nil
}

func (w *Writer) Cleanup(retentionDays int, now time.Time) (int, error) {
	if w == nil {
		return 0, fmt.Errorf("backup writer is nil")
	}
	return Cleanup(w.dir, retentionDays, now)
}

func Cleanup(dir string, retentionDays int, now time.Time) (int, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" || retentionDays <= 0 {
		return 0, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read backup directory: %w", err)
	}

	localNow := now.In(time.Local)
	cutoff := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -retentionDays)
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		backupDay, err := time.ParseInLocation("2006-01-02", entry.Name(), time.Local)
		if err != nil {
			continue
		}
		if backupDay.Before(cutoff.Truncate(24 * time.Hour)) {
			if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
				return removed, fmt.Errorf("remove expired backup directory %s: %w", entry.Name(), err)
			}
			removed++
		}
	}

	return removed, nil
}

func CleanupByMaxCount(dir string, maxCount int) (int, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" || maxCount <= 0 {
		return 0, nil
	}
	files, err := ListCleanupEligibleFiles(dir)
	if err != nil {
		return 0, err
	}
	if len(files) <= maxCount {
		return 0, nil
	}
	removed := 0
	for _, file := range files[:len(files)-maxCount] {
		if err := os.Remove(file); err != nil {
			return removed, fmt.Errorf("remove overflow backup file %s: %w", filepath.Base(file), err)
		}
		removed++
	}
	if err := removeEmptyBackupDirectories(dir); err != nil {
		return removed, err
	}
	return removed, nil
}

// ListCleanupEligibleFiles 只返回普通备份，恢复安全备份不参与自动数量清理。
func ListCleanupEligibleFiles(dir string) ([]string, error) {
	files, err := ListFiles(dir)
	if err != nil {
		return nil, err
	}
	items := make([]string, 0, len(files))
	for _, file := range files {
		if strings.HasPrefix(filepath.Base(file), databaseBackupFilePrefix) {
			items = append(items, file)
		}
	}
	return items, nil
}

func DirectorySize(dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(strings.TrimSpace(dir), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return total, nil
}

func removeEmptyBackupDirectories(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read backup directory for empty cleanup: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		children, err := os.ReadDir(path)
		if err != nil {
			return fmt.Errorf("read backup day directory %s: %w", entry.Name(), err)
		}
		if len(children) == 0 {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove empty backup day directory %s: %w", entry.Name(), err)
			}
		}
	}
	return nil
}

func isBackupFileName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".db" || ext == ".dump"
}

func ListFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if isBackupFileName(d.Name()) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
