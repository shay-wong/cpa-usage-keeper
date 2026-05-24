package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cpa-usage-keeper/internal/config"
	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRunCopiesSQLiteRowsToPostgres(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("POSTGRES_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set POSTGRES_TEST_DATABASE_URL to run PostgreSQL integration test")
	}

	sourcePath := filepath.Join(t.TempDir(), "app.db")
	source, err := repository.OpenDatabase(config.Config{SQLitePath: sourcePath})
	if err != nil {
		t.Fatalf("open source sqlite database: %v", err)
	}
	closeTestDB(t, source)

	now := time.Date(2026, 5, 25, 9, 0, 0, 0, time.UTC)
	if err := source.Create(&entities.UsageEvent{
		ID:          41,
		EventKey:    "event-migrate",
		APIGroupKey: "provider-a",
		Model:       "claude-sonnet",
		Timestamp:   now,
		TotalTokens: 123,
		CreatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed usage event: %v", err)
	}
	if err := source.Create(&entities.ModelPriceSetting{
		ID:                   7,
		Model:                "claude-sonnet",
		PromptPricePer1M:     1,
		CompletionPricePer1M: 2,
		CreatedAt:            now,
		UpdatedAt:            now,
	}).Error; err != nil {
		t.Fatalf("seed model price setting: %v", err)
	}

	schemaName := fmt.Sprintf("keeper_migrate_test_%d", time.Now().UnixNano())
	adminDB, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres admin database: %v", err)
	}
	closeTestDB(t, adminDB)

	if err := adminDB.Exec("CREATE SCHEMA " + schemaName).Error; err != nil {
		t.Fatalf("create postgres test schema: %v", err)
	}
	t.Cleanup(func() {
		if err := adminDB.Exec("DROP SCHEMA " + schemaName + " CASCADE").Error; err != nil {
			t.Logf("drop postgres test schema %s: %v", schemaName, err)
		}
	})

	targetURL := postgresURLWithSearchPath(t, databaseURL, schemaName)
	if err := run(context.Background(), sourcePath, targetURL, false); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	target, err := repository.OpenDatabase(config.Config{DatabaseDriver: config.DatabaseDriverPostgres, DatabaseURL: targetURL})
	if err != nil {
		t.Fatalf("open target postgres database: %v", err)
	}
	closeTestDB(t, target)

	var usageEvent entities.UsageEvent
	if err := target.Where("event_key = ?", "event-migrate").First(&usageEvent).Error; err != nil {
		t.Fatalf("load migrated usage event: %v", err)
	}
	if usageEvent.ID != 41 || usageEvent.TotalTokens != 123 {
		t.Fatalf("unexpected migrated usage event: %+v", usageEvent)
	}

	if err := target.Create(&entities.UsageEvent{
		EventKey:    "event-after-migrate",
		APIGroupKey: "provider-a",
		Model:       "claude-opus",
		Timestamp:   now.Add(time.Hour),
		CreatedAt:   now.Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("insert post-migration usage event: %v", err)
	}
	var nextUsageEvent entities.UsageEvent
	if err := target.Where("event_key = ?", "event-after-migrate").First(&nextUsageEvent).Error; err != nil {
		t.Fatalf("load post-migration usage event: %v", err)
	}
	if nextUsageEvent.ID <= usageEvent.ID {
		t.Fatalf("expected postgres sequence to advance beyond %d, got %d", usageEvent.ID, nextUsageEvent.ID)
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

func closeTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	})
}
