package api

import (
	"context"
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
	err                      error
}

func (s *databaseSettingsStub) GetDatabaseCleanupSettings(context.Context) (service.DatabaseCleanupSettingsSnapshot, error) {
	return service.DatabaseCleanupSettingsSnapshot{Settings: s.settings, CurrentDatabaseSizeBytes: s.currentDatabaseSizeBytes}, s.err
}

func (s *databaseSettingsStub) UpdateDatabaseCleanupSettings(_ context.Context, input servicedto.UpdateDatabaseCleanupSettingsInput) (service.DatabaseCleanupSettingsSnapshot, error) {
	s.lastUpdate = &input
	return service.DatabaseCleanupSettingsSnapshot{Settings: entities.DatabaseCleanupSettings{RequestLogRetentionDays: input.RequestLogRetentionDays, MaxDatabaseSizeMB: input.MaxDatabaseSizeMB}, CurrentDatabaseSizeBytes: s.currentDatabaseSizeBytes}, s.err
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

func TestUpdateDatabaseSettingsRejectsInvalidPayload(t *testing.T) {
	provider := &databaseSettingsStub{}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{DatabaseSettings: provider})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/database", strings.NewReader(`{"request_log_retention_days":-1,"max_database_size_mb":256}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d %s", resp.Code, resp.Body.String())
	}
}
