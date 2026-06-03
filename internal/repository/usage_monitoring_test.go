package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"cpa-usage-keeper/internal/config"
	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository/dto"
)

func TestListUsageMonitoringRecentRequestsWithFilterReturnsRowsPerTarget(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-monitoring.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)

	base := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	events := []entities.UsageEvent{
		{EventKey: "event-a-model-empty-auth", Source: " source-a ", AuthIndex: "", Model: " claude-sonnet ", Timestamp: base.Add(6 * time.Minute), Failed: false, TotalTokens: 15},
		{EventKey: "event-a-new", Source: "source-a", AuthIndex: "1", Model: "claude-sonnet", Timestamp: base.Add(4 * time.Minute), Failed: false, TotalTokens: 10},
		{EventKey: "event-b-new", Source: "source-b", AuthIndex: "2", Model: "claude-opus", Timestamp: base.Add(2 * time.Minute), Failed: false, TotalTokens: 30},
		{EventKey: "event-c-global-newest", Source: "source-c", AuthIndex: "3", Model: "gpt-5", Timestamp: base.Add(5 * time.Minute), Failed: true, TotalTokens: 40},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	rows, err := ListUsageMonitoringRecentRequestsWithFilter(
		context.Background(),
		db,
		dto.UsageQueryFilter{},
		[]UsageMonitoringSourceTargetRecord{{Source: "source-a", AuthIndex: "1"}, {Source: "source-b", AuthIndex: "2"}},
		[]UsageMonitoringSourceModelTargetRecord{{Source: "source-a", AuthIndex: "1", Model: "claude-sonnet"}},
		1,
	)
	if err != nil {
		t.Fatalf("ListUsageMonitoringRecentRequestsWithFilter returned error: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected one row per source target plus one model row, got %+v", rows)
	}

	counts := map[string]int{}
	for _, row := range rows {
		key := row.Source + "\x00" + row.AuthIndex
		if row.ModelScoped {
			key += "\x00" + row.Model
		}
		counts[key]++
		if row.Source == "source-c" {
			t.Fatalf("did not expect untargeted global newest source in rows: %+v", rows)
		}
	}
	if counts["source-a\x001"] != 1 || counts["source-b\x002"] != 1 || counts["source-a\x001\x00claude-sonnet"] != 1 {
		t.Fatalf("unexpected per-target row counts: %+v from rows %+v", counts, rows)
	}
}

func TestListRecentUsageMonitoringEventsWithFilterKeepsReasoningEffort(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-monitoring-reasoning-effort.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)

	requestTime := time.Date(2026, 5, 21, 14, 27, 54, 0, time.UTC)
	events := []entities.UsageEvent{{
		EventKey:        "event-reasoning-effort",
		Source:          "source-a",
		AuthIndex:       "1",
		Model:           "gpt-5.5",
		ReasoningEffort: "xhigh",
		Timestamp:       requestTime,
		TotalTokens:     47_047,
	}}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	rows, err := ListRecentUsageMonitoringEventsWithFilter(context.Background(), db, dto.UsageQueryFilter{Limit: 1})
	if err != nil {
		t.Fatalf("ListRecentUsageMonitoringEventsWithFilter returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one recent event, got %+v", rows)
	}
	if rows[0].ReasoningEffort != "xhigh" {
		t.Fatalf("expected reasoning effort to be preserved, got %+v", rows[0])
	}
}

func TestListUsageMonitoringEventsWithFilterAppliesDashboardFilters(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-monitoring-dashboard-filters.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)

	base := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	events := []entities.UsageEvent{
		{EventKey: "match", APIGroupKey: "sk-target", Source: "source-a", AuthIndex: "auth-a", Provider: "Provider A", Model: "claude-sonnet", Timestamp: base, Failed: true, TotalTokens: 10},
		{EventKey: "other-key", APIGroupKey: "sk-other", Source: "source-a", AuthIndex: "auth-a", Provider: "Provider A", Model: "claude-sonnet", Timestamp: base.Add(time.Minute), Failed: true, TotalTokens: 20},
		{EventKey: "other-auth", APIGroupKey: "sk-target", Source: "source-a", AuthIndex: "auth-b", Provider: "Provider A", Model: "claude-sonnet", Timestamp: base.Add(2 * time.Minute), Failed: true, TotalTokens: 30},
		{EventKey: "other-result", APIGroupKey: "sk-target", Source: "source-a", AuthIndex: "auth-a", Provider: "Provider A", Model: "claude-sonnet", Timestamp: base.Add(3 * time.Minute), Failed: false, TotalTokens: 40},
		{EventKey: "other-query", APIGroupKey: "sk-target", Source: "source-a", AuthIndex: "auth-a", Provider: "Provider A", Model: "gpt-5", Timestamp: base.Add(4 * time.Minute), Failed: true, TotalTokens: 50},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	filter := dto.UsageQueryFilter{APIGroupKey: "sk-target", AuthIndex: "auth-a", Query: "sonnet", Result: "failed"}
	rows, err := ListUsageMonitoringEventsWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("ListUsageMonitoringEventsWithFilter returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].APIGroupKey != "sk-target" || rows[0].AuthIndex != "auth-a" || rows[0].Model != "claude-sonnet" || !rows[0].Failed {
		t.Fatalf("expected only the matching filtered monitoring row, got %+v", rows)
	}

	hourlyRows, err := ListUsageMonitoringHourlyModelStatsWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("ListUsageMonitoringHourlyModelStatsWithFilter returned error: %v", err)
	}
	if len(hourlyRows) != 1 || hourlyRows[0].Model != "claude-sonnet" || hourlyRows[0].Requests != 1 {
		t.Fatalf("expected hourly model stats to use same filters, got %+v", hourlyRows)
	}
}

func TestListUsageMonitoringStatsWithFilterParsesAggregateTimestamps(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-monitoring-aggregate-time.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)

	base := time.Date(2026, 4, 22, 11, 0, 0, 123000000, time.UTC)
	events := []entities.UsageEvent{
		{EventKey: "event-a-old", Source: "source-a", AuthIndex: "1", Model: "claude-sonnet", Timestamp: base, Failed: false, TotalTokens: 10},
		{EventKey: "event-a-new-failed", Source: "source-a", AuthIndex: "1", Model: "claude-sonnet", Timestamp: base.Add(time.Minute), Failed: true, TotalTokens: 20},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	channels, channelModels, err := ListUsageMonitoringChannelStatsWithFilter(context.Background(), db, dto.UsageQueryFilter{})
	if err != nil {
		t.Fatalf("ListUsageMonitoringChannelStatsWithFilter returned error: %v", err)
	}
	if len(channels) != 1 || len(channelModels) != 1 {
		t.Fatalf("expected one channel and model row, got channels=%+v models=%+v", channels, channelModels)
	}
	if !channels[0].LastRequestTime.Equal(base.Add(time.Minute)) || !channelModels[0].LastRequestTime.Equal(base.Add(time.Minute)) {
		t.Fatalf("expected aggregate last request times to parse, got channel=%s model=%s", channels[0].LastRequestTime, channelModels[0].LastRequestTime)
	}

	failures, failureModels, err := ListUsageMonitoringFailureStatsWithFilter(context.Background(), db, dto.UsageQueryFilter{})
	if err != nil {
		t.Fatalf("ListUsageMonitoringFailureStatsWithFilter returned error: %v", err)
	}
	if len(failures) != 1 || len(failureModels) != 1 {
		t.Fatalf("expected one failure and model row, got failures=%+v models=%+v", failures, failureModels)
	}
	if !failures[0].LastFailTime.Equal(base.Add(time.Minute)) || !failureModels[0].LastTimestamp.Equal(base.Add(time.Minute)) {
		t.Fatalf("expected aggregate failure times to parse, got failure=%s model=%s", failures[0].LastFailTime, failureModels[0].LastTimestamp)
	}
}

func TestListUsageMonitoringStatsWithFilterUsesStorageTimeForModelDetails(t *testing.T) {
	previousLocal := time.Local
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	t.Cleanup(func() { time.Local = previousLocal })
	time.Local = location

	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-monitoring-storage-time.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)

	requestTime := time.Date(2026, 5, 16, 23, 30, 0, 0, location)
	events := []entities.UsageEvent{
		{EventKey: "event-storage-time-failed", Source: "source-a", AuthIndex: "1", Model: "gpt-5.5", Timestamp: requestTime, Failed: true, TotalTokens: 42},
		{EventKey: "event-storage-time-success", Source: "source-a", AuthIndex: "1", Model: "gpt-5.5", Timestamp: requestTime.Add(-time.Minute), Failed: false, TotalTokens: 24},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	start := requestTime.Add(-10 * time.Minute).UTC()
	end := requestTime.Add(10 * time.Minute).UTC()
	filter := dto.UsageQueryFilter{StartTime: &start, EndTime: &end}

	channels, channelModels, err := ListUsageMonitoringChannelStatsWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("ListUsageMonitoringChannelStatsWithFilter returned error: %v", err)
	}
	if len(channels) != 1 || channels[0].TotalRequests != 2 {
		t.Fatalf("expected channel requests inside storage-time window, got %+v", channels)
	}
	if len(channelModels) != 1 || channelModels[0].Model != "gpt-5.5" || channelModels[0].Requests != 2 {
		t.Fatalf("expected channel model details inside storage-time window, got %+v", channelModels)
	}

	failures, failureModels, err := ListUsageMonitoringFailureStatsWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("ListUsageMonitoringFailureStatsWithFilter returned error: %v", err)
	}
	if len(failures) != 1 || failures[0].FailedCount != 1 {
		t.Fatalf("expected failure requests inside storage-time window, got %+v", failures)
	}
	if len(failureModels) != 1 || failureModels[0].Model != "gpt-5.5" || failureModels[0].Failure != 1 {
		t.Fatalf("expected failure model details inside storage-time window, got %+v", failureModels)
	}
}
func TestListUsageMonitoringFailureStatsWithFilterMatchesNullAuthIndexModels(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-monitoring-null-auth.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)

	requestTime := time.Date(2026, 5, 16, 8, 30, 0, 0, time.UTC)
	if err := db.Exec(
		`INSERT INTO usage_events (event_key, source, auth_index, model, timestamp, failed, total_tokens, created_at) VALUES (?, ?, NULL, ?, ?, ?, ?, ?)`,
		"event-null-auth-failed",
		"source-a",
		"claude-sonnet",
		requestTime,
		true,
		42,
		requestTime,
	).Error; err != nil {
		t.Fatalf("insert null-auth usage event: %v", err)
	}

	failures, failureModels, err := ListUsageMonitoringFailureStatsWithFilter(context.Background(), db, dto.UsageQueryFilter{})
	if err != nil {
		t.Fatalf("ListUsageMonitoringFailureStatsWithFilter returned error: %v", err)
	}
	if len(failures) != 1 || failures[0].FailedCount != 1 {
		t.Fatalf("expected one failure count for null auth index, got %+v", failures)
	}
	if len(failureModels) != 1 {
		t.Fatalf("expected null-auth failure count to include its model row, got %+v", failureModels)
	}
	if failureModels[0].Model != "claude-sonnet" || failureModels[0].Failure != 1 {
		t.Fatalf("expected claude-sonnet failure model, got %+v", failureModels[0])
	}
}

func TestListUsageMonitoringRecentRequestsWithFilterMatchesChannelRecentRequestsBySource(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-monitoring-channel-source.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)

	base := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	events := []entities.UsageEvent{
		{EventKey: "event-a-empty-auth", Source: " source-a ", AuthIndex: "", Model: " claude-sonnet ", Timestamp: base.Add(4 * time.Minute), Failed: false, TotalTokens: 10},
		{EventKey: "event-a-other-auth", Source: "source-a", AuthIndex: "other", Model: "claude-opus", Timestamp: base.Add(3 * time.Minute), Failed: true, TotalTokens: 20},
		{EventKey: "event-a-target-auth", Source: "source-a", AuthIndex: "1", Model: "claude-haiku", Timestamp: base.Add(2 * time.Minute), Failed: false, TotalTokens: 25},
		{EventKey: "event-b", Source: "source-b", AuthIndex: "1", Model: "gpt-5", Timestamp: base.Add(5 * time.Minute), Failed: false, TotalTokens: 30},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	rows, err := ListUsageMonitoringRecentRequestsWithFilter(
		context.Background(),
		db,
		dto.UsageQueryFilter{},
		[]UsageMonitoringSourceTargetRecord{{Source: "source-a", AuthIndex: "1"}},
		nil,
		2,
	)
	if err != nil {
		t.Fatalf("ListUsageMonitoringRecentRequestsWithFilter returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected target-auth and empty-auth same-source recent requests, got %+v", rows)
	}
	for _, row := range rows {
		if row.Source != "source-a" || row.AuthIndex != "1" || row.ModelScoped {
			t.Fatalf("expected rows to be keyed to the requested channel target, got %+v", rows)
		}
		if row.Model == "claude-opus" {
			t.Fatalf("did not expect a different auth index request to be mixed into the channel, got %+v", rows)
		}
	}
	if rows[0].Timestamp != base.Add(4*time.Minute) || rows[1].Timestamp != base.Add(2*time.Minute) {
		t.Fatalf("expected empty-auth fallback then target-auth rows ordered newest first, got %+v", rows)
	}
}
