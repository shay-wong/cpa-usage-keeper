package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cpa-usage-keeper/internal/service"
	servicedto "cpa-usage-keeper/internal/service/dto"
	"github.com/gin-gonic/gin"
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
	ID         int64                  `json:"id,omitempty"`
	Timestamp  string                 `json:"timestamp"`
	Model      string                 `json:"model"`
	Source     string                 `json:"source"`
	SourceType string                 `json:"source_type,omitempty"`
	SourceKey  string                 `json:"source_key,omitempty"`
	Failed     bool                   `json:"failed"`
	LatencyMS  int64                  `json:"latency_ms"`
	Tokens     usageEventTokenPayload `json:"tokens"`
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
	payload := make([]usageMonitoringChannelStatPayload, 0, len(rows))
	for _, row := range rows {
		models := make([]usageMonitoringChannelModelStatPayload, 0, len(row.Models))
		for _, model := range row.Models {
			models = append(models, usageMonitoringChannelModelStatPayload{
				Model:           model.Model,
				Requests:        model.Requests,
				Success:         model.Success,
				Failed:          model.Failed,
				SuccessRate:     model.SuccessRate,
				TotalTokens:     model.TotalTokens,
				LastRequestTime: model.LastRequestTime,
				RecentRequests:  buildUsageMonitoringRecentRequestsPayload(model.RecentRequests),
			})
		}
		payload = append(payload, usageMonitoringChannelStatPayload{
			usageMonitoringSourcePayload: buildUsageMonitoringSourcePayload(row.Source, row.AuthIndex, resolver),
			TotalRequests:                row.TotalRequests,
			SuccessRequests:              row.SuccessRequests,
			FailedRequests:               row.FailedRequests,
			TotalTokens:                  row.TotalTokens,
			InputTokens:                  row.InputTokens,
			OutputTokens:                 row.OutputTokens,
			CachedTokens:                 row.CachedTokens,
			ReasoningTokens:              row.ReasoningTokens,
			SuccessRate:                  row.SuccessRate,
			LastRequestTime:              row.LastRequestTime,
			RecentRequests:               buildUsageMonitoringRecentRequestsPayload(row.RecentRequests),
			Models:                       models,
		})
	}
	return payload
}

func buildUsageMonitoringFailureAnalysisPayload(rows []service.UsageMonitoringFailureStat, resolver usageSourceResolver) []usageMonitoringFailureStatPayload {
	if len(rows) == 0 {
		return []usageMonitoringFailureStatPayload{}
	}
	payload := make([]usageMonitoringFailureStatPayload, 0, len(rows))
	for _, row := range rows {
		models := make([]usageMonitoringFailureModelStatPayload, 0, len(row.Models))
		for _, model := range row.Models {
			models = append(models, usageMonitoringFailureModelStatPayload{
				Model:          model.Model,
				Success:        model.Success,
				Failure:        model.Failure,
				Total:          model.Total,
				SuccessRate:    model.SuccessRate,
				LastTimestamp:  model.LastTimestamp,
				RecentRequests: buildUsageMonitoringRecentRequestsPayload(model.RecentRequests),
			})
		}
		payload = append(payload, usageMonitoringFailureStatPayload{
			usageMonitoringSourcePayload: buildUsageMonitoringSourcePayload(row.Source, row.AuthIndex, resolver),
			FailedCount:                  row.FailedCount,
			LastFailTime:                 row.LastFailTime,
			Models:                       models,
		})
	}
	return payload
}

func buildUsageMonitoringRequestLogsPayload(rows []service.UsageMonitoringRequestLog, resolver usageSourceResolver) []usageMonitoringRequestLogPayload {
	if len(rows) == 0 {
		return []usageMonitoringRequestLogPayload{}
	}
	payload := make([]usageMonitoringRequestLogPayload, 0, len(rows))
	for _, row := range rows {
		resolved := resolver.resolve(row.Source, row.AuthIndex)
		payload = append(payload, usageMonitoringRequestLogPayload{
			ID:         row.ID,
			Timestamp:  row.Timestamp.UTC().Format(time.RFC3339),
			Model:      row.Model,
			Source:     safeUsageSourceDisplay(resolved, row.AuthIndex),
			SourceType: resolved.SourceType,
			SourceKey:  safeUsageSourceKey(resolved),
			Failed:     row.Failed,
			LatencyMS:  row.LatencyMS,
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
