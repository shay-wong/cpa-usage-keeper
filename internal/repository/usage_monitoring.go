package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository/dto"
	"cpa-usage-keeper/internal/timeutil"
	"gorm.io/gorm"
)

var usageMonitoringAggregateTimeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
}

type usageMonitoringChannelStatRow struct {
	Source          string
	AuthIndex       string
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TotalTokens     int64
	InputTokens     int64
	OutputTokens    int64
	CachedTokens    int64
	ReasoningTokens int64
	LastRequestTime string
}

type usageMonitoringChannelModelStatRow struct {
	Source          string
	AuthIndex       string
	Model           string
	Requests        int64
	Success         int64
	Failed          int64
	TotalTokens     int64
	LastRequestTime string
}

type usageMonitoringFailureStatRow struct {
	Source       string
	AuthIndex    string
	FailedCount  int64
	LastFailTime string
}

type usageMonitoringFailureModelStatRow struct {
	Source        string
	AuthIndex     string
	Model         string
	Success       int64
	Failure       int64
	Total         int64
	LastTimestamp string
}

func ListUsageMonitoringRecentRequestsWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter, sourceTargets []UsageMonitoringSourceTargetRecord, sourceModelTargets []UsageMonitoringSourceModelTargetRecord, limit int) ([]UsageMonitoringRecentRequestRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	if limit <= 0 || (len(sourceTargets) == 0 && len(sourceModelTargets) == 0) {
		return []UsageMonitoringRecentRequestRecord{}, nil
	}

	rows := []UsageMonitoringRecentRequestRecord{}
	sourceTargets = uniqueUsageMonitoringSourceTargets(sourceTargets)
	sourceModelTargets = uniqueUsageMonitoringSourceModelTargets(sourceModelTargets)
	if len(sourceTargets) > 0 {
		records, err := listRecentMonitoringSourceRequests(ctx, db, filter, sourceTargets, limit)
		if err != nil {
			return nil, err
		}
		rows = append(rows, records...)
	}
	if len(sourceModelTargets) > 0 {
		records, err := listRecentMonitoringSourceModelRequests(ctx, db, filter, sourceModelTargets, limit)
		if err != nil {
			return nil, err
		}
		rows = append(rows, records...)
	}
	return rows, nil
}

func uniqueUsageMonitoringSourceTargets(targets []UsageMonitoringSourceTargetRecord) []UsageMonitoringSourceTargetRecord {
	seen := map[string]struct{}{}
	unique := make([]UsageMonitoringSourceTargetRecord, 0, len(targets))
	for _, target := range targets {
		source := strings.TrimSpace(target.Source)
		authIndex := strings.TrimSpace(target.AuthIndex)
		key := source + "\x00" + authIndex
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, UsageMonitoringSourceTargetRecord{Source: source, AuthIndex: authIndex})
	}
	return unique
}

func uniqueUsageMonitoringSourceModelTargets(targets []UsageMonitoringSourceModelTargetRecord) []UsageMonitoringSourceModelTargetRecord {
	seen := map[string]struct{}{}
	unique := make([]UsageMonitoringSourceModelTargetRecord, 0, len(targets))
	for _, target := range targets {
		source := strings.TrimSpace(target.Source)
		authIndex := strings.TrimSpace(target.AuthIndex)
		model := strings.TrimSpace(target.Model)
		key := source + "\x00" + authIndex + "\x00" + model
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, UsageMonitoringSourceModelTargetRecord{Source: source, AuthIndex: authIndex, Model: model})
	}
	return unique
}

func listRecentMonitoringSourceRequests(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter, targets []UsageMonitoringSourceTargetRecord, limit int) ([]UsageMonitoringRecentRequestRecord, error) {
	if len(targets) == 0 {
		return []UsageMonitoringRecentRequestRecord{}, nil
	}
	targetSQL, targetArgs := buildUsageMonitoringSourceTargetSQL(targets)
	filterSQL, filterArgs := usageMonitoringRecentRequestFilterSQL(filter)
	query := fmt.Sprintf(`
WITH targets(target_index, source, auth_index) AS (%s),
ranked AS (
	SELECT
		targets.target_index AS target_index,
		targets.source AS target_source,
		targets.auth_index AS target_auth_index,
		COALESCE(TRIM(usage_events.source), '') AS source,
		COALESCE(TRIM(usage_events.model), '') AS model,
		usage_events.timestamp AS timestamp,
		usage_events.failed AS failed,
		ROW_NUMBER() OVER (PARTITION BY targets.target_index ORDER BY usage_events.timestamp DESC, usage_events.id DESC) AS row_number
	FROM usage_events
	JOIN targets ON COALESCE(TRIM(usage_events.source), '') = targets.source
		AND (COALESCE(TRIM(usage_events.auth_index), '') = targets.auth_index OR COALESCE(TRIM(usage_events.auth_index), '') = '')
	%s
)
SELECT target_source AS source, target_auth_index AS auth_index, model, timestamp, failed, false AS model_scoped
FROM ranked
WHERE row_number <= ?
ORDER BY target_index ASC, timestamp DESC`, targetSQL, filterSQL)
	args := append(targetArgs, filterArgs...)
	args = append(args, limit)
	return scanUsageMonitoringRecentRequests(ctx, db, query, args)
}

func listRecentMonitoringSourceModelRequests(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter, targets []UsageMonitoringSourceModelTargetRecord, limit int) ([]UsageMonitoringRecentRequestRecord, error) {
	if len(targets) == 0 {
		return []UsageMonitoringRecentRequestRecord{}, nil
	}
	targetSQL, targetArgs := buildUsageMonitoringSourceModelTargetSQL(targets)
	filterSQL, filterArgs := usageMonitoringRecentRequestFilterSQL(filter)
	query := fmt.Sprintf(`
WITH targets(target_index, source, auth_index, model) AS (%s),
ranked AS (
	SELECT
		targets.target_index AS target_index,
		COALESCE(TRIM(usage_events.source), '') AS source,
		targets.auth_index AS auth_index,
		COALESCE(TRIM(usage_events.model), '') AS model,
		usage_events.timestamp AS timestamp,
		usage_events.failed AS failed,
		ROW_NUMBER() OVER (PARTITION BY targets.target_index ORDER BY usage_events.timestamp DESC, usage_events.id DESC) AS row_number
	FROM usage_events
	JOIN targets ON COALESCE(TRIM(usage_events.source), '') = targets.source
		AND (COALESCE(TRIM(usage_events.auth_index), '') = targets.auth_index OR COALESCE(TRIM(usage_events.auth_index), '') = '')
		AND COALESCE(TRIM(usage_events.model), '') = targets.model
	%s
)
SELECT source, auth_index, model, timestamp, failed, true AS model_scoped
FROM ranked
WHERE row_number <= ?
ORDER BY target_index ASC, timestamp DESC`, targetSQL, filterSQL)
	args := append(targetArgs, filterArgs...)
	args = append(args, limit)
	return scanUsageMonitoringRecentRequests(ctx, db, query, args)
}

func buildUsageMonitoringSourceTargetSQL(targets []UsageMonitoringSourceTargetRecord) (string, []any) {
	parts := make([]string, 0, len(targets))
	args := make([]any, 0, len(targets)*3)
	for index, target := range targets {
		parts = append(parts, "SELECT ? AS target_index, ? AS source, ? AS auth_index")
		args = append(args, index, target.Source, target.AuthIndex)
	}
	return strings.Join(parts, " UNION ALL "), args
}

func buildUsageMonitoringSourceModelTargetSQL(targets []UsageMonitoringSourceModelTargetRecord) (string, []any) {
	parts := make([]string, 0, len(targets))
	args := make([]any, 0, len(targets)*4)
	for index, target := range targets {
		parts = append(parts, "SELECT ? AS target_index, ? AS source, ? AS auth_index, ? AS model")
		args = append(args, index, target.Source, target.AuthIndex, target.Model)
	}
	return strings.Join(parts, " UNION ALL "), args
}

func usageMonitoringRecentRequestFilterSQL(filter dto.UsageQueryFilter) (string, []any) {
	conditions := []string{}
	args := []any{}
	if filter.StartTime != nil {
		conditions = append(conditions, "usage_events.timestamp >= ?")
		args = append(args, timeutil.FormatStorageTime(*filter.StartTime))
	}
	if filter.EndTime != nil {
		conditions = append(conditions, "usage_events.timestamp <= ?")
		args = append(args, timeutil.FormatStorageTime(*filter.EndTime))
	}
	if model := strings.TrimSpace(filter.Model); model != "" {
		conditions = append(conditions, "COALESCE(TRIM(usage_events.model), '') = ?")
		args = append(args, model)
	}
	switch strings.TrimSpace(filter.Result) {
	case "success":
		conditions = append(conditions, "usage_events.failed = ?")
		args = append(args, false)
	case "failed":
		conditions = append(conditions, "usage_events.failed = ?")
		args = append(args, true)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func scanUsageMonitoringRecentRequests(ctx context.Context, db *gorm.DB, query string, args []any) ([]UsageMonitoringRecentRequestRecord, error) {
	rows := []UsageMonitoringRecentRequestRecord{}
	if err := db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load recent usage monitoring requests: %w", err)
	}
	for index := range rows {
		rows[index].Source = strings.TrimSpace(rows[index].Source)
		rows[index].AuthIndex = strings.TrimSpace(rows[index].AuthIndex)
		rows[index].Model = strings.TrimSpace(rows[index].Model)
		rows[index].Timestamp = rows[index].Timestamp.UTC()
	}
	return rows, nil
}

func loadUsageMonitoringEventsWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter) ([]entities.UsageEvent, error) {
	query := applyUsageEventListQuery(db.WithContext(ctx).Model(&entities.UsageEvent{}), filter)
	query = query.Order("timestamp DESC, id DESC")
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	var events []entities.UsageEvent
	if err := query.Find(&events).Error; err != nil {
		return nil, fmt.Errorf("load usage monitoring events: %w", err)
	}
	return events, nil
}

func ListUsageMonitoringEventsWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter) ([]dto.UsageEventRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}

	events, err := loadUsageMonitoringEventsWithFilter(ctx, db, filter)
	if err != nil {
		return nil, err
	}
	return mapUsageMonitoringEventRecords(events), nil
}

func ListRecentUsageMonitoringEventsWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter) ([]dto.UsageEventRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = dto.DefaultUsageEventsLimit
	}
	query := applyUsageEventListQuery(db.WithContext(ctx).Model(&entities.UsageEvent{}), filter)
	query = query.Order("timestamp DESC, id DESC").Limit(limit)

	var events []entities.UsageEvent
	if err := query.Find(&events).Error; err != nil {
		return nil, fmt.Errorf("load recent usage monitoring events: %w", err)
	}
	return mapUsageMonitoringEventRecords(events), nil
}

func ListUsageMonitoringHourlyModelStatsWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter) ([]UsageMonitoringHourlyModelStatRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	query := applyUsageEventListQuery(db.WithContext(ctx).Model(&entities.UsageEvent{}), filter)
	query = query.Select(strings.Join([]string{
		"strftime('%Y-%m-%dT%H:00:00Z', timestamp) AS hour",
		"COALESCE(TRIM(model), '') AS model",
		"COUNT(*) AS requests",
		"SUM(total_tokens) AS tokens",
		"SUM(CASE WHEN failed THEN 0 ELSE 1 END) AS success_count",
		"SUM(CASE WHEN failed THEN 1 ELSE 0 END) AS failure_count",
	}, ", "))
	query = query.Group("strftime('%Y-%m-%dT%H:00:00Z', timestamp), COALESCE(TRIM(model), '')")
	query = query.Order("hour ASC, requests DESC, model ASC")

	var rows []UsageMonitoringHourlyModelStatRecord
	if err := query.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load usage monitoring hourly model stats: %w", err)
	}
	return rows, nil
}

func ListUsageMonitoringChannelStatsWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter) ([]UsageMonitoringChannelStatRecord, []UsageMonitoringChannelModelStatRecord, error) {
	if db == nil {
		return nil, nil, fmt.Errorf("database is nil")
	}
	baseQuery := applyUsageEventListQuery(db.WithContext(ctx).Model(&entities.UsageEvent{}), filter)

	channelQuery := baseQuery.Session(&gorm.Session{})
	channelQuery = channelQuery.Select(strings.Join([]string{
		"COALESCE(TRIM(source), '') AS source",
		"COALESCE(TRIM(auth_index), '') AS auth_index",
		"COUNT(*) AS total_requests",
		"SUM(CASE WHEN failed THEN 0 ELSE 1 END) AS success_requests",
		"SUM(CASE WHEN failed THEN 1 ELSE 0 END) AS failed_requests",
		"SUM(input_tokens) AS input_tokens",
		"SUM(output_tokens) AS output_tokens",
		"SUM(reasoning_tokens) AS reasoning_tokens",
		"SUM(cached_tokens) AS cached_tokens",
		"SUM(total_tokens) AS total_tokens",
		"MAX(timestamp) AS last_request_time",
	}, ", "))
	channelQuery = channelQuery.Group("COALESCE(TRIM(source), ''), COALESCE(TRIM(auth_index), '')")
	channelQuery = channelQuery.Order("total_requests DESC, source ASC, auth_index ASC")

	var channelRows []usageMonitoringChannelStatRow
	if err := channelQuery.Scan(&channelRows).Error; err != nil {
		return nil, nil, fmt.Errorf("load usage monitoring channel stats: %w", err)
	}
	channels, err := mapUsageMonitoringChannelStatRows(channelRows)
	if err != nil {
		return nil, nil, err
	}

	models, err := listUsageMonitoringChannelModelStatsForChannels(ctx, db, filter, channels)
	if err != nil {
		return nil, nil, err
	}
	return channels, models, nil
}

func ListUsageMonitoringFailureStatsWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter) ([]UsageMonitoringFailureStatRecord, []UsageMonitoringFailureModelStatRecord, error) {
	if db == nil {
		return nil, nil, fmt.Errorf("database is nil")
	}
	baseQuery := applyUsageEventListQuery(db.WithContext(ctx).Model(&entities.UsageEvent{}), filter)

	failureQuery := baseQuery.Session(&gorm.Session{}).Where("failed = ?", true)
	failureQuery = failureQuery.Select("COALESCE(TRIM(source), '') AS source, COALESCE(TRIM(auth_index), '') AS auth_index, COUNT(*) AS failed_count, MAX(timestamp) AS last_fail_time")
	failureQuery = failureQuery.Group("COALESCE(TRIM(source), ''), COALESCE(TRIM(auth_index), '')")
	failureQuery = failureQuery.Order("failed_count DESC, source ASC, auth_index ASC")

	var failureRows []usageMonitoringFailureStatRow
	if err := failureQuery.Scan(&failureRows).Error; err != nil {
		return nil, nil, fmt.Errorf("load usage monitoring failure stats: %w", err)
	}
	failures, err := mapUsageMonitoringFailureStatRows(failureRows)
	if err != nil {
		return nil, nil, err
	}

	models, err := listUsageMonitoringFailureModelStatsForFailures(ctx, db, filter, failures)
	if err != nil {
		return nil, nil, err
	}
	return failures, models, nil
}

func listUsageMonitoringChannelModelStatsForChannels(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter, channels []UsageMonitoringChannelStatRecord) ([]UsageMonitoringChannelModelStatRecord, error) {
	if len(channels) == 0 {
		return []UsageMonitoringChannelModelStatRecord{}, nil
	}
	targetSQL, targetArgs := buildUsageMonitoringSourceTargetSQL(channelRowsToSourceTargets(channels))
	filterSQL, filterArgs := usageMonitoringRecentRequestFilterSQL(filter)
	query := fmt.Sprintf(`
WITH targets(target_index, source, auth_index) AS (%s),
aggregated AS (
	SELECT
		targets.target_index AS target_index,
		COALESCE(TRIM(usage_events.source), '') AS source,
		COALESCE(TRIM(usage_events.auth_index), '') AS auth_index,
		COALESCE(TRIM(usage_events.model), '') AS model,
		COUNT(*) AS requests,
		SUM(CASE WHEN usage_events.failed THEN 0 ELSE 1 END) AS success,
		SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END) AS failed,
		SUM(usage_events.total_tokens) AS total_tokens,
		MAX(usage_events.timestamp) AS last_request_time
	FROM usage_events
	JOIN targets ON COALESCE(TRIM(usage_events.source), '') = targets.source AND COALESCE(TRIM(usage_events.auth_index), '') = targets.auth_index
	%s
	GROUP BY targets.target_index, COALESCE(TRIM(usage_events.source), ''), COALESCE(TRIM(usage_events.auth_index), ''), COALESCE(TRIM(usage_events.model), '')
)
SELECT source, auth_index, model, requests, success, failed, total_tokens, last_request_time
FROM aggregated
ORDER BY target_index ASC, requests DESC, model ASC`, targetSQL, filterSQL)
	args := append(targetArgs, filterArgs...)
	rowValues := []usageMonitoringChannelModelStatRow{}
	if err := db.WithContext(ctx).Raw(query, args...).Scan(&rowValues).Error; err != nil {
		return nil, fmt.Errorf("load usage monitoring channel model stats: %w", err)
	}
	return mapUsageMonitoringChannelModelStatRows(rowValues)
}

func listUsageMonitoringFailureModelStatsForFailures(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter, failures []UsageMonitoringFailureStatRecord) ([]UsageMonitoringFailureModelStatRecord, error) {
	if len(failures) == 0 {
		return []UsageMonitoringFailureModelStatRecord{}, nil
	}
	targetSQL, targetArgs := buildUsageMonitoringSourceTargetSQL(failureRowsToSourceTargets(failures))
	filterSQL, filterArgs := usageMonitoringRecentRequestFilterSQL(filter)
	query := fmt.Sprintf(`
WITH targets(target_index, source, auth_index) AS (%s),
aggregated AS (
	SELECT
		targets.target_index AS target_index,
		COALESCE(TRIM(usage_events.source), '') AS source,
		COALESCE(TRIM(usage_events.auth_index), '') AS auth_index,
		COALESCE(TRIM(usage_events.model), '') AS model,
		SUM(CASE WHEN usage_events.failed THEN 0 ELSE 1 END) AS success,
		SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END) AS failure,
		COUNT(*) AS total,
		MAX(usage_events.timestamp) AS last_timestamp
	FROM usage_events
	JOIN targets ON COALESCE(TRIM(usage_events.source), '') = targets.source AND COALESCE(TRIM(usage_events.auth_index), '') = targets.auth_index
	%s
	GROUP BY targets.target_index, COALESCE(TRIM(usage_events.source), ''), COALESCE(TRIM(usage_events.auth_index), ''), COALESCE(TRIM(usage_events.model), '')
	HAVING SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END) > 0
)
SELECT source, auth_index, model, success, failure, total, last_timestamp
FROM aggregated
ORDER BY target_index ASC, failure DESC, model ASC`, targetSQL, filterSQL)
	args := append(targetArgs, filterArgs...)
	rowValues := []usageMonitoringFailureModelStatRow{}
	if err := db.WithContext(ctx).Raw(query, args...).Scan(&rowValues).Error; err != nil {
		return nil, fmt.Errorf("load usage monitoring failure model stats: %w", err)
	}
	return mapUsageMonitoringFailureModelStatRows(rowValues)
}

func mapUsageMonitoringChannelStatRows(rows []usageMonitoringChannelStatRow) ([]UsageMonitoringChannelStatRecord, error) {
	result := make([]UsageMonitoringChannelStatRecord, 0, len(rows))
	for _, row := range rows {
		lastRequestTime, err := parseUsageMonitoringAggregateTime(row.LastRequestTime)
		if err != nil {
			return nil, fmt.Errorf("parse usage monitoring channel last request time: %w", err)
		}
		result = append(result, UsageMonitoringChannelStatRecord{
			Source:          row.Source,
			AuthIndex:       row.AuthIndex,
			TotalRequests:   row.TotalRequests,
			SuccessRequests: row.SuccessRequests,
			FailedRequests:  row.FailedRequests,
			TotalTokens:     row.TotalTokens,
			InputTokens:     row.InputTokens,
			OutputTokens:    row.OutputTokens,
			CachedTokens:    row.CachedTokens,
			ReasoningTokens: row.ReasoningTokens,
			LastRequestTime: lastRequestTime,
		})
	}
	return result, nil
}

func mapUsageMonitoringChannelModelStatRows(rows []usageMonitoringChannelModelStatRow) ([]UsageMonitoringChannelModelStatRecord, error) {
	result := make([]UsageMonitoringChannelModelStatRecord, 0, len(rows))
	for _, row := range rows {
		lastRequestTime, err := parseUsageMonitoringAggregateTime(row.LastRequestTime)
		if err != nil {
			return nil, fmt.Errorf("parse usage monitoring channel model last request time: %w", err)
		}
		result = append(result, UsageMonitoringChannelModelStatRecord{
			Source:          row.Source,
			AuthIndex:       row.AuthIndex,
			Model:           row.Model,
			Requests:        row.Requests,
			Success:         row.Success,
			Failed:          row.Failed,
			TotalTokens:     row.TotalTokens,
			LastRequestTime: lastRequestTime,
		})
	}
	return result, nil
}

func mapUsageMonitoringFailureStatRows(rows []usageMonitoringFailureStatRow) ([]UsageMonitoringFailureStatRecord, error) {
	result := make([]UsageMonitoringFailureStatRecord, 0, len(rows))
	for _, row := range rows {
		lastFailTime, err := parseUsageMonitoringAggregateTime(row.LastFailTime)
		if err != nil {
			return nil, fmt.Errorf("parse usage monitoring failure last fail time: %w", err)
		}
		result = append(result, UsageMonitoringFailureStatRecord{
			Source:       row.Source,
			AuthIndex:    row.AuthIndex,
			FailedCount:  row.FailedCount,
			LastFailTime: lastFailTime,
		})
	}
	return result, nil
}

func mapUsageMonitoringFailureModelStatRows(rows []usageMonitoringFailureModelStatRow) ([]UsageMonitoringFailureModelStatRecord, error) {
	result := make([]UsageMonitoringFailureModelStatRecord, 0, len(rows))
	for _, row := range rows {
		lastTimestamp, err := parseUsageMonitoringAggregateTime(row.LastTimestamp)
		if err != nil {
			return nil, fmt.Errorf("parse usage monitoring failure model last timestamp: %w", err)
		}
		result = append(result, UsageMonitoringFailureModelStatRecord{
			Source:        row.Source,
			AuthIndex:     row.AuthIndex,
			Model:         row.Model,
			Success:       row.Success,
			Failure:       row.Failure,
			Total:         row.Total,
			LastTimestamp: lastTimestamp,
		})
	}
	return result, nil
}

func parseUsageMonitoringAggregateTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	for _, layout := range usageMonitoringAggregateTimeLayouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", trimmed)
}

func channelRowsToSourceTargets(rows []UsageMonitoringChannelStatRecord) []UsageMonitoringSourceTargetRecord {
	targets := make([]UsageMonitoringSourceTargetRecord, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, UsageMonitoringSourceTargetRecord{Source: row.Source, AuthIndex: row.AuthIndex})
	}
	return targets
}

func failureRowsToSourceTargets(rows []UsageMonitoringFailureStatRecord) []UsageMonitoringSourceTargetRecord {
	targets := make([]UsageMonitoringSourceTargetRecord, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, UsageMonitoringSourceTargetRecord{Source: row.Source, AuthIndex: row.AuthIndex})
	}
	return targets
}

func mapUsageMonitoringEventRecords(events []entities.UsageEvent) []dto.UsageEventRecord {
	rows := make([]dto.UsageEventRecord, 0, len(events))
	for _, event := range events {
		rows = append(rows, dto.UsageEventRecord{
			ID:              event.ID,
			Timestamp:       event.Timestamp.UTC(),
			APIGroupKey:     strings.TrimSpace(event.APIGroupKey),
			Model:           strings.TrimSpace(event.Model),
			AuthType:        strings.TrimSpace(event.AuthType),
			Provider:        strings.TrimSpace(event.Provider),
			Source:          strings.TrimSpace(event.Source),
			AuthIndex:       strings.TrimSpace(event.AuthIndex),
			Failed:          event.Failed,
			LatencyMS:       event.LatencyMS,
			InputTokens:     event.InputTokens,
			OutputTokens:    event.OutputTokens,
			ReasoningTokens: event.ReasoningTokens,
			CachedTokens:    event.CachedTokens,
			TotalTokens:     event.TotalTokens,
		})
	}
	return rows
}
