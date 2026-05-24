package migration

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestUseInt64PrimaryKeysMigrationAcceptsPostgresBigSerial(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set POSTGRES_TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	db := openPostgresMigrationTestDatabase(t, databaseURL)

	for _, table := range []string{"usage_events", "usage_identities", "redis_usage_inboxes", "model_price_settings"} {
		if err := db.Exec("CREATE TABLE " + table + " (id BIGSERIAL PRIMARY KEY)").Error; err != nil {
			t.Fatalf("create postgres table %s: %v", table, err)
		}
	}

	if err := useInt64PrimaryKeysMigration(db); err != nil {
		t.Fatalf("useInt64PrimaryKeysMigration returned error: %v", err)
	}
}

func openPostgresMigrationTestDatabase(t *testing.T, databaseURL string) *gorm.DB {
	t.Helper()
	schemaName := fmt.Sprintf("keeper_migration_test_%d", time.Now().UnixNano())
	adminDB, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres admin database: %v", err)
	}
	closePostgresMigrationTestDatabase(t, adminDB)

	if err := adminDB.Exec("CREATE SCHEMA " + schemaName).Error; err != nil {
		t.Fatalf("create postgres test schema: %v", err)
	}
	t.Cleanup(func() {
		if err := adminDB.Exec("DROP SCHEMA " + schemaName + " CASCADE").Error; err != nil {
			t.Logf("drop postgres test schema %s: %v", schemaName, err)
		}
	})

	db, err := gorm.Open(postgres.Open(postgresMigrationURLWithSearchPath(t, databaseURL, schemaName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres test database: %v", err)
	}
	closePostgresMigrationTestDatabase(t, db)
	return db
}

func postgresMigrationURLWithSearchPath(t *testing.T, databaseURL string, schemaName string) string {
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

func closePostgresMigrationTestDatabase(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get postgres sql database: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close postgres test database: %v", err)
		}
	})
}
