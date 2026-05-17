package repository

import (
	"cpa-usage-keeper/internal/repository/dto"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"cpa-usage-keeper/internal/config"
	"cpa-usage-keeper/internal/entities"
)

func TestListUsageEventsWithFilterAppliesTimeBoundsAndPagination(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-events.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)

	events := []entities.UsageEvent{
		{EventKey: "event-1", APIGroupKey: "provider-a", RequestID: "req-1", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC), Source: "source-a", AuthIndex: "1", TotalTokens: 10},
		{EventKey: "event-2", APIGroupKey: "provider-a", RequestID: "req-2", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC), Source: "source-b", AuthIndex: "2", TotalTokens: 20},
		{EventKey: "event-3", APIGroupKey: "provider-b", RequestID: "req-3", Model: "claude-opus", Timestamp: time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC), Source: "source-c", AuthIndex: "3", TotalTokens: 30},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	start := time.Date(2026, 4, 16, 9, 30, 0, 0, time.UTC)
	end := time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC)
	page, err := ListUsageEventsWithFilter(db, dto.UsageQueryFilter{StartTime: &start, EndTime: &end, Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("ListUsageEventsWithFilter returned error: %v", err)
	}
	if page.TotalCount != 2 || page.TotalPages != 2 || page.Page != 1 || page.PageSize != 1 {
		t.Fatalf("unexpected pagination metadata: %+v", page)
	}
	if len(page.Events) != 1 {
		t.Fatalf("expected one row after page size, got %d", len(page.Events))
	}
	if page.Events[0].Source != "source-c" || page.Events[0].RequestID != "req-3" {
		t.Fatalf("expected newest in-range row with request_id first, got %+v", page.Events[0])
	}
}

func TestListUsageEventsWithFilterFindsProjectTimezoneStorageTimestamp(t *testing.T) {
	previousLocal := time.Local
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	time.Local = location
	t.Cleanup(func() { time.Local = previousLocal })

	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-events-project-tz.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)

	eventTime := time.Date(2026, 5, 12, 21, 59, 18, 353569620, location)
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{EventKey: "event-project-tz", Model: "claude-sonnet", Timestamp: eventTime, TotalTokens: 10}}); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	start := time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	page, err := ListUsageEventsWithFilter(db, dto.UsageQueryFilter{StartTime: &start, EndTime: &end, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListUsageEventsWithFilter returned error: %v", err)
	}
	if page.TotalCount != 1 || len(page.Events) != 1 || page.Events[0].Model != "claude-sonnet" {
		t.Fatalf("expected project timezone timestamp to match UTC query window, got %+v", page)
	}
}

func TestListUsageEventsWithFilterPagesByTimestampAndID(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-events-pages.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	timestamp := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	events := []entities.UsageEvent{
		{EventKey: "event-1", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: timestamp, Source: "source-a", AuthIndex: "1", TotalTokens: 10},
		{EventKey: "event-2", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: timestamp, Source: "source-b", AuthIndex: "2", TotalTokens: 20},
		{EventKey: "event-3", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: timestamp.Add(-time.Hour), Source: "source-c", AuthIndex: "3", TotalTokens: 30},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	firstPage, err := ListUsageEventsWithFilter(db, dto.UsageQueryFilter{Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("ListUsageEventsWithFilter returned error: %v", err)
	}
	secondPage, err := ListUsageEventsWithFilter(db, dto.UsageQueryFilter{Page: 2, PageSize: 1})
	if err != nil {
		t.Fatalf("ListUsageEventsWithFilter returned error: %v", err)
	}
	if firstPage.TotalCount != 3 || firstPage.TotalPages != 3 || secondPage.TotalCount != 3 || secondPage.TotalPages != 3 {
		t.Fatalf("unexpected page metadata: first=%+v second=%+v", firstPage, secondPage)
	}
	if len(firstPage.Events) != 1 || len(secondPage.Events) != 1 {
		t.Fatalf("expected one event on each page: first=%+v second=%+v", firstPage, secondPage)
	}
	if firstPage.Events[0].ID <= secondPage.Events[0].ID {
		t.Fatalf("expected id desc tie-breaker, first=%+v second=%+v", firstPage.Events[0], secondPage.Events[0])
	}
}

func TestListUsageEventsWithFilterAppliesModelSourceAndResultFilters(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-events-filtered.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	events := []entities.UsageEvent{
		{EventKey: "event-1", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC), Source: "source-a", AuthIndex: "auth-a", Failed: false, TotalTokens: 10},
		{EventKey: "event-2", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC), Source: "source-a", AuthIndex: "auth-a", Failed: true, TotalTokens: 20},
		{EventKey: "event-3", APIGroupKey: "provider-b", Model: "claude-opus", Timestamp: time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC), Source: "source-a", AuthIndex: "auth-a", Failed: false, TotalTokens: 30},
		{EventKey: "event-4", APIGroupKey: "provider-c", Model: "gpt-5", Timestamp: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC), Source: "source-b", AuthIndex: "auth-b", Failed: false, TotalTokens: 40},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	page, err := ListUsageEventsWithFilter(db, dto.UsageQueryFilter{Page: 1, PageSize: 20, Model: "claude-sonnet", AuthIndex: "auth-a", Result: "success"})
	if err != nil {
		t.Fatalf("ListUsageEventsWithFilter returned error: %v", err)
	}
	if page.TotalCount != 1 || len(page.Events) != 1 {
		t.Fatalf("expected one matching event, got %+v", page)
	}
	if page.Events[0].Model != "claude-sonnet" || page.Events[0].Source != "source-a" || page.Events[0].Failed {
		t.Fatalf("unexpected filtered event: %+v", page.Events[0])
	}
}

func TestListUsageEventsWithFilterAppliesAuthIndexFilter(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-events-auth-filter.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	events := []entities.UsageEvent{
		{EventKey: "event-1", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC), Source: "auth-1", AuthIndex: "auth-1", TotalTokens: 10},
		{EventKey: "event-2", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC), Source: "source-alias", AuthIndex: "auth-1", TotalTokens: 20},
		{EventKey: "event-3", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC), Source: "other", AuthIndex: "other", TotalTokens: 30},
		{EventKey: "event-4", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC), Source: "auth-1", AuthIndex: "auth-1", Provider: "Provider A", TotalTokens: 40},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	page, err := ListUsageEventsWithFilter(db, dto.UsageQueryFilter{Source: "auth-1", AuthIndex: "auth-1", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListUsageEventsWithFilter returned error: %v", err)
	}
	if page.TotalCount != 3 || len(page.Events) != 3 {
		t.Fatalf("expected three matching auth events, got %+v", page)
	}
	for _, event := range page.Events {
		if event.AuthIndex != "auth-1" {
			t.Fatalf("unexpected auth filtered event: %+v", event)
		}
	}
}

func TestListUsageEventFilterOptionsWithFilterReturnsStableModels(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-events-filter-options.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	events := []entities.UsageEvent{
		{EventKey: "event-1", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC), Source: "source-a", Failed: false, TotalTokens: 10},
		{EventKey: "event-2", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC), Source: "source-b", Failed: true, TotalTokens: 20},
		{EventKey: "event-3", APIGroupKey: "provider-b", Model: "gpt-5", Timestamp: time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC), Source: "source-a", Failed: false, TotalTokens: 30},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	options, err := ListUsageEventFilterOptionsWithFilter(db, dto.UsageQueryFilter{Result: "success"})
	if err != nil {
		t.Fatalf("ListUsageEventFilterOptionsWithFilter returned error: %v", err)
	}
	if len(options.Models) != 2 || options.Models[0] != "claude-sonnet" || options.Models[1] != "gpt-5" {
		t.Fatalf("expected stable model options, got %+v", options.Models)
	}
}

func TestUsageRequestDetailCacheSavesReadsAndReusesExistingRequestID(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-request-details.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	now := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)

	first, err := SaveUsageRequestDetail(db, entities.UsageRequestDetail{RequestID: " req-cache ", Content: "first log", Source: "cliproxyapi", FetchedAt: now})
	if err != nil {
		t.Fatalf("SaveUsageRequestDetail returned error: %v", err)
	}
	second, err := SaveUsageRequestDetail(db, entities.UsageRequestDetail{RequestID: "req-cache", Content: "second log", Source: "cliproxyapi", FetchedAt: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("SaveUsageRequestDetail duplicate returned error: %v", err)
	}
	if first.ID != second.ID || second.Content != "first log" || second.RequestID != "req-cache" {
		t.Fatalf("expected duplicate save to return existing cache row, first=%+v second=%+v", first, second)
	}

	cached, err := GetUsageRequestDetailByRequestID(db, "req-cache")
	if err != nil {
		t.Fatalf("GetUsageRequestDetailByRequestID returned error: %v", err)
	}
	if cached.ID != first.ID || cached.Content != "first log" {
		t.Fatalf("unexpected cached detail: %+v", cached)
	}
}

func TestListCachedUsageRequestDetailRequestIDsReturnsCachedSubset(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-request-details-cached-subset.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	now := time.Date(2026, 5, 17, 8, 0, 0, 0, time.UTC)
	for _, requestID := range []string{"req-a", "req-b"} {
		if _, err := SaveUsageRequestDetail(db, entities.UsageRequestDetail{RequestID: requestID, Content: "log", Source: "cliproxyapi", FetchedAt: now}); err != nil {
			t.Fatalf("SaveUsageRequestDetail returned error: %v", err)
		}
	}

	cached, err := ListCachedUsageRequestDetailRequestIDs(db, []string{" req-a ", "req-a", "req-c", "", " req-b "})
	if err != nil {
		t.Fatalf("ListCachedUsageRequestDetailRequestIDs returned error: %v", err)
	}
	if len(cached) != 2 || !cached["req-a"] || !cached["req-b"] || cached["req-c"] {
		t.Fatalf("expected only cached request ids, got %+v", cached)
	}

	empty, err := ListCachedUsageRequestDetailRequestIDs(db, nil)
	if err != nil {
		t.Fatalf("empty ListCachedUsageRequestDetailRequestIDs returned error: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected empty map for empty input, got %+v", empty)
	}
}

func TestEnforceUsageRequestDetailLimitDeletesOldestRows(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-request-details-limit.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	base := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)
	for index := 0; index < 5; index++ {
		_, err := SaveUsageRequestDetail(db, entities.UsageRequestDetail{RequestID: fmt.Sprintf("req-%d", index), Content: "log", Source: "cliproxyapi", FetchedAt: base.Add(time.Duration(index) * time.Minute)})
		if err != nil {
			t.Fatalf("SaveUsageRequestDetail returned error: %v", err)
		}
	}
	if err := EnforceUsageRequestDetailLimit(db, 3); err != nil {
		t.Fatalf("EnforceUsageRequestDetailLimit returned error: %v", err)
	}

	var remaining []entities.UsageRequestDetail
	if err := db.Order("fetched_at ASC, id ASC").Find(&remaining).Error; err != nil {
		t.Fatalf("list remaining request details: %v", err)
	}
	if len(remaining) != 3 || remaining[0].RequestID != "req-2" || remaining[2].RequestID != "req-4" {
		t.Fatalf("expected newest three rows to remain, got %+v", remaining)
	}
}
