package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestWriterWriteDatabaseBacksUpSQLiteDatabase(t *testing.T) {
	root := t.TempDir()
	source := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "source.db"))
	defer source.Close()
	if _, err := source.Exec(`CREATE TABLE records (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := source.Exec(`INSERT INTO records (name) VALUES (?)`, "saved"); err != nil {
		t.Fatalf("insert row: %v", err)
	}
	writer := NewWriter(root)
	backupAt := time.Date(2026, 4, 16, 12, 34, 56, 123456789, time.UTC)

	path, err := writer.WriteDatabase(context.Background(), source, backupAt)
	if err != nil {
		t.Fatalf("WriteDatabase returned error: %v", err)
	}

	if filepath.Dir(path) != filepath.Join(root, "2026-04-16") {
		t.Fatalf("unexpected backup directory: %s", path)
	}
	if filepath.Ext(path) != ".db" || !strings.HasPrefix(filepath.Base(path), "database_") {
		t.Fatalf("unexpected backup file name: %s", filepath.Base(path))
	}
	backupDB := openTestSQLiteDB(t, path)
	defer backupDB.Close()
	var name string
	if err := backupDB.QueryRow(`SELECT name FROM records WHERE id = 1`).Scan(&name); err != nil {
		t.Fatalf("query backup row: %v", err)
	}
	if name != "saved" {
		t.Fatalf("expected backed up row name saved, got %q", name)
	}
}

func TestWriterWriteDatabaseUsesLocalDateDirectory(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.FixedZone("Test/Local", 8*60*60)
	t.Cleanup(func() { time.Local = previousLocal })
	root := t.TempDir()
	source := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "source.db"))
	defer source.Close()
	if _, err := source.Exec(`CREATE TABLE records (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	writer := NewWriter(root)
	backupAt := time.Date(2026, 4, 16, 4, 0, 0, 0, time.Local)

	path, err := writer.WriteDatabase(context.Background(), source, backupAt)
	if err != nil {
		t.Fatalf("WriteDatabase returned error: %v", err)
	}
	if filepath.Dir(path) != filepath.Join(root, "2026-04-16") {
		t.Fatalf("expected local backup date directory 2026-04-16, got %s", path)
	}
}

func TestWriterWriteDatabaseRestrictsBackupPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not meaningful on Windows")
	}
	root := t.TempDir()
	source := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "source.db"))
	defer source.Close()
	if _, err := source.Exec(`CREATE TABLE records (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	writer := NewWriter(root)
	backupAt := time.Date(2026, 4, 16, 12, 34, 56, 0, time.UTC)

	path, err := writer.WriteDatabase(context.Background(), source, backupAt)
	if err != nil {
		t.Fatalf("WriteDatabase returned error: %v", err)
	}

	dayDirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat backup day directory: %v", err)
	}
	if mode := dayDirInfo.Mode().Perm(); mode != 0o700 {
		t.Fatalf("expected backup day directory mode 0700, got %o", mode)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat backup file: %v", err)
	}
	if mode := fileInfo.Mode().Perm(); mode != 0o600 {
		t.Fatalf("expected backup file mode 0600, got %o", mode)
	}
}

func TestWriterWriteDatabaseHonorsCanceledContext(t *testing.T) {
	root := t.TempDir()
	source := openTestSQLiteDB(t, filepath.Join(t.TempDir(), "source.db"))
	defer source.Close()
	if _, err := source.Exec(`CREATE TABLE records (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewWriter(root).WriteDatabase(ctx, source, time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected canceled context error")
	}
	files, listErr := ListFiles(root)
	if listErr != nil {
		t.Fatalf("ListFiles returned error: %v", listErr)
	}
	if len(files) != 0 {
		t.Fatalf("expected no finalized backup files after cancellation, got %+v", files)
	}
}

func TestWriterWriteDatabaseValidatesInputs(t *testing.T) {
	if _, err := NewWriter("").WriteDatabase(context.Background(), openTestSQLiteDB(t, filepath.Join(t.TempDir(), "source.db")), time.Now()); err == nil || !strings.Contains(err.Error(), "backup directory is required") {
		t.Fatalf("expected backup directory error, got %v", err)
	}
	if _, err := NewWriter(t.TempDir()).WriteDatabase(context.Background(), nil, time.Now()); err == nil || !strings.Contains(err.Error(), "database is required") {
		t.Fatalf("expected database required error, got %v", err)
	}
}

func TestCleanupRemovesExpiredBackupDirectories(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, "2026-04-10")
	keepDir := filepath.Join(root, "2026-04-15")
	if err := os.MkdirAll(oldDir, 0o700); err != nil {
		t.Fatalf("create old dir: %v", err)
	}
	if err := os.MkdirAll(keepDir, 0o700); err != nil {
		t.Fatalf("create keep dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "database_old.db"), []byte("old"), 0o600); err != nil {
		t.Fatalf("write old backup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(keepDir, "database_keep.db"), []byte("keep"), 0o600); err != nil {
		t.Fatalf("write keep backup: %v", err)
	}

	removed, err := Cleanup(root, 3, time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed directory, got %d", removed)
	}

	files, err := ListFiles(root)
	if err != nil {
		t.Fatalf("ListFiles returned error: %v", err)
	}
	if len(files) != 1 || filepath.Base(filepath.Dir(files[0])) != "2026-04-15" {
		t.Fatalf("unexpected remaining files: %+v", files)
	}
}

func TestCleanupUsesLocalDateBoundaries(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.FixedZone("Test/Local", 8*60*60)
	t.Cleanup(func() { time.Local = previousLocal })
	root := t.TempDir()
	keepDir := filepath.Join(root, "2026-04-15")
	if err := os.MkdirAll(keepDir, 0o700); err != nil {
		t.Fatalf("create keep dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(keepDir, "database_keep.db"), []byte("keep"), 0o600); err != nil {
		t.Fatalf("write keep backup: %v", err)
	}

	removed, err := Cleanup(root, 1, time.Date(2026, 4, 16, 0, 30, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected local-date retention to keep previous local day, removed %d", removed)
	}
}

func TestListFilesReturnsDatabaseBackups(t *testing.T) {
	root := t.TempDir()
	dayDir := filepath.Join(root, "2026-04-16")
	if err := os.MkdirAll(dayDir, 0o700); err != nil {
		t.Fatalf("create day dir: %v", err)
	}
	databasePath := filepath.Join(dayDir, "database.db")
	dumpPath := filepath.Join(dayDir, "database.dump")
	if err := os.WriteFile(databasePath, []byte("db"), 0o600); err != nil {
		t.Fatalf("write db backup: %v", err)
	}
	if err := os.WriteFile(dumpPath, []byte("dump"), 0o600); err != nil {
		t.Fatalf("write dump backup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dayDir, "snapshot.json"), []byte("json"), 0o600); err != nil {
		t.Fatalf("write json backup: %v", err)
	}

	files, err := ListFiles(root)
	if err != nil {
		t.Fatalf("ListFiles returned error: %v", err)
	}
	if len(files) != 2 || files[0] != databasePath || files[1] != dumpPath {
		t.Fatalf("expected database and dump backups, got %+v", files)
	}
}

func TestCleanupIgnoresMissingDirectory(t *testing.T) {
	removed, err := Cleanup(filepath.Join(t.TempDir(), "missing"), 30, time.Now())
	if err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed directories, got %d", removed)
	}
}

func TestApplyBackupOptionsFiltersUnselectedStorageDomains(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.db")
	db := openTestSQLiteDB(t, path)
	createStorageDomainTables(t, db)
	defer db.Close()

	if err := applyBackupOptions(path, Options{APIKeys: true, RedisInbox: true, ModelPrices: true}); err != nil {
		t.Fatalf("applyBackupOptions returned error: %v", err)
	}

	expectedRows := map[string]int64{
		"usage_request_details":                  0,
		"usage_events":                           0,
		"usage_overview_hourly_stats":            0,
		"usage_overview_daily_stats":             0,
		"usage_overview_health_stats":            0,
		"usage_overview_aggregation_checkpoints": 0,
		"usage_identities":                       0,
		"cpa_api_keys":                           1,
		"redis_usage_inboxes":                    1,
		"model_price_settings":                   1,
	}
	for table, expected := range expectedRows {
		if got := storageTableCount(t, db, table); got != expected {
			t.Fatalf("expected %s row count %d, got %d", table, expected, got)
		}
	}
}

func TestCleanupByMaxCountSkipsRestoreSafetyBackups(t *testing.T) {
	root := t.TempDir()
	dayDir := filepath.Join(root, "2026-05-24")
	if err := os.MkdirAll(dayDir, 0o700); err != nil {
		t.Fatalf("create day dir: %v", err)
	}
	oldRegular := filepath.Join(dayDir, "database_20260524T010000.db")
	newRegular := filepath.Join(dayDir, "database_20260524T020000.db")
	safety := filepath.Join(dayDir, "restore_safety_20260524T015000.db")
	for _, path := range []string{oldRegular, newRegular, safety} {
		if err := os.WriteFile(path, []byte("db"), 0o600); err != nil {
			t.Fatalf("write backup %s: %v", path, err)
		}
	}

	removed, err := CleanupByMaxCount(root, 1)
	if err != nil {
		t.Fatalf("CleanupByMaxCount returned error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected one regular backup removed, got %d", removed)
	}
	if _, err := os.Stat(oldRegular); !os.IsNotExist(err) {
		t.Fatalf("expected old regular backup removed, stat err %v", err)
	}
	for _, path := range []string{newRegular, safety} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected backup %s to remain: %v", path, err)
		}
	}
}

func createStorageDomainTables(t *testing.T, db *sql.DB) {
	t.Helper()
	statTables := []string{
		"usage_request_details",
		"usage_events",
		"usage_overview_hourly_stats",
		"usage_overview_daily_stats",
		"usage_overview_health_stats",
		"usage_overview_aggregation_checkpoints",
		"cpa_api_keys",
		"redis_usage_inboxes",
		"model_price_settings",
	}
	for _, table := range statTables {
		if _, err := db.Exec(`CREATE TABLE ` + table + ` (id INTEGER PRIMARY KEY)`); err != nil {
			t.Fatalf("create %s: %v", table, err)
		}
		if _, err := db.Exec(`INSERT INTO ` + table + ` (id) VALUES (1)`); err != nil {
			t.Fatalf("insert %s: %v", table, err)
		}
	}
	if _, err := db.Exec(`CREATE TABLE usage_identities (
		id INTEGER PRIMARY KEY,
		total_requests INTEGER,
		success_count INTEGER,
		failure_count INTEGER,
		input_tokens INTEGER,
		output_tokens INTEGER,
		reasoning_tokens INTEGER,
		cached_tokens INTEGER,
		total_tokens INTEGER,
		last_aggregated_usage_event_id INTEGER,
		first_used_at TEXT,
		last_used_at TEXT,
		stats_updated_at TEXT
	)`); err != nil {
		t.Fatalf("create usage_identities: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO usage_identities (id, total_requests) VALUES (1, 1)`); err != nil {
		t.Fatalf("insert usage_identities: %v", err)
	}
}

func storageTableCount(t *testing.T, db *sql.DB, table string) int64 {
	t.Helper()
	var count int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func openTestSQLiteDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}
