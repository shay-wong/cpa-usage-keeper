package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"cpa-usage-keeper/internal/service"
	servicedto "cpa-usage-keeper/internal/service/dto"
	"github.com/gin-gonic/gin"
)

const (
	// 监控中心聚合后的最近状态最多展示 12 条，避免合并多来源后 tooltip 被旧数据撑大。
	usageMonitoringRecentRequestPayloadLimit = 12
	// 监控中心聚合后的模型明细最多展示 10 个，保持列表可读。
	usageMonitoringPayloadTopListLimit = 10
)

type usageMonitoringResponse struct {
	KPIs              usageMonitoringKPIPayload                     `json:"kpis"`
	ModelDistribution []usageMonitoringModelDistributionItemPayload `json:"model_distribution"`
	DailyTrend        []usageMonitoringDailyTrendPointPayload       `json:"daily_trend"`
	HourlyModelTrend  []usageMonitoringHourlyModelPointPayload      `json:"hourly_model_trend"`
	HourlyTokenTrend  []usageMonitoringHourlyTokenPointPayload      `json:"hourly_token_trend"`
	ChannelStats      []usageMonitoringChannelStatPayload           `json:"channel_stats"`
	FailureAnalysis   []usageMonitoringFailureStatPayload           `json:"failure_analysis"`
	RequestLogs       []usageMonitoringRequestLogPayload            `json:"request_logs"`
	Timezone          string                                        `json:"timezone"`
	RangeStart        *time.Time                                    `json:"range_start,omitempty"`
	RangeEnd          *time.Time                                    `json:"range_end,omitempty"`
}

type usageMonitoringKPIPayload struct {
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	FailedRequests  int64   `json:"failed_requests"`
	TotalTokens     int64   `json:"total_tokens"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CachedTokens    int64   `json:"cached_tokens"`
	ReasoningTokens int64   `json:"reasoning_tokens"`
	RPM             float64 `json:"rpm"`
	TPM             float64 `json:"tpm"`
	TotalCost       float64 `json:"total_cost"`
	CostAvailable   bool    `json:"cost_available"`
}

type usageMonitoringModelDistributionItemPayload struct {
	Model           string  `json:"model"`
	TotalRequests   int64   `json:"total_requests"`
	SuccessCount    int64   `json:"success_count"`
	FailureCount    int64   `json:"failure_count"`
	TotalTokens     int64   `json:"total_tokens"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CachedTokens    int64   `json:"cached_tokens"`
	ReasoningTokens int64   `json:"reasoning_tokens"`
	SuccessRate     float64 `json:"success_rate"`
}

type usageMonitoringDailyTrendPointPayload struct {
	Date            string  `json:"date"`
	Requests        int64   `json:"requests"`
	Tokens          int64   `json:"tokens"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CachedTokens    int64   `json:"cached_tokens"`
	ReasoningTokens int64   `json:"reasoning_tokens"`
	Cost            float64 `json:"cost"`
}

type usageMonitoringHourlyModelPointPayload struct {
	Hour   string                                  `json:"hour"`
	Models []usageMonitoringHourlyModelStatPayload `json:"models"`
}

type usageMonitoringHourlyModelStatPayload struct {
	Model        string `json:"model"`
	Requests     int64  `json:"requests"`
	Tokens       int64  `json:"tokens"`
	SuccessCount int64  `json:"success_count"`
	FailureCount int64  `json:"failure_count"`
}

type usageMonitoringHourlyTokenPointPayload struct {
	Hour            string  `json:"hour"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CachedTokens    int64   `json:"cached_tokens"`
	ReasoningTokens int64   `json:"reasoning_tokens"`
	TotalTokens     int64   `json:"total_tokens"`
	Cost            float64 `json:"cost"`
}

type usageMonitoringSourcePayload struct {
	Source     string `json:"source"`
	SourceType string `json:"source_type,omitempty"`
	SourceKey  string `json:"source_key,omitempty"`
}

type usageMonitoringRecentRequestPayload struct {
	Timestamp string `json:"timestamp"`
	Failed    bool   `json:"failed"`
}

type usageMonitoringChannelStatPayload struct {
	usageMonitoringSourcePayload
	TotalRequests   int64                                    `json:"total_requests"`
	SuccessRequests int64                                    `json:"success_requests"`
	FailedRequests  int64                                    `json:"failed_requests"`
	TotalTokens     int64                                    `json:"total_tokens"`
	InputTokens     int64                                    `json:"input_tokens"`
	OutputTokens    int64                                    `json:"output_tokens"`
	CachedTokens    int64                                    `json:"cached_tokens"`
	ReasoningTokens int64                                    `json:"reasoning_tokens"`
	SuccessRate     float64                                  `json:"success_rate"`
	LastRequestTime *time.Time                               `json:"last_request_time"`
	RecentRequests  []usageMonitoringRecentRequestPayload    `json:"recent_requests"`
	Models          []usageMonitoringChannelModelStatPayload `json:"models"`
}

type usageMonitoringChannelModelStatPayload struct {
	Model           string                                `json:"model"`
	Requests        int64                                 `json:"requests"`
	Success         int64                                 `json:"success"`
	Failed          int64                                 `json:"failed"`
	SuccessRate     float64                               `json:"success_rate"`
	TotalTokens     int64                                 `json:"total_tokens"`
	LastRequestTime *time.Time                            `json:"last_request_time"`
	RecentRequests  []usageMonitoringRecentRequestPayload `json:"recent_requests"`
}

type usageMonitoringFailureStatPayload struct {
	usageMonitoringSourcePayload
	FailedCount  int64                                    `json:"failed_count"`
	LastFailTime *time.Time                               `json:"last_fail_time"`
	Models       []usageMonitoringFailureModelStatPayload `json:"models"`
}

type usageMonitoringFailureModelStatPayload struct {
	Model          string                                `json:"model"`
	Success        int64                                 `json:"success"`
	Failure        int64                                 `json:"failure"`
	Total          int64                                 `json:"total"`
	SuccessRate    float64                               `json:"success_rate"`
	LastTimestamp  *time.Time                            `json:"last_timestamp"`
	RecentRequests []usageMonitoringRecentRequestPayload `json:"recent_requests"`
}

type usageMonitoringRequestLogPayload struct {
	ID              int64                  `json:"id,omitempty"`
	Timestamp       string                 `json:"timestamp"`
	Model           string                 `json:"model"`
	ReasoningEffort string                 `json:"reasoning_effort,omitempty"`
	Source          string                 `json:"source"`
	SourceType      string                 `json:"source_type,omitempty"`
	SourceKey       string                 `json:"source_key,omitempty"`
	Failed          bool                   `json:"failed"`
	LatencyMS       int64                  `json:"latency_ms"`
	Tokens          usageEventTokenPayload `json:"tokens"`
}

func registerUsageMonitoringRoute(
	router gin.IRoutes,
	usageProvider service.UsageProvider,
	usageIdentityProvider service.UsageIdentityProvider,
) {
	router.GET("/usage/monitoring", func(c *gin.Context) {
		filter, err := parseUsageMonitoringFilterQuery(c.Request, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		monitoringProvider, ok := usageProvider.(service.UsageMonitoringProvider)
		if usageProvider == nil || !ok {
			c.JSON(http.StatusOK, buildUsageMonitoringPayload(nil, usageSourceResolver{}, filter))
			return
		}

		resolver := usageSourceResolver{}
		identities, err := loadUsageResolutionData(c, usageIdentityProvider)
		if err != nil {
			slog.Warn("load usage resolution data failed", "error", err)
		} else {
			resolver = newUsageSourceResolver(identities)
		}

		snapshot, err := monitoringProvider.GetUsageMonitoring(c.Request.Context(), filter)
		if err != nil {
			writeInternalError(c, "get usage monitoring failed", err)
			return
		}

		c.JSON(http.StatusOK, buildUsageMonitoringPayload(snapshot, resolver, filter))
	})
}

func parseUsageMonitoringFilterQuery(req *http.Request, anchor time.Time) (servicedto.UsageFilter, error) {
	filter, err := parseUsageTimeFilterQuery(req, anchor)
	if err != nil {
		return servicedto.UsageFilter{}, err
	}
	logLimitValue := strings.TrimSpace(req.URL.Query().Get("log_limit"))
	if logLimitValue == "" {
		return filter, nil
	}
	logLimit, err := strconv.Atoi(logLimitValue)
	if err != nil || logLimit <= 0 {
		return servicedto.UsageFilter{}, fmt.Errorf("invalid log_limit %q", logLimitValue)
	}
	filter.Limit = logLimit
	return filter, nil
}

func buildUsageMonitoringPayload(snapshot *service.UsageMonitoringSnapshot, resolver usageSourceResolver, filter servicedto.UsageFilter) usageMonitoringResponse {
	if snapshot == nil {
		return usageMonitoringResponse{
			ModelDistribution: []usageMonitoringModelDistributionItemPayload{},
			DailyTrend:        []usageMonitoringDailyTrendPointPayload{},
			HourlyModelTrend:  []usageMonitoringHourlyModelPointPayload{},
			HourlyTokenTrend:  []usageMonitoringHourlyTokenPointPayload{},
			ChannelStats:      []usageMonitoringChannelStatPayload{},
			FailureAnalysis:   []usageMonitoringFailureStatPayload{},
			RequestLogs:       []usageMonitoringRequestLogPayload{},
			Timezone:          time.Local.String(),
			RangeStart:        filter.StartTime,
			RangeEnd:          filter.EndTime,
		}
	}

	return usageMonitoringResponse{
		KPIs:              buildUsageMonitoringKPI(snapshot.KPIs),
		ModelDistribution: buildUsageMonitoringModelDistributionPayload(snapshot.ModelDistribution),
		DailyTrend:        buildUsageMonitoringDailyTrendPayload(snapshot.DailyTrend),
		HourlyModelTrend:  buildUsageMonitoringHourlyModelTrendPayload(snapshot.HourlyModelTrend),
		HourlyTokenTrend:  buildUsageMonitoringHourlyTokenTrendPayload(snapshot.HourlyTokenTrend),
		ChannelStats:      buildUsageMonitoringChannelStatsPayload(snapshot.ChannelStats, resolver),
		FailureAnalysis:   buildUsageMonitoringFailureAnalysisPayload(snapshot.FailureAnalysis, resolver),
		RequestLogs:       buildUsageMonitoringRequestLogsPayload(snapshot.RequestLogs, resolver),
		Timezone:          time.Local.String(),
		RangeStart:        filter.StartTime,
		RangeEnd:          filter.EndTime,
	}
}

func buildUsageMonitoringKPI(kpi service.UsageMonitoringKPI) usageMonitoringKPIPayload {
	return usageMonitoringKPIPayload{
		TotalRequests:   kpi.TotalRequests,
		SuccessRequests: kpi.SuccessRequests,
		FailedRequests:  kpi.FailedRequests,
		TotalTokens:     kpi.TotalTokens,
		InputTokens:     kpi.InputTokens,
		OutputTokens:    kpi.OutputTokens,
		CachedTokens:    kpi.CachedTokens,
		ReasoningTokens: kpi.ReasoningTokens,
		RPM:             kpi.RPM,
		TPM:             kpi.TPM,
		TotalCost:       kpi.TotalCost,
		CostAvailable:   kpi.CostAvailable,
	}
}

func buildUsageMonitoringModelDistributionPayload(items []service.UsageMonitoringModelDistributionItem) []usageMonitoringModelDistributionItemPayload {
	if len(items) == 0 {
		return []usageMonitoringModelDistributionItemPayload{}
	}
	payload := make([]usageMonitoringModelDistributionItemPayload, 0, len(items))
	for _, item := range items {
		payload = append(payload, usageMonitoringModelDistributionItemPayload{
			Model:           item.Model,
			TotalRequests:   item.TotalRequests,
			SuccessCount:    item.SuccessCount,
			FailureCount:    item.FailureCount,
			TotalTokens:     item.TotalTokens,
			InputTokens:     item.InputTokens,
			OutputTokens:    item.OutputTokens,
			CachedTokens:    item.CachedTokens,
			ReasoningTokens: item.ReasoningTokens,
			SuccessRate:     item.SuccessRate,
		})
	}
	return payload
}

func buildUsageMonitoringDailyTrendPayload(points []service.UsageMonitoringTrendPoint) []usageMonitoringDailyTrendPointPayload {
	if len(points) == 0 {
		return []usageMonitoringDailyTrendPointPayload{}
	}
	payload := make([]usageMonitoringDailyTrendPointPayload, 0, len(points))
	for _, point := range points {
		payload = append(payload, usageMonitoringDailyTrendPointPayload{
			Date:            point.Bucket,
			Requests:        point.Requests,
			Tokens:          point.Tokens,
			InputTokens:     point.InputTokens,
			OutputTokens:    point.OutputTokens,
			CachedTokens:    point.CachedTokens,
			ReasoningTokens: point.ReasoningTokens,
			Cost:            point.Cost,
		})
	}
	return payload
}

func buildUsageMonitoringHourlyModelTrendPayload(points []service.UsageMonitoringHourlyModelPoint) []usageMonitoringHourlyModelPointPayload {
	if len(points) == 0 {
		return []usageMonitoringHourlyModelPointPayload{}
	}
	payload := make([]usageMonitoringHourlyModelPointPayload, 0, len(points))
	for _, point := range points {
		models := make([]usageMonitoringHourlyModelStatPayload, 0, len(point.Models))
		for _, model := range point.Models {
			models = append(models, usageMonitoringHourlyModelStatPayload{
				Model:        model.Model,
				Requests:     model.Requests,
				Tokens:       model.Tokens,
				SuccessCount: model.SuccessCount,
				FailureCount: model.FailureCount,
			})
		}
		payload = append(payload, usageMonitoringHourlyModelPointPayload{Hour: point.Hour, Models: models})
	}
	return payload
}

func buildUsageMonitoringHourlyTokenTrendPayload(points []service.UsageMonitoringTrendPoint) []usageMonitoringHourlyTokenPointPayload {
	if len(points) == 0 {
		return []usageMonitoringHourlyTokenPointPayload{}
	}
	payload := make([]usageMonitoringHourlyTokenPointPayload, 0, len(points))
	for _, point := range points {
		payload = append(payload, usageMonitoringHourlyTokenPointPayload{
			Hour:            point.Bucket,
			InputTokens:     point.InputTokens,
			OutputTokens:    point.OutputTokens,
			CachedTokens:    point.CachedTokens,
			ReasoningTokens: point.ReasoningTokens,
			TotalTokens:     point.Tokens,
			Cost:            point.Cost,
		})
	}
	return payload
}

func buildUsageMonitoringChannelStatsPayload(rows []service.UsageMonitoringChannelStat, resolver usageSourceResolver) []usageMonitoringChannelStatPayload {
	if len(rows) == 0 {
		return []usageMonitoringChannelStatPayload{}
	}
	merged := make([]usageMonitoringChannelStatPayload, 0, len(rows))
	indexBySourceKey := map[string]int{}
	for _, row := range rows {
		source := buildUsageMonitoringSourcePayload(row.Source, row.AuthIndex, resolver)
		key := monitoringPayloadSourceKey(source)
		index, exists := indexBySourceKey[key]
		if !exists {
			indexBySourceKey[key] = len(merged)
			merged = append(merged, usageMonitoringChannelStatPayload{usageMonitoringSourcePayload: source})
			index = len(merged) - 1
		}
		current := &merged[index]
		current.TotalRequests += row.TotalRequests
		current.SuccessRequests += row.SuccessRequests
		current.FailedRequests += row.FailedRequests
		current.TotalTokens += row.TotalTokens
		current.InputTokens += row.InputTokens
		current.OutputTokens += row.OutputTokens
		current.CachedTokens += row.CachedTokens
		current.ReasoningTokens += row.ReasoningTokens
		current.SuccessRate = service.MonitoringPercentage(current.SuccessRequests, current.TotalRequests)
		current.LastRequestTime = latestTimePtr(current.LastRequestTime, row.LastRequestTime)
		current.RecentRequests = mergeUsageMonitoringRecentRequestPayloads(current.RecentRequests, buildUsageMonitoringRecentRequestsPayload(row.RecentRequests))
		current.Models = mergeUsageMonitoringChannelModelPayloads(current.Models, row.Models)
	}
	for index := range merged {
		merged[index].Models = limitUsageMonitoringChannelModelPayloads(merged[index].Models)
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].TotalRequests == merged[j].TotalRequests {
			if merged[i].Source == merged[j].Source {
				return merged[i].SourceKey < merged[j].SourceKey
			}
			return merged[i].Source < merged[j].Source
		}
		return merged[i].TotalRequests > merged[j].TotalRequests
	})
	return limitUsageMonitoringChannelStatsPayload(merged)
}

func buildUsageMonitoringFailureAnalysisPayload(rows []service.UsageMonitoringFailureStat, resolver usageSourceResolver) []usageMonitoringFailureStatPayload {
	if len(rows) == 0 {
		return []usageMonitoringFailureStatPayload{}
	}
	merged := make([]usageMonitoringFailureStatPayload, 0, len(rows))
	indexBySourceKey := map[string]int{}
	for _, row := range rows {
		source := buildUsageMonitoringSourcePayload(row.Source, row.AuthIndex, resolver)
		key := monitoringPayloadSourceKey(source)
		index, exists := indexBySourceKey[key]
		if !exists {
			indexBySourceKey[key] = len(merged)
			merged = append(merged, usageMonitoringFailureStatPayload{usageMonitoringSourcePayload: source})
			index = len(merged) - 1
		}
		current := &merged[index]
		current.FailedCount += row.FailedCount
		current.LastFailTime = latestTimePtr(current.LastFailTime, row.LastFailTime)
		current.Models = mergeUsageMonitoringFailureModelPayloads(current.Models, row.Models)
	}
	for index := range merged {
		merged[index].Models = limitUsageMonitoringFailureModelPayloads(merged[index].Models)
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].FailedCount == merged[j].FailedCount {
			if merged[i].Source == merged[j].Source {
				return merged[i].SourceKey < merged[j].SourceKey
			}
			return merged[i].Source < merged[j].Source
		}
		return merged[i].FailedCount > merged[j].FailedCount
	})
	return limitUsageMonitoringFailureStatsPayload(merged)
}

func limitUsageMonitoringChannelStatsPayload(items []usageMonitoringChannelStatPayload) []usageMonitoringChannelStatPayload {
	if len(items) <= usageMonitoringPayloadTopListLimit {
		return items
	}
	return items[:usageMonitoringPayloadTopListLimit]
}

func limitUsageMonitoringFailureStatsPayload(items []usageMonitoringFailureStatPayload) []usageMonitoringFailureStatPayload {
	if len(items) <= usageMonitoringPayloadTopListLimit {
		return items
	}
	return items[:usageMonitoringPayloadTopListLimit]
}

func monitoringPayloadSourceKey(source usageMonitoringSourcePayload) string {
	if strings.TrimSpace(source.SourceKey) != "" {
		return strings.TrimSpace(source.SourceKey)
	}
	return strings.TrimSpace(source.Source) + "\x00" + strings.TrimSpace(source.SourceType)
}

func latestTimePtr(current *time.Time, next *time.Time) *time.Time {
	if next == nil {
		return current
	}
	if current == nil || next.After(*current) {
		value := next.UTC()
		return &value
	}
	return current
}

func mergeUsageMonitoringRecentRequestPayloads(current []usageMonitoringRecentRequestPayload, next []usageMonitoringRecentRequestPayload) []usageMonitoringRecentRequestPayload {
	merged := append(append([]usageMonitoringRecentRequestPayload{}, current...), next...)
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Timestamp < merged[j].Timestamp
	})
	if len(merged) <= usageMonitoringRecentRequestPayloadLimit {
		return merged
	}
	return merged[len(merged)-usageMonitoringRecentRequestPayloadLimit:]
}

func mergeUsageMonitoringChannelModelPayloads(current []usageMonitoringChannelModelStatPayload, rows []service.UsageMonitoringChannelModelStat) []usageMonitoringChannelModelStatPayload {
	indexByModel := map[string]int{}
	for index, model := range current {
		indexByModel[model.Model] = index
	}
	for _, row := range rows {
		index, exists := indexByModel[row.Model]
		if !exists {
			indexByModel[row.Model] = len(current)
			current = append(current, usageMonitoringChannelModelStatPayload{Model: row.Model})
			index = len(current) - 1
		}
		model := &current[index]
		model.Requests += row.Requests
		model.Success += row.Success
		model.Failed += row.Failed
		model.TotalTokens += row.TotalTokens
		model.SuccessRate = service.MonitoringPercentage(model.Success, model.Requests)
		model.LastRequestTime = latestTimePtr(model.LastRequestTime, row.LastRequestTime)
		model.RecentRequests = mergeUsageMonitoringRecentRequestPayloads(model.RecentRequests, buildUsageMonitoringRecentRequestsPayload(row.RecentRequests))
	}
	sort.Slice(current, func(i, j int) bool {
		if current[i].Requests == current[j].Requests {
			return current[i].Model < current[j].Model
		}
		return current[i].Requests > current[j].Requests
	})
	return current
}

func limitUsageMonitoringChannelModelPayloads(current []usageMonitoringChannelModelStatPayload) []usageMonitoringChannelModelStatPayload {
	if len(current) > usageMonitoringPayloadTopListLimit {
		return current[:usageMonitoringPayloadTopListLimit]
	}
	return current
}

func mergeUsageMonitoringFailureModelPayloads(current []usageMonitoringFailureModelStatPayload, rows []service.UsageMonitoringFailureModelStat) []usageMonitoringFailureModelStatPayload {
	indexByModel := map[string]int{}
	for index, model := range current {
		indexByModel[model.Model] = index
	}
	for _, row := range rows {
		index, exists := indexByModel[row.Model]
		if !exists {
			indexByModel[row.Model] = len(current)
			current = append(current, usageMonitoringFailureModelStatPayload{Model: row.Model})
			index = len(current) - 1
		}
		model := &current[index]
		model.Success += row.Success
		model.Failure += row.Failure
		model.Total += row.Total
		model.SuccessRate = service.MonitoringPercentage(model.Success, model.Total)
		model.LastTimestamp = latestTimePtr(model.LastTimestamp, row.LastTimestamp)
		model.RecentRequests = mergeUsageMonitoringRecentRequestPayloads(model.RecentRequests, buildUsageMonitoringRecentRequestsPayload(row.RecentRequests))
	}
	sort.Slice(current, func(i, j int) bool {
		if current[i].Failure == current[j].Failure {
			return current[i].Model < current[j].Model
		}
		return current[i].Failure > current[j].Failure
	})
	return current
}

func limitUsageMonitoringFailureModelPayloads(current []usageMonitoringFailureModelStatPayload) []usageMonitoringFailureModelStatPayload {
	if len(current) > usageMonitoringPayloadTopListLimit {
		return current[:usageMonitoringPayloadTopListLimit]
	}
	return current
}

func buildUsageMonitoringRequestLogsPayload(rows []service.UsageMonitoringRequestLog, resolver usageSourceResolver) []usageMonitoringRequestLogPayload {
	if len(rows) == 0 {
		return []usageMonitoringRequestLogPayload{}
	}
	payload := make([]usageMonitoringRequestLogPayload, 0, len(rows))
	for _, row := range rows {
		resolved := resolver.resolve(row.Source, row.AuthIndex)
		payload = append(payload, usageMonitoringRequestLogPayload{
			ID:              row.ID,
			Timestamp:       row.Timestamp.UTC().Format(time.RFC3339),
			Model:           row.Model,
			ReasoningEffort: row.ReasoningEffort,
			Source:          safeUsageSourceDisplay(resolved, row.AuthIndex),
			SourceType:      resolved.SourceType,
			SourceKey:       safeUsageSourceKey(resolved),
			Failed:          row.Failed,
			LatencyMS:       row.LatencyMS,
			Tokens: usageEventTokenPayload{
				InputTokens:     row.InputTokens,
				OutputTokens:    row.OutputTokens,
				ReasoningTokens: row.ReasoningTokens,
				CachedTokens:    row.CachedTokens,
				TotalTokens:     row.TotalTokens,
			},
		})
	}
	return payload
}

func buildUsageMonitoringSourcePayload(rawSource string, authIndex string, resolver usageSourceResolver) usageMonitoringSourcePayload {
	resolved := resolver.resolve(rawSource, authIndex)
	return usageMonitoringSourcePayload{
		Source:     safeUsageSourceDisplay(resolved, authIndex),
		SourceType: resolved.SourceType,
		SourceKey:  safeUsageSourceKey(resolved),
	}
}

func buildUsageMonitoringRecentRequestsPayload(rows []service.UsageMonitoringRecentRequest) []usageMonitoringRecentRequestPayload {
	if len(rows) == 0 {
		return []usageMonitoringRecentRequestPayload{}
	}
	payload := make([]usageMonitoringRecentRequestPayload, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, usageMonitoringRecentRequestPayload{
			Timestamp: row.Timestamp.UTC().Format(time.RFC3339),
			Failed:    row.Failed,
		})
	}
	return payload
}
