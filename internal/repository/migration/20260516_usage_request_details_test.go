package migration

import (
	"path/filepath"
	"testing"
	"time"

	"cpa-usage-keeper/internal/entities"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateUsageRequestDetailsMigrationCreatesCacheTableAndUniqueIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "legacy.db"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeOpenedDatabase(t, db)

	if err := createUsageRequestDetailsMigration(db); err != nil {
		t.Fatalf("create usage request details: %v", err)
	}
	if err := createUsageRequestDetailsMigration(db); err != nil {
		t.Fatalf("create usage request details should be idempotent: %v", err)
	}
	if !db.Migrator().HasTable(&entities.UsageRequestDetail{}) {
		t.Fatal("expected usage_request_details table to exist")
	}
	if !db.Migrator().HasIndex(&entities.UsageRequestDetail{}, "uniq_usage_request_details_request_id") {
		t.Fatal("expected request_id unique index to exist")
	}

	now := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)
	detail := entities.UsageRequestDetail{RequestID: "req-1", Content: "raw log", Source: "cliproxyapi", FetchedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&detail).Error; err != nil {
		t.Fatalf("insert usage request detail: %v", err)
	}
	duplicate := entities.UsageRequestDetail{RequestID: "req-1", Content: "other", Source: "cliproxyapi", FetchedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("expected duplicate request_id insert to fail")
	}
}
