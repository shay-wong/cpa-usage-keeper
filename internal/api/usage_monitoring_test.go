package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cpa-usage-keeper/internal/cpa"
	"cpa-usage-keeper/internal/models"
	"cpa-usage-keeper/internal/service"
)

type usageMonitoringStub struct {
	monitoring      *service.UsageMonitoringSnapshot
	err             error
	lastFilter      service.UsageFilter
	monitoringCalls int
}

func (s *usageMonitoringStub) GetUsageWithFilter(context.Context, service.UsageFilter) (*cpa.StatisticsSnapshot, error) {
	return nil, s.err
}

func (s *usageMonitoringStub) GetUsageOverview(context.Context, service.UsageFilter) (*service.UsageOverviewSnapshot, error) {
	return nil, s.err
}

func (s *usageMonitoringStub) ListUsageEvents(context.Context, service.UsageFilter) (*service.UsageEventsPage, error) {
	return nil, s.err
}

func (s *usageMonitoringStub) ListUsageEventFilterOptions(context.Context, service.UsageFilter) (*service.UsageEventFilterOptions, error) {
	return nil, s.err
}

func (s *usageMonitoringStub) ListUsageCredentialStats(context.Context, service.UsageFilter) ([]service.UsageCredentialStat, error) {
	return nil, s.err
}

func (s *usageMonitoringStub) GetUsageAnalysis(context.Context, service.UsageFilter) (*service.UsageAnalysisSnapshot, error) {
	return nil, s.err
}

func (s *usageMonitoringStub) GetUsageMonitoring(_ context.Context, filter service.UsageFilter) (*service.UsageMonitoringSnapshot, error) {
	s.lastFilter = filter
	s.monitoringCalls++
	return s.monitoring, s.err
}

func TestUsageMonitoringReturnsResolvedPayload(t *testing.T) {
	requestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	provider := &usageMonitoringStub{monitoring: &service.UsageMonitoringSnapshot{
		KPIs: service.UsageMonitoringKPI{
			TotalRequests:   2,
			SuccessRequests: 1,
			FailedRequests:  1,
			TotalTokens:     42,
			InputTokens:     30,
			OutputTokens:    9,
			CachedTokens:    1,
			ReasoningTokens: 2,
			RPM:             0.5,
			TPM:             10.5,
			TotalCost:       0.123,
			CostAvailable:   true,
		},
		ModelDistribution: []service.UsageMonitoringModelDistributionItem{{
			Model: "claude-sonnet", TotalRequests: 2, SuccessCount: 1, FailureCount: 1, TotalTokens: 42, InputTokens: 30, OutputTokens: 9, CachedTokens: 1, ReasoningTokens: 2, SuccessRate: 50,
		}},
		DailyTrend:       []service.UsageMonitoringTrendPoint{{Bucket: "2026-04-22", Requests: 2, Tokens: 42, InputTokens: 30, OutputTokens: 9, CachedTokens: 1, ReasoningTokens: 2, Cost: 0.123}},
		HourlyModelTrend: []service.UsageMonitoringHourlyModelPoint{{Hour: "2026-04-22T11:00:00Z", Models: []service.UsageMonitoringHourlyModelStat{{Model: "claude-sonnet", Requests: 2, Tokens: 42, SuccessCount: 1, FailureCount: 1}}}},
		HourlyTokenTrend: []service.UsageMonitoringTrendPoint{{Bucket: "2026-04-22T11:00:00Z", Tokens: 42, InputTokens: 30, OutputTokens: 9, CachedTokens: 1, ReasoningTokens: 2, Cost: 0.123}},
		ChannelStats: []service.UsageMonitoringChannelStat{{
			Source: "sk-provider-key", AuthIndex: "2", TotalRequests: 2, SuccessRequests: 1, FailedRequests: 1, TotalTokens: 42, InputTokens: 30, OutputTokens: 9, CachedTokens: 1, ReasoningTokens: 2, SuccessRate: 50, LastRequestTime: &requestTime, RecentRequests: []service.UsageMonitoringRecentRequest{{Timestamp: requestTime, Failed: true}},
			Models: []service.UsageMonitoringChannelModelStat{{Model: "claude-sonnet", Requests: 2, Success: 1, Failed: 1, SuccessRate: 50, TotalTokens: 42, LastRequestTime: &requestTime, RecentRequests: []service.UsageMonitoringRecentRequest{{Timestamp: requestTime, Failed: false}}}},
		}},
		FailureAnalysis: []service.UsageMonitoringFailureStat{{
			Source: "sk-provider-key", AuthIndex: "2", FailedCount: 1, LastFailTime: &requestTime,
			Models: []service.UsageMonitoringFailureModelStat{{Model: "claude-sonnet", Success: 1, Failure: 1, Total: 2, SuccessRate: 50, LastTimestamp: &requestTime, RecentRequests: []service.UsageMonitoringRecentRequest{{Timestamp: requestTime, Failed: true}}}},
		}},
		RequestLogs: []service.UsageMonitoringRequestLog{{
			ID: 42, Timestamp: requestTime, Model: "claude-sonnet", Source: "sk-provider-key", AuthIndex: "2", Failed: true, LatencyMS: 321, InputTokens: 30, OutputTokens: 9, ReasoningTokens: 2, CachedTokens: 1, TotalTokens: 42,
		}},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", usageIdentitiesStub{items: []models.UsageIdentity{{ID: 2, Name: "OpenAI Mirror", AuthType: models.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "sk-provider-key", Type: "openai", Provider: "OpenAI Mirror"}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?range=24h&log_limit=250", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if !contains(body, `"kpis":{"total_requests":2`) || !contains(body, `"model_distribution":[`) || !contains(body, `"daily_trend":[{"date":"2026-04-22"`) {
		t.Fatalf("expected monitoring payload, got %s", body)
	}
	if !contains(body, `"source":"OpenAI Mirror"`) || !contains(body, `"source_type":"openai"`) || !contains(body, `"source_key":"provider:2"`) {
		t.Fatalf("expected resolved source fields, got %s", body)
	}
	if contains(body, "sk-provider-key") {
		t.Fatalf("expected raw source to be redacted, got %s", body)
	}
	if !contains(body, `"request_logs":[{"id":42`) || !contains(body, `"latency_ms":321`) || !contains(body, `"total_tokens":42`) {
		t.Fatalf("expected request log fields, got %s", body)
	}
	if provider.monitoringCalls != 1 {
		t.Fatalf("expected GetUsageMonitoring to be called once, got %d", provider.monitoringCalls)
	}
	if provider.lastFilter.Range != "24h" || provider.lastFilter.Limit != 250 {
		t.Fatalf("expected range and log_limit filter, got %+v", provider.lastFilter)
	}
}

func TestUsageMonitoringReturnsPayloadWhenIdentityResolutionFails(t *testing.T) {
	requestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	provider := &usageMonitoringStub{monitoring: &service.UsageMonitoringSnapshot{
		KPIs: service.UsageMonitoringKPI{TotalRequests: 1},
		RequestLogs: []service.UsageMonitoringRequestLog{{
			ID: 7, Timestamp: requestTime, Model: "claude-sonnet", Source: "sk-provider-key", Failed: false, TotalTokens: 10,
		}},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", usageIdentitiesStub{err: errors.New("identity store unavailable")})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?range=24h", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if !contains(body, `"kpis":{"total_requests":1`) || !contains(body, `"request_logs":[{"id":7`) {
		t.Fatalf("expected monitoring payload when identity resolution fails, got %s", body)
	}
	if !contains(body, `"source":"openai"`) || !contains(body, `"source_key":"provider:fallback:openai"`) {
		t.Fatalf("expected fallback source resolution, got %s", body)
	}
	if provider.monitoringCalls != 1 {
		t.Fatalf("expected GetUsageMonitoring to be called once, got %d", provider.monitoringCalls)
	}
}

func TestUsageMonitoringDoesNotExposeAuthIdentityFallbackIndex(t *testing.T) {
	requestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	provider := &usageMonitoringStub{monitoring: &service.UsageMonitoringSnapshot{
		KPIs: service.UsageMonitoringKPI{TotalRequests: 1},
		ChannelStats: []service.UsageMonitoringChannelStat{{
			AuthIndex: "auth-secret", TotalRequests: 1, SuccessRequests: 1, LastRequestTime: &requestTime,
		}},
		RequestLogs: []service.UsageMonitoringRequestLog{{
			ID: 11, Timestamp: requestTime, Model: "claude-sonnet", AuthIndex: "auth-secret", Failed: false, TotalTokens: 10,
		}},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", usageIdentitiesStub{items: []models.UsageIdentity{{ID: 1, AuthType: models.UsageIdentityAuthTypeAuthFile, AuthTypeName: "oauth", Identity: "auth-secret", Type: "claude", Provider: "Claude"}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?range=24h", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if contains(body, "auth-secret") || contains(body, `"auth_index"`) {
		t.Fatalf("expected raw auth index to be hidden, got %s", body)
	}
	if !contains(body, `"source_key":"auth:redacted_api_`) {
		t.Fatalf("expected redacted auth source key, got %s", body)
	}
}

func TestUsageMonitoringDoesNotExposeAuthIndexOrRawEmailFallback(t *testing.T) {
	requestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	provider := &usageMonitoringStub{monitoring: &service.UsageMonitoringSnapshot{
		KPIs: service.UsageMonitoringKPI{TotalRequests: 1},
		ChannelStats: []service.UsageMonitoringChannelStat{{
			Source: "user@example.com", AuthIndex: "auth-secret", TotalRequests: 1, SuccessRequests: 1, LastRequestTime: &requestTime,
		}},
		FailureAnalysis: []service.UsageMonitoringFailureStat{{
			Source: "user@example.com", AuthIndex: "auth-secret", FailedCount: 1, LastFailTime: &requestTime,
		}},
		RequestLogs: []service.UsageMonitoringRequestLog{{
			ID: 9, Timestamp: requestTime, Model: "claude-sonnet", Source: "user@example.com", AuthIndex: "auth-secret", Failed: false, TotalTokens: 10,
		}},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?range=24h", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if contains(body, "user@example.com") || contains(body, "auth-secret") || contains(body, `"auth_index"`) {
		t.Fatalf("expected raw email and auth index to be hidden, got %s", body)
	}
	if !contains(body, `"source_key":"email:redacted_api_`) {
		t.Fatalf("expected redacted email source key, got %s", body)
	}
}

func TestUsageMonitoringRedactsAuthIdentityEmailDisplayName(t *testing.T) {
	requestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	provider := &usageMonitoringStub{monitoring: &service.UsageMonitoringSnapshot{
		KPIs: service.UsageMonitoringKPI{TotalRequests: 1},
		ChannelStats: []service.UsageMonitoringChannelStat{{
			AuthIndex: "auth-secret", TotalRequests: 1, SuccessRequests: 1, LastRequestTime: &requestTime,
		}},
		FailureAnalysis: []service.UsageMonitoringFailureStat{{
			AuthIndex: "auth-secret", FailedCount: 1, LastFailTime: &requestTime,
		}},
		RequestLogs: []service.UsageMonitoringRequestLog{{
			ID: 12, Timestamp: requestTime, Model: "claude-sonnet", AuthIndex: "auth-secret", Failed: false, TotalTokens: 10,
		}},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", usageIdentitiesStub{items: []models.UsageIdentity{{ID: 1, Name: "user@example.com", AuthType: models.UsageIdentityAuthTypeAuthFile, AuthTypeName: "oauth", Identity: "auth-secret", Type: "claude", Provider: "Claude"}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?range=24h", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if contains(body, "user@example.com") || contains(body, "auth-secret") || contains(body, `"auth_index"`) {
		t.Fatalf("expected auth email display and auth index to be hidden, got %s", body)
	}
	if !contains(body, `"source_key":"auth:redacted_api_`) {
		t.Fatalf("expected redacted auth source key, got %s", body)
	}
}

func TestUsageMonitoringNilProviderReturnsEmptyPayload(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?range=custom&start=2026-04-20&end=2026-04-21", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if !contains(body, `"model_distribution":[]`) || !contains(body, `"channel_stats":[]`) || !contains(body, `"request_logs":[]`) || !contains(body, `"timezone":`) {
		t.Fatalf("expected empty monitoring payload, got %s", body)
	}
}

func TestUsageMonitoringRejectsInvalidLogLimit(t *testing.T) {
	router := NewRouter(nil, nil, &usageMonitoringStub{}, nil, AuthConfig{}, nil, "")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?log_limit=bad", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.Code)
	}
	if !contains(resp.Body.String(), `"invalid log_limit`) {
		t.Fatalf("expected log_limit error, got %s", resp.Body.String())
	}
}
