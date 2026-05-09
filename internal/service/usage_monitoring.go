package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"cpa-usage-keeper/internal/repository"
	repodto "cpa-usage-keeper/internal/repository/dto"
	servicedto "cpa-usage-keeper/internal/service/dto"
)

const (
	monitoringRecentRequestLimit = 12
	monitoringDefaultLogLimit    = 100
	monitoringMaxLogLimit        = 500
	monitoringTopListLimit       = 10
)

type UsageMonitoringProvider interface {
	GetUsageMonitoring(context.Context, servicedto.UsageFilter) (*UsageMonitoringSnapshot, error)
}

func (s *usageService) GetUsageMonitoring(ctx context.Context, filter servicedto.UsageFilter) (*UsageMonitoringSnapshot, error) {
	overview, err := s.GetUsageOverview(ctx, filter)
	if err != nil {
		return nil, err
	}
	analysis, err := s.GetUsageAnalysis(ctx, filter)
	if err != nil {
		return nil, err
	}

	logLimit := filter.Limit
	if logLimit <= 0 {
		logLimit = monitoringDefaultLogLimit
	}
	if logLimit > monitoringMaxLogLimit {
		logLimit = monitoringMaxLogLimit
	}
	repositoryFilter := repodto.UsageQueryFilter{
		StartTime: filter.StartTime,
		EndTime:   filter.EndTime,
	}
	hourlyModelRows, err := repository.ListUsageMonitoringHourlyModelStatsWithFilter(ctx, s.db, repositoryFilter)
	if err != nil {
		return nil, err
	}
	channelRows, channelModelRows, err := repository.ListUsageMonitoringChannelStatsWithFilter(ctx, s.db, repositoryFilter)
	if err != nil {
		return nil, err
	}
	failureRows, failureModelRows, err := repository.ListUsageMonitoringFailureStatsWithFilter(ctx, s.db, repositoryFilter)
	if err != nil {
		return nil, err
	}
	sourceTargets, sourceModelTargets := buildMonitoringRecentRequestTargets(channelRows, channelModelRows, failureRows, failureModelRows)
	recentRequestRows, err := repository.ListUsageMonitoringRecentRequestsWithFilter(ctx, s.db, repositoryFilter, sourceTargets, sourceModelTargets, monitoringRecentRequestLimit)
	if err != nil {
		return nil, err
	}
	recentEvents, err := repository.ListRecentUsageMonitoringEventsWithFilter(ctx, s.db, repodto.UsageQueryFilter{
		StartTime: filter.StartTime,
		EndTime:   filter.EndTime,
		Limit:     logLimit,
	})
	if err != nil {
		return nil, err
	}

	recentRequestsBySource, recentRequestsBySourceModel := buildMonitoringRecentRequestMaps(recentRequestRows)

	return &UsageMonitoringSnapshot{
		KPIs:              buildMonitoringKPI(overview, analysis),
		ModelDistribution: buildMonitoringModelDistribution(analysis),
		DailyTrend:        buildMonitoringTrend(overview.DailySeries, true),
		HourlyModelTrend:  buildMonitoringHourlyModelTrend(hourlyModelRows),
		HourlyTokenTrend:  buildMonitoringTrend(overview.HourlySeries, false),
		ChannelStats:      buildMonitoringChannelStats(channelRows, channelModelRows, recentRequestsBySource, recentRequestsBySourceModel),
		FailureAnalysis:   buildMonitoringFailureAnalysis(failureRows, failureModelRows, recentRequestsBySourceModel),
		RequestLogs:       buildMonitoringRequestLogs(recentEvents, logLimit),
	}, nil
}

func buildMonitoringKPI(overview *servicedto.UsageOverviewSnapshot, analysis *servicedto.UsageAnalysisSnapshot) UsageMonitoringKPI {
	if overview == nil {
		return UsageMonitoringKPI{}
	}
	kpi := UsageMonitoringKPI{
		TotalRequests:   overview.Summary.RequestCount,
		SuccessRequests: overview.Health.TotalSuccess,
		FailedRequests:  overview.Health.TotalFailure,
		TotalTokens:     overview.Summary.TokenCount,
		CachedTokens:    overview.Summary.CachedTokens,
		ReasoningTokens: overview.Summary.ReasoningTokens,
		RPM:             overview.Summary.RPM,
		TPM:             overview.Summary.TPM,
		TotalCost:       overview.Summary.TotalCost,
		CostAvailable:   overview.Summary.CostAvailable,
	}
	if analysis != nil {
		for _, model := range analysis.Models {
			kpi.InputTokens += model.InputTokens
			kpi.OutputTokens += model.OutputTokens
		}
	}
	return kpi
}

func buildMonitoringModelDistribution(analysis *servicedto.UsageAnalysisSnapshot) []UsageMonitoringModelDistributionItem {
	if analysis == nil || len(analysis.Models) == 0 {
		return []UsageMonitoringModelDistributionItem{}
	}
	items := make([]UsageMonitoringModelDistributionItem, 0, len(analysis.Models))
	for _, model := range analysis.Models {
		items = append(items, UsageMonitoringModelDistributionItem{
			Model:           model.Model,
			TotalRequests:   model.TotalRequests,
			SuccessCount:    model.SuccessCount,
			FailureCount:    model.FailureCount,
			TotalTokens:     model.TotalTokens,
			InputTokens:     model.InputTokens,
			OutputTokens:    model.OutputTokens,
			CachedTokens:    model.CachedTokens,
			ReasoningTokens: model.ReasoningTokens,
			SuccessRate:     percentage(model.SuccessCount, model.TotalRequests),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalRequests == items[j].TotalRequests {
			return items[i].Model < items[j].Model
		}
		return items[i].TotalRequests > items[j].TotalRequests
	})
	return items
}

func buildMonitoringTrend(series servicedto.UsageOverviewSeries, byDay bool) []UsageMonitoringTrendPoint {
	keys := sortedInt64MapKeys(series.Requests)
	points := make([]UsageMonitoringTrendPoint, 0, len(keys))
	for _, key := range keys {
		bucket := key
		if !byDay {
			bucket = normalizeHourBucket(key)
		}
		points = append(points, UsageMonitoringTrendPoint{
			Bucket:          bucket,
			Requests:        series.Requests[key],
			Tokens:          series.Tokens[key],
			InputTokens:     series.InputTokens[key],
			OutputTokens:    series.OutputTokens[key],
			CachedTokens:    series.CachedTokens[key],
			ReasoningTokens: series.ReasoningTokens[key],
			Cost:            series.Cost[key],
		})
	}
	return points
}

func buildMonitoringHourlyModelTrend(rows []repository.UsageMonitoringHourlyModelStatRecord) []UsageMonitoringHourlyModelPoint {
	pointsByHour := map[string]*UsageMonitoringHourlyModelPoint{}
	orderedHours := []string{}
	for _, row := range rows {
		hour := normalizeHourBucket(row.Hour)
		point := pointsByHour[hour]
		if point == nil {
			point = &UsageMonitoringHourlyModelPoint{Hour: hour}
			pointsByHour[hour] = point
			orderedHours = append(orderedHours, hour)
		}
		point.Models = append(point.Models, UsageMonitoringHourlyModelStat{
			Model:        normalizeMonitoringDimension(row.Model),
			Requests:     row.Requests,
			Tokens:       row.Tokens,
			SuccessCount: row.SuccessCount,
			FailureCount: row.FailureCount,
		})
	}
	points := make([]UsageMonitoringHourlyModelPoint, 0, len(orderedHours))
	for _, hour := range orderedHours {
		point := pointsByHour[hour]
		sort.Slice(point.Models, func(i, j int) bool {
			if point.Models[i].Requests == point.Models[j].Requests {
				return point.Models[i].Model < point.Models[j].Model
			}
			return point.Models[i].Requests > point.Models[j].Requests
		})
		points = append(points, *point)
	}
	return points
}

func buildMonitoringChannelStats(rows []repository.UsageMonitoringChannelStatRecord, modelRows []repository.UsageMonitoringChannelModelStatRecord, recentRequestsBySource map[string][]UsageMonitoringRecentRequest, recentRequestsBySourceModel map[string][]UsageMonitoringRecentRequest) []UsageMonitoringChannelStat {
	modelsBySource := map[string][]UsageMonitoringChannelModelStat{}
	for _, row := range modelRows {
		key := monitoringSourceKey(row.Source, row.AuthIndex)
		modelKey := monitoringSourceModelKey(row.Source, row.AuthIndex, row.Model)
		lastRequestTime := row.LastRequestTime.UTC()
		modelsBySource[key] = append(modelsBySource[key], UsageMonitoringChannelModelStat{
			Model:           normalizeMonitoringDimension(row.Model),
			Requests:        row.Requests,
			Success:         row.Success,
			Failed:          row.Failed,
			SuccessRate:     percentage(row.Success, row.Requests),
			TotalTokens:     row.TotalTokens,
			LastRequestTime: &lastRequestTime,
			RecentRequests:  trimRecentRequests(recentRequestsBySourceModel[modelKey]),
		})
	}

	result := make([]UsageMonitoringChannelStat, 0, len(rows))
	for _, row := range rows {
		key := monitoringSourceKey(row.Source, row.AuthIndex)
		models := modelsBySource[key]
		sort.Slice(models, func(i, j int) bool {
			if models[i].Requests == models[j].Requests {
				return models[i].Model < models[j].Model
			}
			return models[i].Requests > models[j].Requests
		})
		models = limitMonitoringChannelModelStats(models, monitoringTopListLimit)
		lastRequestTime := row.LastRequestTime.UTC()
		result = append(result, UsageMonitoringChannelStat{
			Source:          strings.TrimSpace(row.Source),
			AuthIndex:       strings.TrimSpace(row.AuthIndex),
			TotalRequests:   row.TotalRequests,
			SuccessRequests: row.SuccessRequests,
			FailedRequests:  row.FailedRequests,
			TotalTokens:     row.TotalTokens,
			InputTokens:     row.InputTokens,
			OutputTokens:    row.OutputTokens,
			CachedTokens:    row.CachedTokens,
			ReasoningTokens: row.ReasoningTokens,
			SuccessRate:     percentage(row.SuccessRequests, row.TotalRequests),
			LastRequestTime: &lastRequestTime,
			RecentRequests:  trimRecentRequests(recentRequestsBySource[key]),
			Models:          models,
		})
	}
	return result
}

func buildMonitoringFailureAnalysis(rows []repository.UsageMonitoringFailureStatRecord, modelRows []repository.UsageMonitoringFailureModelStatRecord, recentRequestsBySourceModel map[string][]UsageMonitoringRecentRequest) []UsageMonitoringFailureStat {
	modelsBySource := map[string][]UsageMonitoringFailureModelStat{}
	for _, row := range modelRows {
		key := monitoringSourceKey(row.Source, row.AuthIndex)
		modelKey := monitoringSourceModelKey(row.Source, row.AuthIndex, row.Model)
		lastTimestamp := row.LastTimestamp.UTC()
		modelsBySource[key] = append(modelsBySource[key], UsageMonitoringFailureModelStat{
			Model:          normalizeMonitoringDimension(row.Model),
			Success:        row.Success,
			Failure:        row.Failure,
			Total:          row.Total,
			SuccessRate:    percentage(row.Success, row.Total),
			LastTimestamp:  &lastTimestamp,
			RecentRequests: trimRecentRequests(recentRequestsBySourceModel[modelKey]),
		})
	}

	failures := make([]UsageMonitoringFailureStat, 0, len(rows))
	for _, row := range rows {
		key := monitoringSourceKey(row.Source, row.AuthIndex)
		models := modelsBySource[key]
		sort.Slice(models, func(i, j int) bool {
			if models[i].Failure == models[j].Failure {
				return models[i].Model < models[j].Model
			}
			return models[i].Failure > models[j].Failure
		})
		models = limitMonitoringFailureModelStats(models, monitoringTopListLimit)
		lastFailTime := row.LastFailTime.UTC()
		failures = append(failures, UsageMonitoringFailureStat{
			Source:       strings.TrimSpace(row.Source),
			AuthIndex:    strings.TrimSpace(row.AuthIndex),
			FailedCount:  row.FailedCount,
			LastFailTime: &lastFailTime,
			Models:       models,
		})
	}
	return failures
}

func buildMonitoringRequestLogs(events []repodto.UsageEventRecord, limit int) []UsageMonitoringRequestLog {
	logs := make([]UsageMonitoringRequestLog, 0, len(events))
	for _, event := range events {
		logs = append(logs, UsageMonitoringRequestLog{
			ID:              event.ID,
			Timestamp:       event.Timestamp.UTC(),
			Model:           normalizeMonitoringDimension(event.Model),
			Source:          event.Source,
			AuthIndex:       event.AuthIndex,
			Failed:          event.Failed,
			LatencyMS:       event.LatencyMS,
			InputTokens:     event.InputTokens,
			OutputTokens:    event.OutputTokens,
			ReasoningTokens: event.ReasoningTokens,
			CachedTokens:    event.CachedTokens,
			TotalTokens:     event.TotalTokens,
		})
		if len(logs) >= limit {
			break
		}
	}
	return logs
}

func buildMonitoringRecentRequestTargets(channelRows []repository.UsageMonitoringChannelStatRecord, channelModelRows []repository.UsageMonitoringChannelModelStatRecord, failureRows []repository.UsageMonitoringFailureStatRecord, failureModelRows []repository.UsageMonitoringFailureModelStatRecord) ([]repository.UsageMonitoringSourceTargetRecord, []repository.UsageMonitoringSourceModelTargetRecord) {
	channelSources := map[string]struct{}{}
	sourceTargets := []repository.UsageMonitoringSourceTargetRecord{}
	for _, row := range channelRows {
		key := monitoringSourceKey(row.Source, row.AuthIndex)
		if _, ok := channelSources[key]; ok {
			continue
		}
		channelSources[key] = struct{}{}
		sourceTargets = append(sourceTargets, repository.UsageMonitoringSourceTargetRecord{Source: row.Source, AuthIndex: row.AuthIndex})
	}

	failureSources := map[string]struct{}{}
	for _, row := range failureRows {
		failureSources[monitoringSourceKey(row.Source, row.AuthIndex)] = struct{}{}
	}

	sourceModelTargets := []repository.UsageMonitoringSourceModelTargetRecord{}
	seenModels := map[string]struct{}{}
	addModelTarget := func(source string, authIndex string, model string) {
		key := monitoringSourceModelKey(source, authIndex, model)
		if _, ok := seenModels[key]; ok {
			return
		}
		seenModels[key] = struct{}{}
		sourceModelTargets = append(sourceModelTargets, repository.UsageMonitoringSourceModelTargetRecord{Source: source, AuthIndex: authIndex, Model: model})
	}
	for _, rows := range topChannelModelRowsBySource(channelModelRows, channelSources) {
		for _, row := range rows {
			addModelTarget(row.Source, row.AuthIndex, row.Model)
		}
	}
	for _, rows := range topFailureModelRowsBySource(failureModelRows, failureSources) {
		for _, row := range rows {
			addModelTarget(row.Source, row.AuthIndex, row.Model)
		}
	}
	return sourceTargets, sourceModelTargets
}

func topChannelModelRowsBySource(rows []repository.UsageMonitoringChannelModelStatRecord, channelSources map[string]struct{}) map[string][]repository.UsageMonitoringChannelModelStatRecord {
	modelsBySource := map[string][]repository.UsageMonitoringChannelModelStatRecord{}
	for _, row := range rows {
		key := monitoringSourceKey(row.Source, row.AuthIndex)
		if _, ok := channelSources[key]; !ok {
			continue
		}
		modelsBySource[key] = append(modelsBySource[key], row)
	}
	for key, models := range modelsBySource {
		sort.Slice(models, func(i, j int) bool {
			if models[i].Requests == models[j].Requests {
				return normalizeMonitoringDimension(models[i].Model) < normalizeMonitoringDimension(models[j].Model)
			}
			return models[i].Requests > models[j].Requests
		})
		if len(models) > monitoringTopListLimit {
			models = models[:monitoringTopListLimit]
		}
		modelsBySource[key] = models
	}
	return modelsBySource
}

func topFailureModelRowsBySource(rows []repository.UsageMonitoringFailureModelStatRecord, failureSources map[string]struct{}) map[string][]repository.UsageMonitoringFailureModelStatRecord {
	modelsBySource := map[string][]repository.UsageMonitoringFailureModelStatRecord{}
	for _, row := range rows {
		key := monitoringSourceKey(row.Source, row.AuthIndex)
		if _, ok := failureSources[key]; !ok {
			continue
		}
		modelsBySource[key] = append(modelsBySource[key], row)
	}
	for key, models := range modelsBySource {
		sort.Slice(models, func(i, j int) bool {
			if models[i].Failure == models[j].Failure {
				return normalizeMonitoringDimension(models[i].Model) < normalizeMonitoringDimension(models[j].Model)
			}
			return models[i].Failure > models[j].Failure
		})
		if len(models) > monitoringTopListLimit {
			models = models[:monitoringTopListLimit]
		}
		modelsBySource[key] = models
	}
	return modelsBySource
}

func buildMonitoringRecentRequestMaps(events []repository.UsageMonitoringRecentRequestRecord) (map[string][]UsageMonitoringRecentRequest, map[string][]UsageMonitoringRecentRequest) {
	bySource := map[string][]UsageMonitoringRecentRequest{}
	bySourceModel := map[string][]UsageMonitoringRecentRequest{}
	for _, event := range events {
		request := UsageMonitoringRecentRequest{Timestamp: event.Timestamp.UTC(), Failed: event.Failed}
		if event.ModelScoped {
			modelKey := monitoringSourceModelKey(event.Source, event.AuthIndex, event.Model)
			bySourceModel[modelKey] = append(bySourceModel[modelKey], request)
			continue
		}
		sourceKey := monitoringSourceKey(event.Source, event.AuthIndex)
		bySource[sourceKey] = append(bySource[sourceKey], request)
	}
	for key, requests := range bySource {
		bySource[key] = trimRecentRequests(requests)
	}
	for key, requests := range bySourceModel {
		bySourceModel[key] = trimRecentRequests(requests)
	}
	return bySource, bySourceModel
}

func sortedInt64MapKeys(values map[string]int64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func trimRecentRequests(requests []UsageMonitoringRecentRequest) []UsageMonitoringRecentRequest {
	sort.Slice(requests, func(i, j int) bool { return requests[i].Timestamp.Before(requests[j].Timestamp) })
	if len(requests) <= monitoringRecentRequestLimit {
		return requests
	}
	return requests[len(requests)-monitoringRecentRequestLimit:]
}

func monitoringSourceKey(source string, authIndex string) string {
	return strings.TrimSpace(source) + "\x00" + strings.TrimSpace(authIndex)
}

func monitoringSourceModelKey(source string, authIndex string, model string) string {
	return monitoringSourceKey(source, authIndex) + "\x00" + normalizeMonitoringDimension(model)
}

func normalizeMonitoringDimension(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

func normalizeHourBucket(value string) string {
	parsed, err := time.Parse("2006-01-02T15:00:00Z", value)
	if err != nil {
		return value
	}
	return parsed.UTC().Format(time.RFC3339)
}

func percentage(numerator int64, denominator int64) float64 {
	if denominator <= 0 {
		return 0
	}
	return (float64(numerator) / float64(denominator)) * 100
}

func limitMonitoringChannelModelStats(items []UsageMonitoringChannelModelStat, limit int) []UsageMonitoringChannelModelStat {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func limitMonitoringFailureModelStats(items []UsageMonitoringFailureModelStat, limit int) []UsageMonitoringFailureModelStat {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func limitChannelStats(items []UsageMonitoringChannelStat, limit int) []UsageMonitoringChannelStat {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func limitFailureStats(items []UsageMonitoringFailureStat, limit int) []UsageMonitoringFailureStat {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}
