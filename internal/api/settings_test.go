package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/service"
	servicedto "cpa-usage-keeper/internal/service/dto"
)

type databaseSettingsStub struct {
	settings                 entities.DatabaseCleanupSettings
	currentDatabaseSizeBytes *int64
	lastUpdate               *servicedto.UpdateDatabaseCleanupSettingsInput
	lastCreateBackup         *servicedto.CreateBackupInput
	lastRestoreBackup        *servicedto.RestoreBackupInput
	err                      error
}

func (s *databaseSettingsStub) GetDatabaseCleanupSettings(context.Context) (service.DatabaseCleanupSettingsSnapshot, error) {
	return service.DatabaseCleanupSettingsSnapshot{Settings: s.settings, CurrentDatabaseSizeBytes: s.currentDatabaseSizeBytes}, s.err
}

func (s *databaseSettingsStub) UpdateDatabaseCleanupSettings(_ context.Context, input servicedto.UpdateDatabaseCleanupSettingsInput) (service.DatabaseCleanupSettingsSnapshot, error) {
	s.lastUpdate = &input
	if input.RequestLogRetentionDays < 0 || input.UsageLogRetentionDays < 0 || input.MaxDatabaseSizeMB < 0 || input.MaxBackupCount < 0 {
		return service.DatabaseCleanupSettingsSnapshot{}, fmt.Errorf("%w: database cleanup settings must be non-negative", service.ErrStorageSettingsInvalid)
	}
	return service.DatabaseCleanupSettingsSnapshot{Settings: entities.DatabaseCleanupSettings{RequestLogRetentionDays: input.RequestLogRetentionDays, MaxDatabaseSizeMB: input.MaxDatabaseSizeMB}, CurrentDatabaseSizeBytes: s.currentDatabaseSizeBytes}, s.err
}

func (s *databaseSettingsStub) GetStorageInfo(context.Context) (servicedto.StorageInfo, error) {
	backupSizeBytes := int64(1024)
	return servicedto.StorageInfo{
		Settings: servicedto.UpdateDatabaseCleanupSettingsInput{
			RecordRequestDetails: true,
			CleanupRequestLogs:   true,
			BackupUsageLogs:      true,
			BackupHour:           4,
			MaxBackupCount:       1,
		},
		CurrentDatabaseSizeBytes: s.currentDatabaseSizeBytes,
		BackupTotalSizeBytes:     backupSizeBytes,
		BackupCount:              1,
		Domains: []servicedto.StorageDomainInfo{
			{Key: "usage_logs", Label: "用量日志", Description: "usage events", TableNames: []string{"usage_events"}, Rows: 2, SizeBytes: 512},
		},
		Backups: []servicedto.BackupFileInfo{
			{ID: "2026-05-24/database.db", FileName: "database.db", SizeBytes: 1024, CreatedAt: "2026-05-24T04:00:00+08:00"},
		},
	}, s.err
}

func (s *databaseSettingsStub) CreateBackup(_ context.Context, input servicedto.CreateBackupInput) (servicedto.BackupOperationResult, error) {
	s.lastCreateBackup = &input
	return servicedto.BackupOperationResult{}, s.err
}

func (s *databaseSettingsStub) RestoreBackup(_ context.Context, input servicedto.RestoreBackupInput) (servicedto.RestoreOperationResult, error) {
	s.lastRestoreBackup = &input
	return servicedto.RestoreOperationResult{}, s.err
}

func TestDatabaseSettingsRoutesReturnConfiguredData(t *testing.T) {
	sizeBytes := int64(2048)
	provider := &databaseSettingsStub{settings: entities.DatabaseCleanupSettings{RequestLogRetentionDays: 30, MaxDatabaseSizeMB: 512}, currentDatabaseSizeBytes: &sizeBytes}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{DatabaseSettings: provider})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/database", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK || !contains(body, `"request_log_retention_days":30`) || !contains(body, `"max_database_size_mb":512`) || !contains(body, `"current_database_size_bytes":2048`) {
		t.Fatalf("unexpected database settings response: %d %s", resp.Code, body)
	}
}

func TestUpdateDatabaseSettingsRoute(t *testing.T) {
	sizeBytes := int64(4096)
	provider := &databaseSettingsStub{currentDatabaseSizeBytes: &sizeBytes}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{DatabaseSettings: provider})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/database", strings.NewReader(`{"request_log_retention_days":7,"max_database_size_mb":256}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !contains(resp.Body.String(), `"request_log_retention_days":7`) || !contains(resp.Body.String(), `"current_database_size_bytes":4096`) {
		t.Fatalf("unexpected update response: %d %s", resp.Code, resp.Body.String())
	}
	if provider.lastUpdate == nil || provider.lastUpdate.RequestLogRetentionDays != 7 || provider.lastUpdate.MaxDatabaseSizeMB != 256 {
		t.Fatalf("expected update payload to be passed through, got %+v", provider.lastUpdate)
	}
}

func TestStorageInfoRouteReturnsSnakeCaseData(t *testing.T) {
	sizeBytes := int64(4096)
	provider := &databaseSettingsStub{currentDatabaseSizeBytes: &sizeBytes}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{DatabaseSettings: provider})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/storage", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK || !contains(body, `"settings"`) || !contains(body, `"max_backup_count":1`) || !contains(body, `"backup_hour":4`) || !contains(body, `"current_database_size_bytes":4096`) {
		t.Fatalf("unexpected storage info response: %d %s", resp.Code, body)
	}
	if !contains(body, `"table_names":["usage_events"]`) || !contains(body, `"file_name":"database.db"`) || contains(body, `"Settings"`) || contains(body, `"MaxBackupCount"`) {
		t.Fatalf("expected snake_case storage info response, got %s", body)
	}
}

func TestCreateStorageBackupRoutePassesAllDomains(t *testing.T) {
	provider := &databaseSettingsStub{}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{DatabaseSettings: provider})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/storage/backups", strings.NewReader(`{"request_logs":true,"usage_logs":true,"usage_identities":true,"api_keys":true,"redis_inbox":true,"model_prices":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected create backup response: %d %s", resp.Code, resp.Body.String())
	}
	if provider.lastCreateBackup == nil || !provider.lastCreateBackup.RequestLogs || !provider.lastCreateBackup.UsageLogs || !provider.lastCreateBackup.UsageIdentities || !provider.lastCreateBackup.APIKeys || !provider.lastCreateBackup.RedisInbox || !provider.lastCreateBackup.ModelPrices {
		t.Fatalf("expected all backup domains to be passed through, got %+v", provider.lastCreateBackup)
	}
}

func TestRestoreStorageBackupRoutePassesDomainsAndSkipSafetyBackup(t *testing.T) {
	provider := &databaseSettingsStub{}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{DatabaseSettings: provider})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/storage/restore", strings.NewReader(`{"id":"2026-05-24/database.db","request_logs":true,"usage_logs":true,"usage_identities":true,"api_keys":true,"redis_inbox":true,"model_prices":true,"skip_safety_backup":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected restore backup response: %d %s", resp.Code, resp.Body.String())
	}
	if provider.lastRestoreBackup == nil || provider.lastRestoreBackup.ID != "2026-05-24/database.db" || !provider.lastRestoreBackup.RequestLogs || !provider.lastRestoreBackup.UsageLogs || !provider.lastRestoreBackup.UsageIdentities || !provider.lastRestoreBackup.APIKeys || !provider.lastRestoreBackup.RedisInbox || !provider.lastRestoreBackup.ModelPrices || !provider.lastRestoreBackup.SkipSafetyBackup {
		t.Fatalf("expected restore payload to be passed through, got %+v", provider.lastRestoreBackup)
	}
}
