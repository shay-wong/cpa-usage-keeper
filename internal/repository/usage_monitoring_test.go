package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"cpa-usage-keeper/internal/config"
	"cpa-usage-keeper/internal/models"
)

func TestListUsageMonitoringRecentRequestsWithFilterReturnsRowsPerTarget(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-monitoring.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)

	base := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	events := []models.UsageEvent{
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
		UsageQueryFilter{},
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

func TestListUsageMonitoringRecentRequestsWithFilterMatchesChannelRecentRequestsBySource(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-monitoring-channel-source.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)

	base := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	events := []models.UsageEvent{
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
		UsageQueryFilter{},
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
