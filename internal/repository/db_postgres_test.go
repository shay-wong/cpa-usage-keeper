package repository

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"cpa-usage-keeper/internal/config"
	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository/dto"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestOpenDatabasePostgresAutoMigratesCoreTables(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set POSTGRES_TEST_DATABASE_URL to run PostgreSQL integration test")
	}

	schemaName := fmt.Sprintf("keeper_test_%d", time.Now().UnixNano())
	adminDB, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres admin database: %v", err)
	}
	closeTestDatabase(t, adminDB)

	if err := adminDB.Exec("CREATE SCHEMA " + schemaName).Error; err != nil {
		t.Fatalf("create postgres test schema: %v", err)
	}
	t.Cleanup(func() {
		if err := adminDB.Exec("DROP SCHEMA " + schemaName + " CASCADE").Error; err != nil {
			t.Logf("drop postgres test schema %s: %v", schemaName, err)
		}
	})

	db, err := OpenDatabase(config.Config{
		DatabaseDriver: config.DatabaseDriverPostgres,
		DatabaseURL:    postgresURLWithSearchPath(t, databaseURL, schemaName),
	})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)

	if db.Dialector.Name() != config.DatabaseDriverPostgres {
		t.Fatalf("expected postgres dialector, got %s", db.Dialector.Name())
	}
	if !db.Migrator().HasTable("usage_events") {
		t.Fatal("expected usage_events table to exist")
	}
	if !db.Migrator().HasTable("schema_migrations") {
		t.Fatal("expected schema_migrations table to exist")
	}
	assertPostgresColumnDataType(t, db, "usage_events", "timestamp", "timestamp with time zone")

	sizeBytes, exists, err := GetDatabaseSizeBytes(db, "")
	if err != nil {
		t.Fatalf("GetDatabaseSizeBytes returned error: %v", err)
	}
	if !exists || sizeBytes <= 0 {
		t.Fatalf("expected positive postgres database size, got exists=%v size=%d", exists, sizeBytes)
	}
}

func TestUsageMonitoringStatsRunsOnPostgres(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set POSTGRES_TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	db := openPostgresTestDatabase(t, databaseURL, "keeper_monitoring_test")

	base := time.Date(2026, 5, 25, 9, 30, 0, 0, time.UTC)
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "pg-hour-a", Source: "source-a", AuthIndex: "auth-1", Model: "claude-sonnet", Timestamp: base, TotalTokens: 10},
		{EventKey: "pg-hour-b", Source: "source-a", AuthIndex: "auth-1", Model: "claude-sonnet", Timestamp: base.Add(15 * time.Minute), Failed: true, TotalTokens: 20},
	}); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	hourlyRows, err := ListUsageMonitoringHourlyModelStatsWithFilter(context.Background(), db, dto.UsageQueryFilter{})
	if err != nil {
		t.Fatalf("ListUsageMonitoringHourlyModelStatsWithFilter returned error: %v", err)
	}
	if len(hourlyRows) != 1 || hourlyRows[0].Model != "claude-sonnet" || hourlyRows[0].Requests != 2 || hourlyRows[0].Tokens != 30 {
		t.Fatalf("unexpected postgres hourly stats rows: %+v", hourlyRows)
	}
	if hourlyRows[0].Hour != "2026-05-25T09:00:00Z" {
		t.Fatalf("expected UTC hour bucket, got %s", hourlyRows[0].Hour)
	}

	channelRows, channelModelRows, err := ListUsageMonitoringChannelStatsWithFilter(context.Background(), db, dto.UsageQueryFilter{})
	if err != nil {
		t.Fatalf("ListUsageMonitoringChannelStatsWithFilter returned error: %v", err)
	}
	if len(channelRows) != 1 || len(channelModelRows) != 1 {
		t.Fatalf("expected one channel and one model row, got channels=%+v models=%+v", channelRows, channelModelRows)
	}
	if channelModelRows[0].Model != "claude-sonnet" || channelModelRows[0].Requests != 2 || channelModelRows[0].Failed != 1 {
		t.Fatalf("unexpected postgres channel model rows: %+v", channelModelRows)
	}
}

func openPostgresTestDatabase(t *testing.T, databaseURL string, prefix string) *gorm.DB {
	t.Helper()
	schemaName := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	adminDB, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres admin database: %v", err)
	}
	closeTestDatabase(t, adminDB)

	if err := adminDB.Exec("CREATE SCHEMA " + schemaName).Error; err != nil {
		t.Fatalf("create postgres test schema: %v", err)
	}
	t.Cleanup(func() {
		if err := adminDB.Exec("DROP SCHEMA " + schemaName + " CASCADE").Error; err != nil {
			t.Logf("drop postgres test schema %s: %v", schemaName, err)
		}
	})

	db, err := OpenDatabase(config.Config{
		DatabaseDriver: config.DatabaseDriverPostgres,
		DatabaseURL:    postgresURLWithSearchPath(t, databaseURL, schemaName),
	})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	return db
}
func assertPostgresColumnDataType(t *testing.T, db *gorm.DB, table string, column string, want string) {
	t.Helper()
	var got string
	if err := db.Raw(`
		SELECT data_type
		FROM information_schema.columns
		WHERE table_schema = current_schema()
			AND table_name = ?
			AND column_name = ?
	`, table, column).Scan(&got).Error; err != nil {
		t.Fatalf("load postgres column type for %s.%s: %v", table, column, err)
	}
	if got != want {
		t.Fatalf("expected %s.%s data type %q, got %q", table, column, want, got)
	}
}

func postgresURLWithSearchPath(t *testing.T, databaseURL string, schemaName string) string {
	t.Helper()
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Scheme == "" {
		t.Fatalf("POSTGRES_TEST_DATABASE_URL must be a URL-style PostgreSQL DSN: %v", err)
	}
	query := parsed.Query()
	query.Set("search_path", schemaName)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
