package repository

import (
	"path/filepath"
	"testing"
	"time"

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
	if settings.MaxBackupCount != 1 {
		t.Fatalf("expected max backup count default 1, got %+v", settings)
	}
	if !settings.BackupUsageLogs || !settings.BackupUsageIdentities || !settings.BackupAPIKeys || !settings.BackupModelPrices {
		t.Fatalf("expected usage logs, credentials, API keys and model prices to be backed up by default, got %+v", settings)
	}
	if settings.BackupRequestLogs || settings.BackupRedisInbox {
		t.Fatalf("expected request logs and Redis inbox backup to default off, got %+v", settings)
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

func TestUpsertDatabaseCleanupSettingsPersistsExplicitZeroValuesOnCreate(t *testing.T) {
	db := openSettingsTestDatabase(t)

	created, err := UpsertDatabaseCleanupSettings(db, dto.DatabaseCleanupSettingsInput{
		RecordRequestDetails:  false,
		CleanupRequestLogs:    false,
		BackupUsageLogs:       false,
		BackupUsageIdentities: false,
		BackupAPIKeys:         false,
		BackupModelPrices:     false,
		MaxBackupCount:        0,
	})
	if err != nil {
		t.Fatalf("create database cleanup settings with zero values: %v", err)
	}
	if created.RecordRequestDetails || created.CleanupRequestLogs || created.BackupUsageLogs || created.BackupUsageIdentities || created.BackupAPIKeys || created.BackupModelPrices {
		t.Fatalf("expected explicit false values to persist on create, got %+v", created)
	}
	if created.MaxBackupCount != 0 {
		t.Fatalf("expected explicit max backup count 0 to persist on create, got %+v", created)
	}
}

func TestGetStorageDomainStatsIncludesConfigAndCacheDomains(t *testing.T) {
	db := openSettingsTestDatabase(t)
	now := time.Date(2026, 5, 24, 1, 0, 0, 0, time.UTC)
	if err := db.Create(&entities.UsageIdentity{Name: "Auth File", AuthType: entities.UsageIdentityAuthTypeAuthFile, AuthTypeName: "oauth", Identity: "auth-index", Type: "account", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed usage identity: %v", err)
	}
	if err := db.Create(&entities.CPAAPIKey{APIKey: "sk-test", DisplayKey: "sk-***test", LastSyncedAt: &now, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed cpa api key: %v", err)
	}
	if err := db.Create(&entities.RedisUsageInbox{QueueKey: "usage", MessageHash: "hash", RawMessage: "{}", Status: "pending", PoppedAt: now, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed redis inbox: %v", err)
	}
	if err := db.Create(&entities.ModelPriceSetting{Model: "claude-sonnet", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed model price: %v", err)
	}

	domains, err := GetStorageDomainStats(db)
	if err != nil {
		t.Fatalf("GetStorageDomainStats returned error: %v", err)
	}

	expected := map[string]string{
		"request_logs":     "usage_request_details",
		"usage_logs":       "usage_events",
		"usage_identities": "usage_identities",
		"api_keys":         "cpa_api_keys",
		"redis_inbox":      "redis_usage_inboxes",
		"model_prices":     "model_price_settings",
	}
	seen := make(map[string]StorageDomainStats, len(domains))
	for _, domain := range domains {
		seen[domain.Key] = domain
	}
	for key, table := range expected {
		domain, ok := seen[key]
		if !ok {
			t.Fatalf("expected storage domain %q in %+v", key, domains)
		}
		if len(domain.TableNames) == 0 || domain.TableNames[0] != table {
			t.Fatalf("expected domain %q to include table %q, got %+v", key, table, domain.TableNames)
		}
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
