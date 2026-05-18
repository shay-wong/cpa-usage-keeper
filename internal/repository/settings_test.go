package repository

import (
	"path/filepath"
	"testing"

	"cpa-usage-keeper/internal/config"
	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository/dto"
	"gorm.io/gorm"
)

func TestGetDatabaseCleanupSettingsReturnsDefaultsWhenEmpty(t *testing.T) {
	db := openSettingsTestDatabase(t)

	settings, err := GetDatabaseCleanupSettings(db)
	if err != nil {
		t.Fatalf("GetDatabaseCleanupSettings returned error: %v", err)
	}
	if settings.RequestLogRetentionDays != 0 || settings.MaxDatabaseSizeMB != 0 {
		t.Fatalf("expected cleanup defaults to be disabled, got %+v", settings)
	}
}

func TestUpsertDatabaseCleanupSettingsCreatesAndUpdatesSingleRow(t *testing.T) {
	db := openSettingsTestDatabase(t)

	created, err := UpsertDatabaseCleanupSettings(db, dto.DatabaseCleanupSettingsInput{RequestLogRetentionDays: 30, MaxDatabaseSizeMB: 512})
	if err != nil {
		t.Fatalf("create database cleanup settings: %v", err)
	}
	if created.RequestLogRetentionDays != 30 || created.MaxDatabaseSizeMB != 512 {
		t.Fatalf("unexpected created settings: %+v", created)
	}

	updated, err := UpsertDatabaseCleanupSettings(db, dto.DatabaseCleanupSettingsInput{RequestLogRetentionDays: 7, MaxDatabaseSizeMB: 256})
	if err != nil {
		t.Fatalf("update database cleanup settings: %v", err)
	}
	if updated.ID != created.ID || updated.RequestLogRetentionDays != 7 || updated.MaxDatabaseSizeMB != 256 {
		t.Fatalf("unexpected updated settings: %+v", updated)
	}

	var count int64
	if err := db.Model(&entities.DatabaseCleanupSettings{}).Count(&count).Error; err != nil {
		t.Fatalf("count database cleanup settings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one settings row, got %d", count)
	}
}

func openSettingsTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "settings.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	return db
}
