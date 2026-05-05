package service

import (
	"fmt"
	"testing"
	"time"

	"cpa-usage-keeper/internal/repository"
)

func TestBuildMonitoringChannelStatsLimitsModelDetails(t *testing.T) {
	lastRequestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	modelRows := make([]repository.UsageMonitoringChannelModelStatRecord, 0, monitoringTopListLimit+2)
	for i := 0; i < monitoringTopListLimit+2; i++ {
		modelRows = append(modelRows, repository.UsageMonitoringChannelModelStatRecord{
			Source:          "source-a",
			AuthIndex:       "1",
			Model:           fmt.Sprintf("model-%02d", i),
			Requests:        int64(100 - i),
			Success:         int64(100 - i),
			LastRequestTime: lastRequestTime,
		})
	}

	stats := buildMonitoringChannelStats(
		[]repository.UsageMonitoringChannelStatRecord{{Source: "source-a", AuthIndex: "1", TotalRequests: 100, SuccessRequests: 100, LastRequestTime: lastRequestTime}},
		modelRows,
		map[string][]UsageMonitoringRecentRequest{},
		map[string][]UsageMonitoringRecentRequest{},
	)

	if len(stats) != 1 {
		t.Fatalf("expected one channel stat, got %+v", stats)
	}
	if len(stats[0].Models) != monitoringTopListLimit {
		t.Fatalf("expected channel model details to be capped at %d, got %d", monitoringTopListLimit, len(stats[0].Models))
	}
	if stats[0].Models[0].Model != "model-00" || stats[0].Models[monitoringTopListLimit-1].Model != "model-09" {
		t.Fatalf("expected top request models to be kept, got %+v", stats[0].Models)
	}
}

func TestBuildMonitoringFailureAnalysisLimitsModelDetails(t *testing.T) {
	lastFailTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	modelRows := make([]repository.UsageMonitoringFailureModelStatRecord, 0, monitoringTopListLimit+2)
	for i := 0; i < monitoringTopListLimit+2; i++ {
		modelRows = append(modelRows, repository.UsageMonitoringFailureModelStatRecord{
			Source:        "source-a",
			AuthIndex:     "1",
			Model:         fmt.Sprintf("model-%02d", i),
			Failure:       int64(100 - i),
			Total:         int64(100 - i),
			LastTimestamp: lastFailTime,
		})
	}

	stats := buildMonitoringFailureAnalysis(
		[]repository.UsageMonitoringFailureStatRecord{{Source: "source-a", AuthIndex: "1", FailedCount: 100, LastFailTime: lastFailTime}},
		modelRows,
		map[string][]UsageMonitoringRecentRequest{},
	)

	if len(stats) != 1 {
		t.Fatalf("expected one failure stat, got %+v", stats)
	}
	if len(stats[0].Models) != monitoringTopListLimit {
		t.Fatalf("expected failure model details to be capped at %d, got %d", monitoringTopListLimit, len(stats[0].Models))
	}
	if stats[0].Models[0].Model != "model-00" || stats[0].Models[monitoringTopListLimit-1].Model != "model-09" {
		t.Fatalf("expected top failure models to be kept, got %+v", stats[0].Models)
	}
}

func TestBuildMonitoringRecentRequestTargetsKeepsOnlyRenderedTopModels(t *testing.T) {
	lastRequestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	channelRows := []repository.UsageMonitoringChannelStatRecord{{Source: "source-a", AuthIndex: "1", LastRequestTime: lastRequestTime}}
	channelModelRows := make([]repository.UsageMonitoringChannelModelStatRecord, 0, monitoringTopListLimit+2)
	for i := 0; i < monitoringTopListLimit+2; i++ {
		channelModelRows = append(channelModelRows, repository.UsageMonitoringChannelModelStatRecord{
			Source: "source-a", AuthIndex: "1", Model: fmt.Sprintf("channel-model-%02d", i), Requests: int64(100 - i), LastRequestTime: lastRequestTime,
		})
	}

	failureRows := []repository.UsageMonitoringFailureStatRecord{{Source: "source-a", AuthIndex: "1", LastFailTime: lastRequestTime}}
	failureModelRows := make([]repository.UsageMonitoringFailureModelStatRecord, 0, monitoringTopListLimit+2)
	for i := 0; i < monitoringTopListLimit+2; i++ {
		failureModelRows = append(failureModelRows, repository.UsageMonitoringFailureModelStatRecord{
			Source: "source-a", AuthIndex: "1", Model: fmt.Sprintf("failure-model-%02d", i), Failure: int64(100 - i), LastTimestamp: lastRequestTime,
		})
	}

	_, modelTargets := buildMonitoringRecentRequestTargets(channelRows, channelModelRows, failureRows, failureModelRows)

	if len(modelTargets) != monitoringTopListLimit*2 {
		t.Fatalf("expected only rendered top model targets, got %d: %+v", len(modelTargets), modelTargets)
	}
	for _, target := range modelTargets {
		if target.Model == "channel-model-10" || target.Model == "channel-model-11" || target.Model == "failure-model-10" || target.Model == "failure-model-11" {
			t.Fatalf("did not expect pruned model target %+v in %+v", target, modelTargets)
		}
	}
}
