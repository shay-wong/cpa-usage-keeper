package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository/dto"
	"cpa-usage-keeper/internal/service"
	servicedto "cpa-usage-keeper/internal/service/dto"
)

type usageMonitoringStub struct {
	monitoring      *service.UsageMonitoringSnapshot
	err             error
	lastFilter      servicedto.UsageFilter
	monitoringCalls int
}

func (s *usageMonitoringStub) GetUsageWithFilter(context.Context, servicedto.UsageFilter) (*dto.StatisticsSnapshot, error) {
	return nil, s.err
}

func (s *usageMonitoringStub) GetUsageOverview(context.Context, servicedto.UsageFilter) (*servicedto.UsageOverviewSnapshot, error) {
	return nil, s.err
}

func (s *usageMonitoringStub) ListUsageEvents(context.Context, servicedto.UsageFilter) (*servicedto.UsageEventsPage, error) {
	return nil, s.err
}

func (s *usageMonitoringStub) ListUsageEventFilterOptions(context.Context, servicedto.UsageFilter) (*servicedto.UsageEventFilterOptions, error) {
	return nil, s.err
}

func (s *usageMonitoringStub) GetUsageEventRequestDetail(context.Context, string) (*servicedto.UsageEventRequestDetail, error) {
	return nil, s.err
}

func (s *usageMonitoringStub) GetAnalysis(context.Context, servicedto.UsageFilter) (*servicedto.AnalysisSnapshot, error) {
	return nil, s.err
}

func (s *usageMonitoringStub) GetUsageMonitoring(_ context.Context, filter servicedto.UsageFilter) (*service.UsageMonitoringSnapshot, error) {
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
			ID: 42, Timestamp: requestTime, Model: "claude-sonnet", ReasoningEffort: "xhigh", Source: "sk-provider-key", AuthIndex: "2", Failed: true, LatencyMS: 321, InputTokens: 30, OutputTokens: 9, ReasoningTokens: 2, CachedTokens: 1, TotalTokens: 42,
		}},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{ID: 2, Name: "OpenAI Mirror", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "sk-provider-key", Type: "openai", Provider: "OpenAI Mirror"}}}})
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
	if !contains(body, `"source":"OpenAI Mirror"`) {
		t.Fatalf("expected monitoring source to reuse safe display, got %s", body)
	}
	if contains(body, "sk-provider-key") {
		t.Fatalf("expected monitoring payload to hide raw provider key, got %s", body)
	}
	if !contains(body, `"source_type":"openai"`) || !contains(body, `"source_key":"provider:2"`) {
		t.Fatalf("expected resolved source metadata, got %s", body)
	}
	if !contains(body, `"request_logs":[{"id":42`) || !contains(body, `"latency_ms":321`) || !contains(body, `"reasoning_effort":"xhigh"`) || !contains(body, `"total_tokens":42`) {
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
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{err: errors.New("identity store unavailable")}})
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
	if !contains(body, `"source":"openai"`) {
		t.Fatalf("expected fallback source display to match usage events, got %s", body)
	}
	if contains(body, "sk-provider-key") {
		t.Fatalf("expected fallback source to hide raw provider key, got %s", body)
	}
	if !contains(body, `"source_key":"provider:fallback:openai:redacted_api_`) {
		t.Fatalf("expected stable fallback source key, got %s", body)
	}
	if provider.monitoringCalls != 1 {
		t.Fatalf("expected GetUsageMonitoring to be called once, got %d", provider.monitoringCalls)
	}
}

func TestUsageMonitoringResolvesProviderByLookupKeyAndAuthIndex(t *testing.T) {
	requestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	provider := &usageMonitoringStub{monitoring: &service.UsageMonitoringSnapshot{
		KPIs: service.UsageMonitoringKPI{TotalRequests: 2},
		ChannelStats: []service.UsageMonitoringChannelStat{{
			Source: "sk-provider-key", AuthIndex: "codex-auth", TotalRequests: 1, SuccessRequests: 1, LastRequestTime: &requestTime,
		}, {
			Source: "sk-other-key", AuthIndex: "codex-auth", TotalRequests: 1, SuccessRequests: 1, LastRequestTime: &requestTime,
		}},
		RequestLogs: []service.UsageMonitoringRequestLog{{
			ID: 31, Timestamp: requestTime, Model: "codex-model", Source: "sk-provider-key", AuthIndex: "codex-auth", Failed: false,
		}, {
			ID: 32, Timestamp: requestTime, Model: "codex-model", Source: "sk-other-key", AuthIndex: "codex-auth", Failed: false,
		}},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{ID: 3, Name: "codex", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "codex-auth", Type: "codex", Provider: "codex", LookupKey: "sk-provider-key", BaseURL: "https://codex101.site/v1"}}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?range=24h", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if !contains(body, `"source":"codex(codex101.site)"`) {
		t.Fatalf("expected codex provider display name, got %s", body)
	}
	if contains(body, "sk-provider-key") || contains(body, "sk-other-key") {
		t.Fatalf("expected codex provider raw keys to stay hidden, got %s", body)
	}
	if !contains(body, `"source_type":"codex"`) || !contains(body, `"source_key":"provider:3"`) {
		t.Fatalf("expected codex provider metadata, got %s", body)
	}
	if contains(body, `"source_type":"openai"`) || contains(body, `"source_key":"provider:fallback:openai`) {
		t.Fatalf("expected codex provider fields only, got %s", body)
	}
}

func TestUsageMonitoringMergesRowsByResolvedSourceKey(t *testing.T) {
	firstRequestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	secondRequestTime := firstRequestTime.Add(time.Minute)
	provider := &usageMonitoringStub{monitoring: &service.UsageMonitoringSnapshot{
		KPIs: service.UsageMonitoringKPI{TotalRequests: 4, SuccessRequests: 2, FailedRequests: 2},
		ChannelStats: []service.UsageMonitoringChannelStat{{
			Source: "sk-provider-key", AuthIndex: "codex-auth", TotalRequests: 2, SuccessRequests: 1, FailedRequests: 1, LastRequestTime: &firstRequestTime,
			RecentRequests: []service.UsageMonitoringRecentRequest{{Timestamp: firstRequestTime, Failed: false}, {Timestamp: firstRequestTime.Add(time.Second), Failed: true}},
			Models:         []service.UsageMonitoringChannelModelStat{{Model: "gpt-5.5", Requests: 2, Success: 1, Failed: 1, LastRequestTime: &firstRequestTime}},
		}, {
			Source: "sk-other-key", AuthIndex: "codex-auth", TotalRequests: 2, SuccessRequests: 1, FailedRequests: 1, LastRequestTime: &secondRequestTime,
			RecentRequests: []service.UsageMonitoringRecentRequest{{Timestamp: secondRequestTime, Failed: false}, {Timestamp: secondRequestTime.Add(time.Second), Failed: true}},
			Models:         []service.UsageMonitoringChannelModelStat{{Model: "gpt-5.5", Requests: 1, Success: 1, Failed: 0, LastRequestTime: &secondRequestTime}, {Model: "gpt-5.4", Requests: 1, Success: 0, Failed: 1, LastRequestTime: &secondRequestTime}},
		}},
		FailureAnalysis: []service.UsageMonitoringFailureStat{{
			Source: "sk-provider-key", AuthIndex: "codex-auth", FailedCount: 1, LastFailTime: &firstRequestTime,
			Models: []service.UsageMonitoringFailureModelStat{{Model: "gpt-5.5", Failure: 1, Total: 2, Success: 1, LastTimestamp: &firstRequestTime}},
		}, {
			Source: "sk-other-key", AuthIndex: "codex-auth", FailedCount: 1, LastFailTime: &secondRequestTime,
			Models: []service.UsageMonitoringFailureModelStat{{Model: "gpt-5.4", Failure: 1, Total: 1, LastTimestamp: &secondRequestTime}},
		}},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{ID: 3, Name: "codex", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "codex-auth", Type: "codex", Provider: "codex", LookupKey: "sk-provider-key", BaseURL: "https://codex101.site/v1"}}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?range=24h", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if countSubstring(body, `"source_key":"provider:3"`) != 2 {
		t.Fatalf("expected one channel row and one failure row for provider:3, got %s", body)
	}
	if !contains(body, `"source_type":"codex","source_key":"provider:3","total_requests":4,"success_requests":2,"failed_requests":2`) {
		t.Fatalf("expected merged channel stats, got %s", body)
	}
	if !contains(body, `"models":[{"model":"gpt-5.5","requests":3,"success":2,"failed":1`) || !contains(body, `{"model":"gpt-5.4","requests":1,"success":0,"failed":1`) {
		t.Fatalf("expected merged channel model stats, got %s", body)
	}
	if !contains(body, `"source_type":"codex","source_key":"provider:3","failed_count":2`) {
		t.Fatalf("expected merged failure stats, got %s", body)
	}
	if !contains(body, `"models":[{"model":"gpt-5.4","success":0,"failure":1,"total":1`) || !contains(body, `{"model":"gpt-5.5","success":1,"failure":1,"total":2`) {
		t.Fatalf("expected merged failure model stats, got %s", body)
	}
}

func TestUsageMonitoringMergesBeforeLimitingResolvedSourcesAndModels(t *testing.T) {
	requestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	channelStats := make([]service.UsageMonitoringChannelStat, 0, 12)
	failureStats := make([]service.UsageMonitoringFailureStat, 0, 12)
	for i := 0; i < 12; i++ {
		rawSource := fmt.Sprintf("sk-provider-key-%02d", i)
		model := fmt.Sprintf("model-%02d", i)
		channelStats = append(channelStats, service.UsageMonitoringChannelStat{
			Source: rawSource, AuthIndex: "shared-auth", TotalRequests: int64(i + 1), SuccessRequests: int64(i + 1), LastRequestTime: &requestTime,
			Models: []service.UsageMonitoringChannelModelStat{{Model: model, Requests: int64(i + 1), Success: int64(i + 1), LastRequestTime: &requestTime}},
		})
		failureStats = append(failureStats, service.UsageMonitoringFailureStat{
			Source: rawSource, AuthIndex: "shared-auth", FailedCount: int64(i + 1), LastFailTime: &requestTime,
			Models: []service.UsageMonitoringFailureModelStat{{Model: model, Failure: int64(i + 1), Total: int64(i + 1), LastTimestamp: &requestTime}},
		})
	}
	provider := &usageMonitoringStub{monitoring: &service.UsageMonitoringSnapshot{
		KPIs:            service.UsageMonitoringKPI{TotalRequests: 78, SuccessRequests: 78, FailedRequests: 78},
		ChannelStats:    channelStats,
		FailureAnalysis: failureStats,
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{ID: 9, Name: "Shared Provider", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "shared-auth", Type: "openai", Provider: "Shared Provider"}}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?range=24h", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if countSubstring(body, `"source_key":"provider:9"`) != 2 {
		t.Fatalf("expected one merged channel row and one merged failure row, got %s", body)
	}
	if !contains(body, `"source_type":"openai","source_key":"provider:9","total_requests":78,"success_requests":78`) {
		t.Fatalf("expected merged channel totals to include all raw sources, got %s", body)
	}
	if !contains(body, `"source_type":"openai","source_key":"provider:9","failed_count":78`) {
		t.Fatalf("expected merged failure totals to include all raw sources, got %s", body)
	}
	if contains(body, `"model":"model-01"`) || contains(body, `"model":"model-00"`) {
		t.Fatalf("expected low ranked merged models to be limited after merge, got %s", body)
	}
	if !contains(body, `"model":"model-11"`) || !contains(body, `"model":"model-02"`) {
		t.Fatalf("expected top merged models to remain after limit, got %s", body)
	}
}

func TestUsageMonitoringFallbackProviderSourcesStaySeparate(t *testing.T) {
	requestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	provider := &usageMonitoringStub{monitoring: &service.UsageMonitoringSnapshot{
		KPIs: service.UsageMonitoringKPI{TotalRequests: 3, SuccessRequests: 3},
		ChannelStats: []service.UsageMonitoringChannelStat{{
			Source: "sk-first-openai-key", TotalRequests: 1, SuccessRequests: 1, LastRequestTime: &requestTime,
		}, {
			Source: "sk-second-openai-key", TotalRequests: 2, SuccessRequests: 2, LastRequestTime: &requestTime,
		}},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{err: errors.New("identity store unavailable")}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?range=24h", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if countSubstring(body, `"source":"openai"`) != 2 {
		t.Fatalf("expected two fallback openai rows, got %s", body)
	}
	if countSubstring(body, `"source_key":"provider:fallback:openai:redacted_api_`) != 2 {
		t.Fatalf("expected fallback provider source keys to stay unique, got %s", body)
	}
	if contains(body, "sk-first-openai-key") || contains(body, "sk-second-openai-key") {
		t.Fatalf("expected fallback rows to hide raw keys, got %s", body)
	}
}

func TestUsageMonitoringShowsAuthFileSourceWithoutMasking(t *testing.T) {
	requestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	rawName := "wang@example.com"
	provider := &usageMonitoringStub{monitoring: &service.UsageMonitoringSnapshot{
		KPIs: service.UsageMonitoringKPI{TotalRequests: 1, SuccessRequests: 1},
		ChannelStats: []service.UsageMonitoringChannelStat{{
			Source: "ignored-source", AuthIndex: "auth-file-index", TotalRequests: 1, SuccessRequests: 1, LastRequestTime: &requestTime,
		}},
		RequestLogs: []service.UsageMonitoringRequestLog{{
			ID: 89, Timestamp: requestTime, Model: "codex-model", Source: "ignored-source", AuthIndex: "auth-file-index", Failed: false, TotalTokens: 12,
		}},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{
		ID: 7, Name: rawName, AuthType: entities.UsageIdentityAuthTypeAuthFile, AuthTypeName: "oauth", Identity: "auth-file-index", Type: "codex", Provider: "codex", TotalRequests: 1,
	}}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?range=24h", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if !contains(body, `"source":"wang@example.com"`) {
		t.Fatalf("expected auth file source to stay unmasked in monitoring payload, got %s", body)
	}
	if !contains(body, `"source_type":"codex"`) {
		t.Fatalf("expected auth file source type to remain visible, got %s", body)
	}
}

func TestUsageMonitoringShowsEmailSourceWithoutMasking(t *testing.T) {
	requestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	rawEmail := "user@example.com"
	provider := &usageMonitoringStub{monitoring: &service.UsageMonitoringSnapshot{
		KPIs: service.UsageMonitoringKPI{TotalRequests: 1, SuccessRequests: 1},
		ChannelStats: []service.UsageMonitoringChannelStat{{
			Source: rawEmail, AuthIndex: "oauth-missing", TotalRequests: 1, SuccessRequests: 1, LastRequestTime: &requestTime,
		}},
		RequestLogs: []service.UsageMonitoringRequestLog{{
			ID: 88, Timestamp: requestTime, Model: "claude-sonnet", Source: rawEmail, AuthIndex: "oauth-missing", Failed: false, TotalTokens: 12,
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
	if !contains(body, `"source":"user@example.com"`) {
		t.Fatalf("expected email source to stay unmasked, got %s", body)
	}
	if !contains(body, `"source_key":"email:redacted_api_`) {
		t.Fatalf("expected stable email source key, got %s", body)
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/monitoring?range=24h&log_limit=bad", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.Code)
	}
	if !contains(resp.Body.String(), `"invalid log_limit`) {
		t.Fatalf("expected log_limit error, got %s", resp.Body.String())
	}
}
