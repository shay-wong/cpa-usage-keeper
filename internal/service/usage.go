package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"cpa-usage-keeper/internal/cpa"
	"cpa-usage-keeper/internal/cpa/dto/response"
	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository"
	repodto "cpa-usage-keeper/internal/repository/dto"
	servicedto "cpa-usage-keeper/internal/service/dto"
	"cpa-usage-keeper/internal/timeutil"
	"gorm.io/gorm"
)

type usageService struct {
	db                         *gorm.DB
	requestLogFetcher          RequestLogFetcher
	requestDetailWaitTimeout   time.Duration
	requestDetailRetryInterval time.Duration
}

type RequestLogFetcher interface {
	FetchRequestLogByID(context.Context, string) (*response.RequestLogResult, error)
}

const usageRequestDetailCacheLimit = 10000
const usageRequestDetailFallbackCandidateLimit = 50
const usageRequestDetailDefaultWaitTimeout = 30 * time.Second
const usageRequestDetailDefaultRetryInterval = time.Second

var (
	ErrUsageEventNotFound                = errors.New("usage event not found")
	ErrUsageEventRequestUnavailable      = errors.New("usage event request detail unavailable")
	ErrUsageEventRequestUpstreamNotFound = errors.New("usage event request detail upstream not found")
	ErrUsageEventRequestUpstreamPending  = errors.New("usage event request detail upstream pending")
	ErrUsageEventRequestTooLarge         = errors.New("usage event request detail too large")
	ErrUsageEventRequestUpstream         = errors.New("usage event request detail upstream failed")
)

func NewUsageService(db *gorm.DB) UsageProvider {
	return &usageService{db: db}
}

func NewUsageServiceWithRequestLogFetcher(db *gorm.DB, fetcher RequestLogFetcher) UsageProvider {
	return &usageService{db: db, requestLogFetcher: fetcher}
}

func (s *usageService) detailWaitTimeout() time.Duration {
	if s == nil || s.requestDetailWaitTimeout <= 0 {
		return usageRequestDetailDefaultWaitTimeout
	}
	return s.requestDetailWaitTimeout
}

func (s *usageService) detailRetryInterval() time.Duration {
	if s == nil || s.requestDetailRetryInterval <= 0 {
		return usageRequestDetailDefaultRetryInterval
	}
	return s.requestDetailRetryInterval
}

func (s *usageService) resolveAPIGroupKey(apiKeyID string) (string, error) {
	apiKeyID = strings.TrimSpace(apiKeyID)
	if apiKeyID == "" {
		return "", nil
	}
	parsedID, err := strconv.ParseInt(apiKeyID, 10, 64)
	if err != nil || parsedID <= 0 {
		return "", ErrInvalidID
	}
	apiKey, err := repository.FindActiveCPAAPIKeyByID(s.db, parsedID)
	if err != nil {
		return "", err
	}
	return apiKey.APIKey, nil
}

// Usage 页面里的 Overview tab 下传时间窗口和全局 API-Key，仓储层负责构建 overview 聚合。
func (s *usageService) GetUsageOverview(_ context.Context, filter servicedto.UsageFilter) (*servicedto.UsageOverviewSnapshot, error) {
	apiGroupKey, err := s.resolveAPIGroupKey(filter.APIKeyID)
	if err != nil {
		return nil, err
	}
	overview, err := repository.BuildUsageOverviewWithFilter(s.db, repodto.UsageQueryFilter{
		Range:       filter.Range,
		StartTime:   filter.StartTime,
		EndTime:     filter.EndTime,
		APIGroupKey: apiGroupKey,
	})
	if err != nil {
		return nil, err
	}
	return &servicedto.UsageOverviewSnapshot{
		Usage: overview.Usage,
		Summary: servicedto.UsageOverviewSummary{
			RequestCount:    overview.Summary.RequestCount,
			TokenCount:      overview.Summary.TokenCount,
			WindowMinutes:   overview.Summary.WindowMinutes,
			RPM:             overview.Summary.RPM,
			TPM:             overview.Summary.TPM,
			TotalCost:       overview.Summary.TotalCost,
			CostAvailable:   overview.Summary.CostAvailable,
			CachedTokens:    overview.Summary.CachedTokens,
			ReasoningTokens: overview.Summary.ReasoningTokens,
		},
		Series:       mapUsageOverviewSeries(overview.Series),
		HourlySeries: mapUsageOverviewSeries(overview.HourlySeries),
		DailySeries:  mapUsageOverviewSeries(overview.DailySeries),
		Health: servicedto.UsageOverviewHealth{
			TotalSuccess:  overview.Health.TotalSuccess,
			TotalFailure:  overview.Health.TotalFailure,
			SuccessRate:   overview.Health.SuccessRate,
			Rows:          overview.Health.Rows,
			Columns:       overview.Health.Columns,
			BucketSeconds: overview.Health.BucketSeconds,
			WindowStart:   overview.Health.WindowStart,
			WindowEnd:     overview.Health.WindowEnd,
			BlockDetails: func() []servicedto.UsageOverviewHealthBlock {
				blocks := make([]servicedto.UsageOverviewHealthBlock, 0, len(overview.Health.BlockDetails))
				for _, block := range overview.Health.BlockDetails {
					blocks = append(blocks, servicedto.UsageOverviewHealthBlock{
						StartTime: block.StartTime,
						EndTime:   block.EndTime,
						Success:   block.Success,
						Failure:   block.Failure,
						Rate:      block.Rate,
					})
				}
				return blocks
			}(),
		},
	}, nil
}

func mapUsageOverviewSeries(series repodto.UsageOverviewSeriesRecord) servicedto.UsageOverviewSeries {
	models := make(map[string]servicedto.UsageOverviewSeries, len(series.Models))
	for model, modelSeries := range series.Models {
		models[model] = mapUsageOverviewSeries(modelSeries)
	}
	return servicedto.UsageOverviewSeries{
		Requests:        series.Requests,
		Tokens:          series.Tokens,
		RPM:             series.RPM,
		TPM:             series.TPM,
		Cost:            series.Cost,
		InputTokens:     series.InputTokens,
		OutputTokens:    series.OutputTokens,
		CachedTokens:    series.CachedTokens,
		ReasoningTokens: series.ReasoningTokens,
		Models:          models,
	}
}

func (s *usageService) GetAnalysis(_ context.Context, filter servicedto.UsageFilter) (*servicedto.AnalysisSnapshot, error) {
	apiGroupKey, err := s.resolveAPIGroupKey(filter.APIKeyID)
	if err != nil {
		return nil, err
	}
	record, err := repository.BuildAnalysisWithFilter(s.db, repodto.UsageQueryFilter{
		Range:       filter.Range,
		StartTime:   filter.StartTime,
		EndTime:     filter.EndTime,
		APIGroupKey: apiGroupKey,
	})
	if err != nil {
		return nil, err
	}
	return mapAnalysisRecord(record), nil
}

func mapAnalysisRecord(record *repodto.AnalysisRecord) *servicedto.AnalysisSnapshot {
	if record == nil {
		return &servicedto.AnalysisSnapshot{}
	}
	tokenUsage := make([]servicedto.AnalysisTokenUsageBucket, 0, len(record.TokenUsage))
	for _, bucket := range record.TokenUsage {
		tokenUsage = append(tokenUsage, servicedto.AnalysisTokenUsageBucket{
			Bucket:          bucket.Bucket,
			InputTokens:     bucket.InputTokens,
			OutputTokens:    bucket.OutputTokens,
			CachedTokens:    bucket.CachedTokens,
			ReasoningTokens: bucket.ReasoningTokens,
			TotalTokens:     bucket.TotalTokens,
			Requests:        bucket.Requests,
		})
	}
	apiKeys := make([]servicedto.AnalysisCompositionItem, 0, len(record.APIKeyComposition))
	for _, item := range record.APIKeyComposition {
		apiKeys = append(apiKeys, mapAnalysisCompositionRecord(item))
	}
	models := make([]servicedto.AnalysisCompositionItem, 0, len(record.ModelComposition))
	for _, item := range record.ModelComposition {
		models = append(models, mapAnalysisCompositionRecord(item))
	}
	authFiles := make([]servicedto.AnalysisCompositionItem, 0, len(record.AuthFilesComposition))
	for _, item := range record.AuthFilesComposition {
		authFiles = append(authFiles, mapAnalysisCompositionRecord(item))
	}
	aiProviders := make([]servicedto.AnalysisCompositionItem, 0, len(record.AIProviderComposition))
	for _, item := range record.AIProviderComposition {
		aiProviders = append(aiProviders, mapAnalysisCompositionRecord(item))
	}
	heatmap := make([]servicedto.AnalysisHeatmapCell, 0, len(record.Heatmap))
	for _, cell := range record.Heatmap {
		heatmap = append(heatmap, servicedto.AnalysisHeatmapCell{
			APIKey:      cell.APIKey,
			Model:       cell.Model,
			TotalTokens: cell.TotalTokens,
			Requests:    cell.Requests,
		})
	}
	return &servicedto.AnalysisSnapshot{
		Granularity:           servicedto.AnalysisGranularity(record.Granularity),
		RangeStart:            record.RangeStart,
		RangeEnd:              record.RangeEnd,
		TokenUsage:            tokenUsage,
		APIKeyComposition:     apiKeys,
		ModelComposition:      models,
		AuthFilesComposition:  authFiles,
		AIProviderComposition: aiProviders,
		Heatmap:               heatmap,
	}
}

func mapAnalysisCompositionRecord(item repodto.AnalysisCompositionRecord) servicedto.AnalysisCompositionItem {
	return servicedto.AnalysisCompositionItem{
		Key:             item.Key,
		Label:           item.Label,
		TotalTokens:     item.TotalTokens,
		Requests:        item.Requests,
		InputTokens:     item.InputTokens,
		OutputTokens:    item.OutputTokens,
		CachedTokens:    item.CachedTokens,
		ReasoningTokens: item.ReasoningTokens,
	}
}

// Usage 页面里的 Request Event Log tab 下传分页、列表筛选条件和全局 API-Key。
func (s *usageService) ListUsageEvents(_ context.Context, filter servicedto.UsageFilter) (*servicedto.UsageEventsPage, error) {
	apiGroupKey, err := s.resolveAPIGroupKey(filter.APIKeyID)
	if err != nil {
		return nil, err
	}
	page, err := repository.ListUsageEventsWithFilter(s.db, repodto.UsageQueryFilter{
		StartTime:   filter.StartTime,
		EndTime:     filter.EndTime,
		Limit:       filter.Limit,
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		Offset:      filter.Offset,
		Model:       filter.Model,
		Source:      filter.Source,
		AuthIndex:   filter.AuthIndex,
		APIGroupKey: apiGroupKey,
		Result:      filter.Result,
		Query:       filter.Query,
	})
	if err != nil {
		return nil, err
	}
	result := make([]servicedto.UsageEventRecord, 0, len(page.Events))
	for _, row := range page.Events {
		result = append(result, mapUsageEventRecord(row))
	}
	return &servicedto.UsageEventsPage{Events: result, Models: page.Models, TotalCount: page.TotalCount, Page: page.Page, PageSize: page.PageSize, TotalPages: page.TotalPages}, nil
}

func mapUsageEventRecord(row repodto.UsageEventRecord) servicedto.UsageEventRecord {
	var attempts []servicedto.UsageEventAttemptRecord
	if len(row.Attempts) > 0 {
		attempts = make([]servicedto.UsageEventAttemptRecord, 0, len(row.Attempts))
		for _, attempt := range row.Attempts {
			attempts = append(attempts, servicedto.UsageEventAttemptRecord{
				ID:          attempt.ID,
				Timestamp:   attempt.Timestamp,
				Model:       attempt.Model,
				AuthType:    attempt.AuthType,
				Provider:    attempt.Provider,
				Source:      attempt.Source,
				AuthIndex:   attempt.AuthIndex,
				Failed:      attempt.Failed,
				LatencyMS:   attempt.LatencyMS,
				TotalTokens: attempt.TotalTokens,
			})
		}
	}
	return servicedto.UsageEventRecord{
		ID:                  row.ID,
		Timestamp:           row.Timestamp,
		APIGroupKey:         row.APIGroupKey,
		Model:               row.Model,
		ReasoningEffort:     row.ReasoningEffort,
		ExecutorType:        row.ExecutorType,
		ServiceTier:         row.ServiceTier,
		Endpoint:            row.Endpoint,
		AuthType:            row.AuthType,
		Provider:            row.Provider,
		Source:              row.Source,
		AuthIndex:           row.AuthIndex,
		RequestID:           row.RequestID,
		Failed:              row.Failed,
		LatencyMS:           row.LatencyMS,
		TTFTMS:              row.TTFTMS,
		InputTokens:         row.InputTokens,
		OutputTokens:        row.OutputTokens,
		ReasoningTokens:     row.ReasoningTokens,
		CachedTokens:        row.CachedTokens,
		CacheReadTokens:     row.CacheReadTokens,
		CacheCreationTokens: row.CacheCreationTokens,
		TotalTokens:         row.TotalTokens,
		CostUSD:             row.CostUSD,
		CostAvailable:       row.CostAvailable,
		PricingStyle:        row.PricingStyle,
		AttemptCount:        row.AttemptCount,
		Attempts:            attempts,
	}
}

// usageRequestDetailSourceCLIProxyAPI 标记详情缓存来源，统一 lazy fetch 与同步期预取的写入值。
const usageRequestDetailSourceCLIProxyAPI = "cliproxyapi"

// GetUsageEventRequestDetail 按本地 usage event id 查询 request_id，再从缓存或 CLIProxyAPI 获取详情。
func (s *usageService) GetUsageEventRequestDetail(ctx context.Context, eventID string) (*servicedto.UsageEventRequestDetail, error) {
	parsedID, err := strconv.ParseInt(strings.TrimSpace(eventID), 10, 64)
	if err != nil || parsedID <= 0 {
		return nil, ErrInvalidID
	}
	event, err := repository.GetUsageEventByID(s.db, parsedID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUsageEventNotFound
		}
		return nil, err
	}
	requestID := strings.TrimSpace(event.RequestID)
	if requestID == "" {
		return nil, ErrUsageEventRequestUnavailable
	}
	detail, cached, err := s.fetchAndCacheUsageRequestDetail(ctx, event)
	if err != nil {
		return nil, err
	}
	attempts, err := s.buildUsageEventRequestDetailAttempts(event, detail, cached)
	if err != nil {
		return nil, err
	}
	return &servicedto.UsageEventRequestDetail{UsageEventID: event.ID, RequestID: requestID, Content: detail.Content, Cached: cached, FetchedAt: detail.FetchedAt, Attempts: attempts}, nil
}

func (s *usageService) buildUsageEventRequestDetailAttempts(selectedEvent repodto.UsageEventRecord, selectedDetail entities.UsageRequestDetail, selectedCached bool) ([]servicedto.UsageEventRequestDetailAttempt, error) {
	requestID := strings.TrimSpace(selectedEvent.RequestID)
	if requestID == "" {
		return []servicedto.UsageEventRequestDetailAttempt{}, nil
	}
	rows, err := repository.ListUsageEventAttemptsByRequestID(s.db, requestID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		rows = []repodto.UsageEventRecord{selectedEvent}
	}

	attempts := make([]servicedto.UsageEventRequestDetailAttempt, 0, len(rows))
	for _, row := range rows {
		detail := selectedDetail
		cached := selectedCached
		fallbackDetail, fallbackErr := buildUsageRequestDetailFromRedisInbox(s.db, row)
		if fallbackErr == nil {
			detail = fallbackDetail
			cached = false
		} else if !errors.Is(fallbackErr, gorm.ErrRecordNotFound) {
			return nil, fallbackErr
		}
		attempts = append(attempts, servicedto.UsageEventRequestDetailAttempt{
			UsageEventID: row.ID,
			Timestamp:    row.Timestamp,
			Failed:       row.Failed,
			Model:        row.Model,
			Content:      detail.Content,
			Cached:       cached,
			FetchedAt:    detail.FetchedAt,
		})
	}
	return attempts, nil
}

func (s *usageService) fetchAndCacheUsageRequestDetail(ctx context.Context, event repodto.UsageEventRecord) (entities.UsageRequestDetail, bool, error) {
	requestID := strings.TrimSpace(event.RequestID)
	if requestID == "" {
		return entities.UsageRequestDetail{}, false, ErrUsageEventRequestUnavailable
	}
	if detail, err := repository.GetUsageRequestDetailByRequestID(s.db, requestID); err == nil {
		return detail, true, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return entities.UsageRequestDetail{}, false, err
	}
	settings, err := repository.GetDatabaseCleanupSettings(s.db)
	if err != nil {
		return entities.UsageRequestDetail{}, false, err
	}
	if !settings.RecordRequestDetails {
		return entities.UsageRequestDetail{}, false, ErrUsageEventRequestUnavailable
	}
	if s.requestLogFetcher == nil {
		return entities.UsageRequestDetail{}, false, ErrUsageEventRequestUpstream
	}

	result, err := s.fetchRequestLogByIDWithWait(ctx, event)
	if err != nil {
		switch {
		case errors.Is(err, cpa.ErrRequestLogNotFound):
			if detail, fallbackErr := buildUsageRequestDetailFromRedisInbox(s.db, event); fallbackErr == nil {
				return detail, false, nil
			} else if !errors.Is(fallbackErr, gorm.ErrRecordNotFound) {
				return entities.UsageRequestDetail{}, false, fallbackErr
			}
			return entities.UsageRequestDetail{}, false, ErrUsageEventRequestUpstreamPending
		case errors.Is(err, cpa.ErrRequestLogTooLarge):
			return entities.UsageRequestDetail{}, false, ErrUsageEventRequestTooLarge
		default:
			return entities.UsageRequestDetail{}, false, fmt.Errorf("%w: %v", ErrUsageEventRequestUpstream, err)
		}
	}
	detail, err := repository.SaveUsageRequestDetail(s.db, entities.UsageRequestDetail{RequestID: requestID, Content: result.Content, Source: usageRequestDetailSourceCLIProxyAPI, FetchedAt: time.Now()})
	if err != nil {
		return entities.UsageRequestDetail{}, false, err
	}
	if err := repository.EnforceUsageRequestDetailLimit(s.db, usageRequestDetailCacheLimit); err != nil {
		return entities.UsageRequestDetail{}, false, err
	}
	return detail, false, nil
}

func (s *usageService) fetchRequestLogByIDWithWait(ctx context.Context, event repodto.UsageEventRecord) (*response.RequestLogResult, error) {
	requestID := strings.TrimSpace(event.RequestID)
	if requestID == "" {
		return nil, ErrUsageEventRequestUnavailable
	}
	deadline := time.Now().Add(s.detailWaitTimeout())
	var lastErr error
	for {
		result, err := s.requestLogFetcher.FetchRequestLogByID(ctx, requestID)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, cpa.ErrRequestLogNotFound) {
			return nil, err
		}
		lastErr = err
		if !time.Now().Before(deadline) {
			return nil, lastErr
		}
		wait := s.detailRetryInterval()
		if remaining := time.Until(deadline); remaining < wait {
			wait = remaining
		}
		if wait <= 0 {
			return nil, lastErr
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

type usageRequestDetailFallbackPayload struct {
	Timestamp       time.Time           `json:"timestamp"`
	LatencyMS       int64               `json:"latency_ms"`
	TTFTMS          *int64              `json:"ttft_ms"`
	Source          string              `json:"source"`
	AuthIndex       string              `json:"auth_index"`
	Provider        string              `json:"provider"`
	Model           string              `json:"model"`
	Endpoint        string              `json:"endpoint"`
	AuthType        string              `json:"auth_type"`
	RequestID       string              `json:"request_id"`
	Failed          bool                `json:"failed"`
	Fail            usageRequestFail    `json:"fail"`
	ResponseHeaders map[string][]string `json:"response_headers"`
}

type usageRequestFail struct {
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
}

func buildUsageRequestDetailFromRedisInbox(db *gorm.DB, event repodto.UsageEventRecord) (entities.UsageRequestDetail, error) {
	requestID := strings.TrimSpace(event.RequestID)
	rows, err := repository.ListProcessedRedisUsageInboxByEventKey(db, requestID, usageRequestDetailFallbackCandidateLimit)
	if err != nil {
		return entities.UsageRequestDetail{}, err
	}
	if len(rows) == 0 {
		return entities.UsageRequestDetail{}, gorm.ErrRecordNotFound
	}
	row, rawMessage, err := selectRedisUsageRequestDetailFallback(rows, event.Timestamp)
	if err != nil {
		return entities.UsageRequestDetail{}, err
	}
	content, err := formatRedisUsageRequestDetail(rawMessage)
	if err != nil {
		return entities.UsageRequestDetail{}, err
	}
	fetchedAt := row.PoppedAt
	if row.ProcessedAt != nil && !row.ProcessedAt.IsZero() {
		fetchedAt = *row.ProcessedAt
	}
	return entities.UsageRequestDetail{
		RequestID: strings.TrimSpace(requestID),
		Content:   content,
		Source:    "redis_usage_inbox",
		FetchedAt: fetchedAt,
		CreatedAt: fetchedAt,
		UpdatedAt: fetchedAt,
	}, nil
}

func selectRedisUsageRequestDetailFallback(rows []entities.RedisUsageInbox, eventTimestamp time.Time) (entities.RedisUsageInbox, string, error) {
	var selected entities.RedisUsageInbox
	selectedRawMessage := ""
	var selectedDistance time.Duration
	hasSelected := false
	for _, row := range rows {
		var payload usageRequestDetailFallbackPayload
		if err := json.Unmarshal([]byte(row.RawMessage), &payload); err != nil {
			continue
		}
		distance := time.Duration(0)
		if !eventTimestamp.IsZero() && !payload.Timestamp.IsZero() {
			distance = absDuration(timeutil.NormalizeStorageTime(payload.Timestamp).Sub(timeutil.NormalizeStorageTime(eventTimestamp)))
		}
		if !hasSelected || distance < selectedDistance {
			selected = row
			selectedRawMessage = row.RawMessage
			selectedDistance = distance
			hasSelected = true
		}
	}
	if !hasSelected {
		return entities.RedisUsageInbox{}, "", gorm.ErrRecordNotFound
	}
	return selected, selectedRawMessage, nil
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func formatRedisUsageRequestDetail(rawMessage string) (string, error) {
	var payload usageRequestDetailFallbackPayload
	if err := json.Unmarshal([]byte(rawMessage), &payload); err != nil {
		return "", fmt.Errorf("decode redis usage detail fallback: %w", err)
	}
	method, path := parseUsageEventEndpoint(payload.Endpoint)
	statusCode := payload.Fail.StatusCode
	var builder strings.Builder
	builder.WriteString("=== REQUEST INFO ===\n")
	writeDetailLine(&builder, "URL", path)
	writeDetailLine(&builder, "Method", method)
	writeDetailLine(&builder, "Timestamp", payload.Timestamp.Format(time.RFC3339Nano))
	writeDetailLine(&builder, "Provider", payload.Provider)
	writeDetailLine(&builder, "Model", payload.Model)
	writeDetailLine(&builder, "Source", payload.Source)
	writeDetailLine(&builder, "Auth Type", payload.AuthType)
	writeDetailLine(&builder, "Auth Index", payload.AuthIndex)
	writeDetailLine(&builder, "Failed", strconv.FormatBool(payload.Failed))
	if payload.LatencyMS > 0 {
		writeDetailLine(&builder, "Latency", fmt.Sprintf("%dms", payload.LatencyMS))
	}
	if payload.TTFTMS != nil && *payload.TTFTMS > 0 {
		writeDetailLine(&builder, "TTFT", fmt.Sprintf("%dms", *payload.TTFTMS))
	}
	builder.WriteString("\n\n=== RESPONSE ===\n")
	if statusCode > 0 {
		writeDetailLine(&builder, "Status", strconv.Itoa(statusCode))
	}
	builder.WriteString(formatHeaderLines(payload.ResponseHeaders))
	body := strings.TrimSpace(payload.Fail.Body)
	if body != "" {
		builder.WriteString("\n")
		builder.WriteString(formatJSONLikeText(body))
		builder.WriteString("\n")
	}
	builder.WriteString("\n=== REQUEST BODY ===\n")
	builder.WriteString(formatJSONLikeText(rawMessage))
	builder.WriteString("\n")
	return builder.String(), nil
}

func parseUsageEventEndpoint(endpoint string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(endpoint))
	if len(parts) >= 2 {
		return strings.ToUpper(parts[0]), parts[1]
	}
	if len(parts) == 1 {
		return "", parts[0]
	}
	return "", ""
}

func writeDetailLine(builder *strings.Builder, key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	builder.WriteString(key)
	builder.WriteString(": ")
	builder.WriteString(value)
	builder.WriteString("\n")
}

func formatHeaderLines(headers map[string][]string) string {
	if len(headers) == 0 {
		return ""
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		values := headers[key]
		joined := strings.Join(values, ", ")
		writeDetailLine(&builder, key, joined)
	}
	return builder.String()
}

func formatJSONLikeText(value string) string {
	var parsed any
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return strings.TrimSpace(value)
	}
	formatted, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return strings.TrimSpace(value)
	}
	return string(formatted)
}

func (s *usageService) ListUsageEventFilterOptions(_ context.Context, filter servicedto.UsageFilter) (*servicedto.UsageEventFilterOptions, error) {
	options, err := repository.ListUsageEventFilterOptionsWithFilter(s.db, repodto.UsageQueryFilter{
		StartTime: filter.StartTime,
		EndTime:   filter.EndTime,
	})
	if err != nil {
		return nil, err
	}
	return &servicedto.UsageEventFilterOptions{Models: options.Models}, nil
}
