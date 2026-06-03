package service

import (
	"fmt"
	"testing"
	"time"

	"cpa-usage-keeper/internal/repository"
	repositorydto "cpa-usage-keeper/internal/repository/dto"
)

func TestNormalizeMonitoringLogLimitRequiresExplicitLimit(t *testing.T) {
	cases := []struct {
		name  string
		input int
		want  int
	}{
		{name: "default disabled", input: 0, want: 0},
		{name: "negative disabled", input: -1, want: 0},
		{name: "keeps max", input: 1000, want: 1000},
		{name: "clamps oversized", input: 5000, want: 1000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeMonitoringLogLimit(tc.input); got != tc.want {
				t.Fatalf("expected log limit %d, got %d", tc.want, got)
			}
		})
	}
}

func TestBuildMonitoringRequestLogsRequiresPositiveLimit(t *testing.T) {
	events := []repositorydto.UsageEventRecord{{
		ID:        1,
		Timestamp: time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC),
		Model:     "claude-sonnet",
		Source:    "source-a",
	}}

	if got := buildMonitoringRequestLogs(events, 0); len(got) != 0 {
		t.Fatalf("expected no request logs without explicit limit, got %+v", got)
	}
	if got := buildMonitoringRequestLogs(events, 1); len(got) != 1 {
		t.Fatalf("expected request logs with explicit limit, got %+v", got)
	}
}

func TestBuildMonitoringChannelStatsKeepsAllModelDetailsForAPIMerge(t *testing.T) {
	lastRequestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	totalModels := 12
	modelRows := make([]repository.UsageMonitoringChannelModelStatRecord, 0, totalModels)
	for i := 0; i < totalModels; i++ {
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
		map[string]monitoringChannelCost{"source-a\x001": {TotalCost: 0.42, CostAvailable: true}},
	)

	if len(stats) != 1 {
		t.Fatalf("expected one channel stat, got %+v", stats)
	}
	// service 层保留完整明细，避免 API 按 resolved source 合并前少算。
	if len(stats[0].Models) != totalModels {
		t.Fatalf("expected all channel model details, got %d", len(stats[0].Models))
	}
	if stats[0].Models[0].Model != "model-00" || stats[0].Models[totalModels-1].Model != "model-11" {
		t.Fatalf("expected request models to stay sorted, got %+v", stats[0].Models)
	}
	if stats[0].TotalCost != 0.42 || !stats[0].CostAvailable {
		t.Fatalf("expected channel cost to be attached, got %+v", stats[0])
	}
}

func TestBuildMonitoringFailureAnalysisKeepsAllModelDetailsForAPIMerge(t *testing.T) {
	lastFailTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	totalModels := 12
	modelRows := make([]repository.UsageMonitoringFailureModelStatRecord, 0, totalModels)
	for i := 0; i < totalModels; i++ {
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
	// service 层不截断失败模型，展示上限由 API 合并后统一处理。
	if len(stats[0].Models) != totalModels {
		t.Fatalf("expected all failure model details, got %d", len(stats[0].Models))
	}
	if stats[0].Models[0].Model != "model-00" || stats[0].Models[totalModels-1].Model != "model-11" {
		t.Fatalf("expected failure models to stay sorted, got %+v", stats[0].Models)
	}
}

func TestBuildMonitoringRecentRequestTargetsIncludesAllModelTargetsForAPIMerge(t *testing.T) {
	lastRequestTime := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	totalModels := 12
	channelRows := []repository.UsageMonitoringChannelStatRecord{{Source: "source-a", AuthIndex: "1", LastRequestTime: lastRequestTime}}
	channelModelRows := make([]repository.UsageMonitoringChannelModelStatRecord, 0, totalModels)
	for i := 0; i < totalModels; i++ {
		channelModelRows = append(channelModelRows, repository.UsageMonitoringChannelModelStatRecord{
			Source: "source-a", AuthIndex: "1", Model: fmt.Sprintf("channel-model-%02d", i), Requests: int64(100 - i), LastRequestTime: lastRequestTime,
		})
	}

	failureRows := []repository.UsageMonitoringFailureStatRecord{{Source: "source-a", AuthIndex: "1", LastFailTime: lastRequestTime}}
	failureModelRows := make([]repository.UsageMonitoringFailureModelStatRecord, 0, totalModels)
	for i := 0; i < totalModels; i++ {
		failureModelRows = append(failureModelRows, repository.UsageMonitoringFailureModelStatRecord{
			Source: "source-a", AuthIndex: "1", Model: fmt.Sprintf("failure-model-%02d", i), Failure: int64(100 - i), LastTimestamp: lastRequestTime,
		})
	}

	_, modelTargets := buildMonitoringRecentRequestTargets(channelRows, channelModelRows, failureRows, failureModelRows)

	// 请求状态 targets 覆盖完整模型集合，避免 resolved source 合并后的 top 模型缺少 tooltip 数据。
	if len(modelTargets) != totalModels*2 {
		t.Fatalf("expected all model targets, got %d: %+v", len(modelTargets), modelTargets)
	}
	if modelTargets[totalModels-1].Model != "channel-model-11" || modelTargets[len(modelTargets)-1].Model != "failure-model-11" {
		t.Fatalf("expected lower ranked model targets to be kept, got %+v", modelTargets)
	}
}
