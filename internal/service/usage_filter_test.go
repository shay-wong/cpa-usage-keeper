package service

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"cpa-usage-keeper/internal/config"
	"cpa-usage-keeper/internal/cpa"
	"cpa-usage-keeper/internal/cpa/dto/response"
	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository"
	"cpa-usage-keeper/internal/repository/dto"
	servicedto "cpa-usage-keeper/internal/service/dto"
	"gorm.io/gorm"
)

func TestUsageServiceGetUsageOverviewDelegatesToFilteredOverview(t *testing.T) {
	previousLocal := time.Local
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	t.Cleanup(func() { time.Local = previousLocal })
	time.Local = location

	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-overview.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	if _, err := repository.UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "claude-sonnet",
		PromptPricePer1M:     3,
		CompletionPricePer1M: 15,
		CachePricePer1M:      0.3,
	}); err != nil {
		t.Fatalf("UpsertModelPriceSetting returned error: %v", err)
	}
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "event-1", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC), InputTokens: 1000, OutputTokens: 500, CachedTokens: 100, ReasoningTokens: 50, TotalTokens: 1650},
		{EventKey: "event-2", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC), InputTokens: 500, OutputTokens: 250, CachedTokens: 0, ReasoningTokens: 25, TotalTokens: 775},
	}); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}
	if err := repository.AggregateUsageOverviewStats(context.Background(), db, time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("AggregateUsageOverviewStats returned error: %v", err)
	}

	start := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 16, 23, 59, 59, 0, time.UTC)
	provider := NewUsageService(db)
	overview, err := provider.GetUsageOverview(context.Background(), servicedto.UsageFilter{Range: "24h", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("GetUsageOverview returned error: %v", err)
	}
	if overview.Summary.RequestCount != 2 || overview.Summary.TokenCount != 2425 {
		t.Fatalf("expected overview summary counts, got %+v", overview.Summary)
	}
	if overview.Summary.WindowMinutes != 1440 {
		t.Fatalf("expected 24h overview to use exact 1440 minute window, got %+v", overview.Summary)
	}
	if overview.Series.Requests["2026-04-16T17:00:00+08:00"] != 1 || overview.Series.Requests["2026-04-16T18:00:00+08:00"] != 1 {
		t.Fatalf("expected hourly request series values, got %+v", overview.Series)
	}
	if math.Abs(overview.Series.Cost["2026-04-16T17:00:00+08:00"]-0.01023) > 0.000000001 || math.Abs(overview.Series.Cost["2026-04-16T18:00:00+08:00"]-0.00525) > 0.000000001 {
		t.Fatalf("expected hourly cost series values, got %+v", overview.Series)
	}
}

func TestUsageServiceResolvesAPIKeyIDForUsageQueries(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-api-key-filter.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	if err := repository.SyncCPAAPIKeys(db, []string{"sk-target-key", "sk-other-key"}, time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SyncCPAAPIKeys returned error: %v", err)
	}
	activeKeys, err := repository.ListActiveCPAAPIKeys(db)
	if err != nil {
		t.Fatalf("ListActiveCPAAPIKeys returned error: %v", err)
	}
	var targetID string
	for _, key := range activeKeys {
		if key.APIKey == "sk-target-key" {
			targetID = strconv.FormatInt(key.ID, 10)
		}
	}
	if targetID == "" {
		t.Fatalf("expected synced target API key")
	}
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "target-1", APIGroupKey: "sk-target-key", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC), TotalTokens: 10},
		{EventKey: "target-2", APIGroupKey: "sk-target-key", Model: "claude-opus", Timestamp: time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC), TotalTokens: 20},
		{EventKey: "other-1", APIGroupKey: "sk-other-key", Model: "claude-other", Timestamp: time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC), TotalTokens: 300},
	}); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	if err := repository.AggregateUsageOverviewStats(context.Background(), db, time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("AggregateUsageOverviewStats returned error: %v", err)
	}

	start := time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC)
	provider := NewUsageService(db)
	overview, err := provider.GetUsageOverview(context.Background(), servicedto.UsageFilter{APIKeyID: targetID, Range: "custom", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("GetUsageOverview returned error: %v", err)
	}
	if overview.Summary.RequestCount != 2 || overview.Summary.TokenCount != 30 {
		t.Fatalf("expected overview to use resolved API key, got %+v", overview.Summary)
	}
	analysis, err := provider.GetAnalysis(context.Background(), servicedto.UsageFilter{APIKeyID: targetID, Range: "custom", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("GetAnalysis returned error: %v", err)
	}
	if len(analysis.APIKeyComposition) != 1 || analysis.APIKeyComposition[0].Key != "sk-target-key" || analysis.APIKeyComposition[0].TotalTokens != 30 {
		t.Fatalf("expected analysis to use resolved API key, got %+v", analysis.APIKeyComposition)
	}
	events, err := provider.ListUsageEvents(context.Background(), servicedto.UsageFilter{APIKeyID: targetID, Page: 1, PageSize: 100, Limit: 100})
	if err != nil {
		t.Fatalf("ListUsageEvents returned error: %v", err)
	}
	if events.TotalCount != 2 || len(events.Events) != 2 {
		t.Fatalf("expected events to use resolved API key, got %+v", events)
	}
}

func TestUsageServiceRejectsInvalidAPIKeyID(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-invalid-api-key-id.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	provider := NewUsageService(db)

	_, err = provider.ListUsageEvents(context.Background(), servicedto.UsageFilter{APIKeyID: "not-an-id", Page: 1, PageSize: 100, Limit: 100})
	if !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected ErrInvalidID, got %v", err)
	}
}

type requestLogFetcherStub struct {
	content string
	err     error
	calls   int
	lastID  string
}

func (s *requestLogFetcherStub) FetchRequestLogByID(_ context.Context, requestID string) (*response.RequestLogResult, error) {
	s.calls++
	s.lastID = requestID
	if s.err != nil {
		return &response.RequestLogResult{}, s.err
	}
	return &response.RequestLogResult{StatusCode: 200, Body: []byte(s.content), Content: s.content}, nil
}

type sequenceRequestLogFetcherStub struct {
	results []error
	content string
	calls   int
}

func (s *sequenceRequestLogFetcherStub) FetchRequestLogByID(_ context.Context, requestID string) (*response.RequestLogResult, error) {
	s.calls++
	if len(s.results) >= s.calls && s.results[s.calls-1] != nil {
		return &response.RequestLogResult{}, s.results[s.calls-1]
	}
	return &response.RequestLogResult{StatusCode: 200, Body: []byte(s.content), Content: s.content}, nil
}

func TestUsageServiceGetUsageEventRequestDetailFetchesAndCachesDetail(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-detail.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{{EventKey: "event-detail", RequestID: "req-detail", Model: "claude-sonnet", Timestamp: time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC), TotalTokens: 1}}); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}
	var event entities.UsageEvent
	if err := db.Where("event_key = ?", "event-detail").First(&event).Error; err != nil {
		t.Fatalf("load usage event: %v", err)
	}
	fetcher := &requestLogFetcherStub{content: "raw request log"}
	provider := NewUsageServiceWithRequestLogFetcher(db, fetcher)

	first, err := provider.GetUsageEventRequestDetail(context.Background(), strconv.FormatInt(event.ID, 10))
	if err != nil {
		t.Fatalf("GetUsageEventRequestDetail returned error: %v", err)
	}
	second, err := provider.GetUsageEventRequestDetail(context.Background(), strconv.FormatInt(event.ID, 10))
	if err != nil {
		t.Fatalf("second GetUsageEventRequestDetail returned error: %v", err)
	}

	if first.Cached || first.Content != "raw request log" || first.RequestID != "req-detail" {
		t.Fatalf("unexpected first detail: %+v", first)
	}
	if !second.Cached || second.Content != "raw request log" || fetcher.calls != 1 || fetcher.lastID != "req-detail" {
		t.Fatalf("expected second detail to use cache, second=%+v fetcher=%+v", second, fetcher)
	}
}

func TestUsageServiceGetUsageEventRequestDetailMapsMissingAndUpstreamErrors(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-detail-errors.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{{EventKey: "event-without-request", Model: "claude-sonnet", Timestamp: time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC), TotalTokens: 1}, {EventKey: "event-upstream-missing", RequestID: "req-missing", Model: "claude-sonnet", Timestamp: time.Date(2026, 5, 16, 9, 0, 0, 0, time.UTC), TotalTokens: 1}}); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}
	var noRequest entities.UsageEvent
	if err := db.Where("event_key = ?", "event-without-request").First(&noRequest).Error; err != nil {
		t.Fatalf("load no request event: %v", err)
	}
	var upstreamMissing entities.UsageEvent
	if err := db.Where("event_key = ?", "event-upstream-missing").First(&upstreamMissing).Error; err != nil {
		t.Fatalf("load upstream missing event: %v", err)
	}
	provider := &usageService{db: db, requestLogFetcher: &requestLogFetcherStub{err: cpa.ErrRequestLogNotFound}, requestDetailWaitTimeout: time.Millisecond, requestDetailRetryInterval: time.Millisecond}

	_, err = provider.GetUsageEventRequestDetail(context.Background(), strconv.FormatInt(noRequest.ID, 10))
	if !errors.Is(err, ErrUsageEventRequestUnavailable) {
		t.Fatalf("expected ErrUsageEventRequestUnavailable, got %v", err)
	}
	_, err = provider.GetUsageEventRequestDetail(context.Background(), strconv.FormatInt(upstreamMissing.ID, 10))
	if !errors.Is(err, ErrUsageEventRequestUpstreamPending) {
		t.Fatalf("expected ErrUsageEventRequestUpstreamPending, got %v", err)
	}
}

func TestUsageServiceGetUsageEventRequestDetailWaitsForUpstreamLogReadiness(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-detail-wait.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{{EventKey: "event-delayed", RequestID: "req-delayed", Model: "claude-sonnet", Timestamp: time.Date(2026, 5, 16, 9, 0, 0, 0, time.UTC), TotalTokens: 1}}); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}
	var event entities.UsageEvent
	if err := db.Where("event_key = ?", "event-delayed").First(&event).Error; err != nil {
		t.Fatalf("load usage event: %v", err)
	}
	fetcher := &sequenceRequestLogFetcherStub{
		results: []error{cpa.ErrRequestLogNotFound, cpa.ErrRequestLogNotFound, nil},
		content: "delayed upstream request log",
	}
	provider := &usageService{db: db, requestLogFetcher: fetcher, requestDetailWaitTimeout: 50 * time.Millisecond, requestDetailRetryInterval: time.Millisecond}

	detail, err := provider.GetUsageEventRequestDetail(context.Background(), strconv.FormatInt(event.ID, 10))
	if err != nil {
		t.Fatalf("GetUsageEventRequestDetail returned error: %v", err)
	}

	if detail.Content != "delayed upstream request log" || detail.RequestID != "req-delayed" {
		t.Fatalf("expected delayed upstream detail, got %+v", detail)
	}
	if fetcher.calls != 3 {
		t.Fatalf("expected fetcher to retry until log is ready, got %d calls", fetcher.calls)
	}
}

func TestUsageServiceGetUsageEventRequestDetailFallsBackToRedisUsageFailure(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-detail-fallback.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	eventTime := time.Date(2026, 6, 3, 22, 42, 15, 731898054, time.Local)
	rawMessage := `{"timestamp":"2026-06-03T22:42:15.731898054+08:00","latency_ms":6203,"ttft_ms":6151,"source":"primary","auth_index":"auth-a","tokens":{"input_tokens":0,"output_tokens":0,"total_tokens":0},"failed":true,"fail":{"status_code":429,"body":"{\"error\":{\"type\":\"usage_limit_reached\",\"message\":\"The usage limit has been reached\"}}"},"response_headers":{"Retry-After":["60"]},"provider":"openai","model":"gpt-5.5","endpoint":"POST /v1/responses","auth_type":"apikey","request_id":"req-failed"}`
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{{EventKey: "req-failed", RequestID: "req-failed", Model: "gpt-5.5", Endpoint: "POST /v1/responses", Timestamp: eventTime, Failed: true}}); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}
	inboxRows, err := repository.InsertRedisUsageInboxMessages(db, []dto.RedisInboxInsert{{QueueKey: cpa.ManagementUsageQueueKey, RawMessage: rawMessage, PoppedAt: eventTime.Add(5 * time.Second)}})
	if err != nil {
		t.Fatalf("InsertRedisUsageInboxMessages returned error: %v", err)
	}
	if err := repository.MarkRedisUsageInboxProcessed(db, inboxRows[0].ID, "req-failed", eventTime.Add(10*time.Second)); err != nil {
		t.Fatalf("MarkRedisUsageInboxProcessed returned error: %v", err)
	}
	var event entities.UsageEvent
	if err := db.Where("event_key = ?", "req-failed").First(&event).Error; err != nil {
		t.Fatalf("load usage event: %v", err)
	}
	provider := &usageService{db: db, requestLogFetcher: &requestLogFetcherStub{err: cpa.ErrRequestLogNotFound}, requestDetailWaitTimeout: time.Millisecond, requestDetailRetryInterval: time.Millisecond}

	detail, err := provider.GetUsageEventRequestDetail(context.Background(), strconv.FormatInt(event.ID, 10))
	if err != nil {
		t.Fatalf("GetUsageEventRequestDetail returned error: %v", err)
	}

	if detail.Cached || detail.RequestID != "req-failed" {
		t.Fatalf("expected uncached fallback detail for req-failed, got %+v", detail)
	}
	for _, want := range []string{"=== REQUEST INFO ===", "Method: POST", "URL: /v1/responses", "Status: 429", "Retry-After: 60", "usage_limit_reached"} {
		if !strings.Contains(detail.Content, want) {
			t.Fatalf("expected fallback detail to contain %q, got:\n%s", want, detail.Content)
		}
	}
	if _, err := repository.GetUsageRequestDetailByRequestID(db, "req-failed"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("fallback detail should not be cached permanently, got %v", err)
	}
}

func TestUsageServiceGetUsageEventRequestDetailFallbackMatchesAttemptTimestamp(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-detail-fallback-attempt.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	base := time.Date(2026, 6, 3, 22, 42, 0, 0, time.Local)
	events := []entities.UsageEvent{
		{EventKey: "req-retry", RequestID: "req-retry", Model: "gpt-5.5", Endpoint: "POST /v1/responses", Timestamp: base, Failed: true},
		{EventKey: "req-retry", RequestID: "req-retry", Model: "gpt-5.5", Endpoint: "POST /v1/responses", Timestamp: base.Add(20 * time.Second), Failed: false, TotalTokens: 100},
	}
	if _, _, err := repository.InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}
	messages := []dto.RedisInboxInsert{
		{QueueKey: cpa.ManagementUsageQueueKey, RawMessage: `{"timestamp":"2026-06-03T22:42:00+08:00","latency_ms":9000,"failed":true,"fail":{"status_code":429,"body":"{\"error\":\"first-attempt\"}"},"model":"gpt-5.5","endpoint":"POST /v1/responses","request_id":"req-retry"}`, PoppedAt: base.Add(time.Second)},
		{QueueKey: cpa.ManagementUsageQueueKey, RawMessage: `{"timestamp":"2026-06-03T22:42:20+08:00","latency_ms":1000,"failed":false,"fail":{"status_code":200,"body":""},"model":"gpt-5.5","endpoint":"POST /v1/responses","request_id":"req-retry"}`, PoppedAt: base.Add(21 * time.Second)},
	}
	inboxRows, err := repository.InsertRedisUsageInboxMessages(db, messages)
	if err != nil {
		t.Fatalf("InsertRedisUsageInboxMessages returned error: %v", err)
	}
	for _, row := range inboxRows {
		if err := repository.MarkRedisUsageInboxProcessed(db, row.ID, "req-retry", base.Add(30*time.Second)); err != nil {
			t.Fatalf("MarkRedisUsageInboxProcessed returned error: %v", err)
		}
	}
	var firstAttempt entities.UsageEvent
	if err := db.Where("request_id = ? AND failed = ?", "req-retry", true).First(&firstAttempt).Error; err != nil {
		t.Fatalf("load first attempt: %v", err)
	}
	provider := &usageService{db: db, requestLogFetcher: &requestLogFetcherStub{err: cpa.ErrRequestLogNotFound}, requestDetailWaitTimeout: time.Millisecond, requestDetailRetryInterval: time.Millisecond}

	detail, err := provider.GetUsageEventRequestDetail(context.Background(), strconv.FormatInt(firstAttempt.ID, 10))
	if err != nil {
		t.Fatalf("GetUsageEventRequestDetail returned error: %v", err)
	}

	if !strings.Contains(detail.Content, "first-attempt") {
		t.Fatalf("expected fallback to use the failed attempt raw message, got:\n%s", detail.Content)
	}
	if strings.Contains(detail.Content, "Status: 200") {
		t.Fatalf("fallback should not use later successful attempt, got:\n%s", detail.Content)
	}
	if len(detail.Attempts) != 2 {
		t.Fatalf("expected two request detail attempts, got %+v", detail.Attempts)
	}
	if !detail.Attempts[0].Failed || !strings.Contains(detail.Attempts[0].Content, "first-attempt") {
		t.Fatalf("expected first detail attempt to use failed raw message, got %+v", detail.Attempts[0])
	}
	if detail.Attempts[1].Failed || !strings.Contains(detail.Attempts[1].Content, "Status: 200") {
		t.Fatalf("expected second detail attempt to use successful raw message, got %+v", detail.Attempts[1])
	}
	if !detail.Attempts[0].Timestamp.Before(detail.Attempts[1].Timestamp) {
		t.Fatalf("expected attempts in chronological order, got %+v", detail.Attempts)
	}
}

func TestUsageServiceRejectsDeletedAPIKeyID(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-deleted-api-key-id.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	if err := repository.SyncCPAAPIKeys(db, []string{"sk-deleted-key"}, time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SyncCPAAPIKeys returned error: %v", err)
	}
	activeKeys, err := repository.ListActiveCPAAPIKeys(db)
	if err != nil {
		t.Fatalf("ListActiveCPAAPIKeys returned error: %v", err)
	}
	if len(activeKeys) != 1 {
		t.Fatalf("expected one active key, got %+v", activeKeys)
	}
	if err := db.Model(&entities.CPAAPIKey{}).Where("id = ?", activeKeys[0].ID).Update("is_deleted", true).Error; err != nil {
		t.Fatalf("mark api key deleted: %v", err)
	}
	provider := NewUsageService(db)

	_, err = provider.GetUsageOverview(context.Background(), servicedto.UsageFilter{APIKeyID: strconv.FormatInt(activeKeys[0].ID, 10)})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected deleted key to return record not found, got %v", err)
	}
}
