package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/helper"
	"cpa-usage-keeper/internal/service"
	servicedto "cpa-usage-keeper/internal/service/dto"
	"cpa-usage-keeper/internal/timeutil"

	"github.com/gin-gonic/gin"
)

type usageEventsResponse struct {
	Events     []usageEventPayload `json:"events"`
	TotalCount int64               `json:"total_count"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
	TotalPages int                 `json:"total_pages"`
}

type usageEventRequestDetailPayload struct {
	UsageEventID string                                  `json:"usage_event_id"`
	RequestID    string                                  `json:"request_id"`
	Content      string                                  `json:"content"`
	Cached       bool                                    `json:"cached"`
	FetchedAt    string                                  `json:"fetched_at"`
	Attempts     []usageEventRequestDetailAttemptPayload `json:"attempts,omitempty"`
}

type usageEventRequestDetailAttemptPayload struct {
	UsageEventID string `json:"usage_event_id"`
	Timestamp    string `json:"timestamp"`
	Failed       bool   `json:"failed"`
	Model        string `json:"model,omitempty"`
	Content      string `json:"content"`
	Cached       bool   `json:"cached"`
	FetchedAt    string `json:"fetched_at"`
}

type usageEventDetailErrorPayload struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type usageSourceFilterOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	DisplayName string `json:"displayName"`
}

type usageEventFilterOptionsResponse struct {
	Models  []string                  `json:"models"`
	Sources []usageSourceFilterOption `json:"sources"`
}

type usageEventPayload struct {
	ID              string                     `json:"id,omitempty"`
	RequestID       string                     `json:"request_id,omitempty"`
	Timestamp       string                     `json:"timestamp"`
	APIKey          string                     `json:"api_key,omitempty"`
	Model           string                     `json:"model"`
	ReasoningEffort string                     `json:"reasoning_effort,omitempty"`
	ExecutorType    string                     `json:"executor_type,omitempty"`
	ServiceTier     string                     `json:"service_tier,omitempty"`
	Endpoint        string                     `json:"endpoint,omitempty"`
	Source          string                     `json:"source"`
	SourceNote      string                     `json:"source_note,omitempty"`
	SourceRaw       string                     `json:"source_raw,omitempty"`
	SourceType      string                     `json:"source_type,omitempty"`
	IsDelete        bool                       `json:"isDelete,omitempty"`
	Failed          bool                       `json:"failed"`
	LatencyMS       int64                      `json:"latency_ms"`
	TTFTMS          *int64                     `json:"ttft_ms,omitempty"`
	SpeedTPS        *float64                   `json:"speed_tps,omitempty"`
	Tokens          usageEventTokenPayload     `json:"tokens"`
	CostUSD         float64                    `json:"cost_usd"`
	CostAvailable   bool                       `json:"cost_available"`
	PricingStyle    string                     `json:"pricing_style,omitempty"`
	AttemptCount    int                        `json:"attempt_count,omitempty"`
	Attempts        []usageEventAttemptPayload `json:"attempts,omitempty"`
}

type usageEventAttemptPayload struct {
	ID          string `json:"id,omitempty"`
	Timestamp   string `json:"timestamp"`
	Source      string `json:"source,omitempty"`
	SourceType  string `json:"source_type,omitempty"`
	Failed      bool   `json:"failed"`
	LatencyMS   int64  `json:"latency_ms"`
	TotalTokens int64  `json:"total_tokens"`
}

type usageEventTokenPayload struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	ReasoningTokens     int64 `json:"reasoning_tokens"`
	CachedTokens        int64 `json:"cached_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	TotalTokens         int64 `json:"total_tokens"`
}

func registerUsageEventsRoute(
	router gin.IRoutes,
	usageProvider service.UsageProvider,
	usageIdentityProvider service.UsageIdentityProvider,
	cpaAPIKeyProvider service.CPAAPIKeyProvider,
) {
	router.GET("/usage/events/filters/models", func(c *gin.Context) {
		models, err := loadUsageEventModelFilterOptions(c, usageProvider)
		if err != nil {
			writeInternalError(c, "list usage event model filter options failed", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"models": models})
	})

	router.GET("/usage/events/filters/sources", func(c *gin.Context) {
		sources, err := loadUsageEventSourceFilterOptions(c, usageIdentityProvider)
		if err != nil {
			writeInternalError(c, "list usage event source filter options failed", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"sources": sources})
	})

	router.GET("/usage/events/:id/detail", func(c *gin.Context) {
		if usageProvider == nil {
			writeUsageEventDetailError(c, http.StatusNotFound, "event_not_found")
			return
		}
		detail, err := usageProvider.GetUsageEventRequestDetail(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeUsageEventDetailServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, usageEventRequestDetailPayload{
			UsageEventID: strconv.FormatInt(detail.UsageEventID, 10),
			RequestID:    detail.RequestID,
			Content:      detail.Content,
			Cached:       detail.Cached,
			FetchedAt:    timeutil.FormatStorageTime(detail.FetchedAt),
			Attempts:     buildUsageEventRequestDetailAttemptPayloads(detail.Attempts),
		})
	})

	router.GET("/usage/events", func(c *gin.Context) {
		if usageProvider == nil {
			c.JSON(http.StatusOK, usageEventsResponse{Events: []usageEventPayload{}, Page: 1, PageSize: servicedto.DefaultUsageEventsLimit})
			return
		}

		filter, err := parseUsageFilterQuery(c.Request, timeutil.NormalizeStorageTime(time.Now()))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		identities, err := loadUsageResolutionData(c, usageIdentityProvider)
		if err != nil {
			writeInternalError(c, "load usage resolution data failed", err)
			return
		}
		if err := applyUsageEventsSourceFilter(&filter, identities); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		rows, err := usageProvider.ListUsageEvents(c.Request.Context(), filter)
		if err != nil {
			writeInternalError(c, "list usage events failed", err)
			return
		}

		resolver := newUsageIdentityResolver(identities)
		apiKeyInfos, err := loadCPAAPIKeyInfos(c, cpaAPIKeyProvider)
		if err != nil {
			return
		}
		c.JSON(http.StatusOK, usageEventsResponse{
			Events:     buildUsageEventsPayload(rows.Events, resolver, apiKeyInfos),
			TotalCount: rows.TotalCount,
			Page:       rows.Page,
			PageSize:   rows.PageSize,
			TotalPages: rows.TotalPages,
		})
	})
}

func buildUsageEventRequestDetailAttemptPayloads(attempts []servicedto.UsageEventRequestDetailAttempt) []usageEventRequestDetailAttemptPayload {
	if len(attempts) == 0 {
		return nil
	}
	payload := make([]usageEventRequestDetailAttemptPayload, 0, len(attempts))
	for _, attempt := range attempts {
		payload = append(payload, usageEventRequestDetailAttemptPayload{
			UsageEventID: strconv.FormatInt(attempt.UsageEventID, 10),
			Timestamp:    timeutil.FormatStorageTime(attempt.Timestamp),
			Failed:       attempt.Failed,
			Model:        strings.TrimSpace(attempt.Model),
			Content:      attempt.Content,
			Cached:       attempt.Cached,
			FetchedAt:    timeutil.FormatStorageTime(attempt.FetchedAt),
		})
	}
	return payload
}

// Source 下拉提交的是 usage identity，进入仓储前转换成 auth_index 查询。
func applyUsageEventsSourceFilter(filter *servicedto.UsageFilter, identities []entities.UsageIdentity) error {
	if filter == nil {
		return nil
	}
	source := strings.TrimSpace(filter.Source)
	if source == "" {
		return nil
	}
	for _, identity := range identities {
		if usageEventSourceFilterValue(identity) != source {
			continue
		}
		filter.AuthIndex = strings.TrimSpace(identity.Identity)
		filter.Source = ""
		return nil
	}
	if strings.HasPrefix(source, "provider:") {
		idValue := strings.TrimSpace(strings.TrimPrefix(source, "provider:"))
		for _, identity := range identities {
			if identity.AuthType != entities.UsageIdentityAuthTypeAIProvider || int64ToString(identity.ID) != idValue {
				continue
			}
			filter.AuthIndex = strings.TrimSpace(identity.Identity)
			filter.Source = ""
			return nil
		}
	}
	if strings.HasPrefix(source, "auth:") {
		alias := strings.TrimSpace(strings.TrimPrefix(source, "auth:"))
		for _, identity := range identities {
			if identity.AuthType != entities.UsageIdentityAuthTypeAuthFile || helper.SensitiveValueAlias(identity.Identity) != alias {
				continue
			}
			filter.AuthIndex = strings.TrimSpace(identity.Identity)
			filter.Source = ""
			return nil
		}
	}
	filter.AuthIndex = source
	filter.Source = ""
	return nil
}

func writeUsageEventDetailServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidID):
		writeUsageEventDetailError(c, http.StatusBadRequest, "invalid_event_id")
	case errors.Is(err, service.ErrUsageEventNotFound):
		writeUsageEventDetailError(c, http.StatusNotFound, "event_not_found")
	case errors.Is(err, service.ErrUsageEventRequestUnavailable):
		writeUsageEventDetailError(c, http.StatusNotFound, "request_detail_unavailable")
	case errors.Is(err, service.ErrUsageEventRequestUpstreamPending):
		writeUsageEventDetailError(c, http.StatusNotFound, "upstream_log_pending")
	case errors.Is(err, service.ErrUsageEventRequestUpstreamNotFound):
		writeUsageEventDetailError(c, http.StatusNotFound, "upstream_log_not_found")
	case errors.Is(err, service.ErrUsageEventRequestTooLarge):
		writeUsageEventDetailError(c, http.StatusRequestEntityTooLarge, "request_detail_too_large")
	case errors.Is(err, service.ErrUsageEventRequestUpstream):
		writeUsageEventDetailError(c, http.StatusBadGateway, "upstream_request_failed")
	default:
		writeUsageEventDetailError(c, http.StatusInternalServerError, "internal_error")
	}
}

func writeUsageEventDetailError(c *gin.Context, status int, code string) {
	c.JSON(status, usageEventDetailErrorPayload{Error: code, Code: code})
}

// 列表结果先按 auth_index 解析展示名，再组装前端需要的事件 payload。
func buildUsageEventsPayload(rows []servicedto.UsageEventRecord, resolver usageIdentityResolver, apiKeyInfos map[string]analysisAPIKeyInfo) []usageEventPayload {
	if len(rows) == 0 {
		return []usageEventPayload{}
	}
	payload := make([]usageEventPayload, 0, len(rows))
	for _, row := range rows {
		identity, matched := resolver.resolveByAuthIndex(row.AuthIndex)
		source, isDelete := usageEventPublicSource(row, identity, matched)
		id := ""
		if row.ID != 0 {
			id = strconv.FormatInt(row.ID, 10)
		}
		payload = append(payload, usageEventPayload{
			ID:              id,
			RequestID:       row.RequestID,
			Timestamp:       timeutil.FormatStorageTime(row.Timestamp),
			APIKey:          usageEventAPIKeyLabel(row.APIGroupKey, apiKeyInfos),
			Model:           row.Model,
			ReasoningEffort: strings.TrimSpace(row.ReasoningEffort),
			ExecutorType:    strings.TrimSpace(row.ExecutorType),
			ServiceTier:     strings.TrimSpace(row.ServiceTier),
			Endpoint:        strings.TrimSpace(row.Endpoint),
			Source:          source,
			SourceNote:      identity.Note,
			SourceType:      identity.Type,
			IsDelete:        isDelete,
			Failed:          row.Failed,
			LatencyMS:       row.LatencyMS,
			TTFTMS:          row.TTFTMS,
			SpeedTPS:        usageEventSpeedTPS(row),
			CostUSD:         row.CostUSD,
			CostAvailable:   row.CostAvailable,
			PricingStyle:    strings.TrimSpace(row.PricingStyle),
			Tokens: usageEventTokenPayload{
				InputTokens:         row.InputTokens,
				OutputTokens:        row.OutputTokens,
				ReasoningTokens:     row.ReasoningTokens,
				CachedTokens:        row.CachedTokens,
				CacheReadTokens:     row.CacheReadTokens,
				CacheCreationTokens: row.CacheCreationTokens,
				TotalTokens:         row.TotalTokens,
			},
			AttemptCount: row.AttemptCount,
			Attempts:     buildUsageEventAttemptPayloads(row.Attempts, resolver),
		})
	}
	return payload
}

func buildUsageEventAttemptPayloads(attempts []servicedto.UsageEventAttemptRecord, resolver usageIdentityResolver) []usageEventAttemptPayload {
	if len(attempts) == 0 {
		return nil
	}
	payload := make([]usageEventAttemptPayload, 0, len(attempts))
	for _, attempt := range attempts {
		identity, matched := resolver.resolveByAuthIndex(attempt.AuthIndex)
		source, _ := usageEventPublicSource(servicedto.UsageEventRecord{
			AuthType:  attempt.AuthType,
			Provider:  attempt.Provider,
			Source:    attempt.Source,
			AuthIndex: attempt.AuthIndex,
		}, identity, matched)
		id := ""
		if attempt.ID != 0 {
			id = strconv.FormatInt(attempt.ID, 10)
		}
		payload = append(payload, usageEventAttemptPayload{
			ID:          id,
			Timestamp:   timeutil.FormatStorageTime(attempt.Timestamp),
			Source:      source,
			SourceType:  identity.Type,
			Failed:      attempt.Failed,
			LatencyMS:   attempt.LatencyMS,
			TotalTokens: attempt.TotalTokens,
		})
	}
	return payload
}

func usageEventSpeedTPS(row servicedto.UsageEventRecord) *float64 {
	visibleOutputTokens := row.OutputTokens - row.ReasoningTokens
	if visibleOutputTokens < 0 {
		visibleOutputTokens = 0
	}
	if row.TTFTMS == nil || *row.TTFTMS <= 0 || row.LatencyMS <= *row.TTFTMS || visibleOutputTokens <= 1 {
		return nil
	}
	// Speed 只衡量首字后可见输出 token 的平均生成速度，避免把等待首字的时间重复计入。
	speed := float64(visibleOutputTokens-1) / (float64(row.LatencyMS-*row.TTFTMS) / 1000)
	return &speed
}

func usageEventAPIKeyLabel(apiGroupKey string, apiKeyInfos map[string]analysisAPIKeyInfo) string {
	apiKey := strings.TrimSpace(apiGroupKey)
	if apiKey == "" {
		return ""
	}
	return analysisAPIKeyLabel(apiKey, apiKeyInfos)
}

func usageEventPublicSource(row servicedto.UsageEventRecord, identity resolvedUsageIdentity, matched bool) (string, bool) {
	if matched {
		return identity.DisplayName, false
	}
	authIndex := strings.TrimSpace(row.AuthIndex)
	isDelete := authIndex != ""
	switch strings.TrimSpace(row.AuthType) {
	case "apikey":
		provider := safeAIProviderDisplayValue(row.Provider, strings.TrimSpace(row.Source), inferUsageProviderType(row.Provider))
		if provider == "" {
			provider = "AI Provider"
		}
		return provider, isDelete
	case "oauth":
		resolved := usageSourceResolver{}.resolve(firstNonEmptyString(row.Source, authIndex, "unknown"), authIndex)
		return safeUsageSourceDisplay(resolved, authIndex), isDelete
	default:
		if provider := safeAIProviderDisplayValue(row.Provider, strings.TrimSpace(row.Source), inferUsageProviderType(row.Provider)); provider != "" {
			return provider, isDelete
		}
		resolved := usageSourceResolver{}.resolve(firstNonEmptyString(row.Source, authIndex, "unknown"), authIndex)
		return safeUsageSourceDisplay(resolved, authIndex), isDelete
	}
}

func loadUsageEventModelFilterOptions(c *gin.Context, usageProvider service.UsageProvider) ([]string, error) {
	if usageProvider == nil {
		return []string{}, nil
	}
	options, err := usageProvider.ListUsageEventFilterOptions(c.Request.Context(), servicedto.UsageFilter{})
	if err != nil {
		return nil, err
	}
	return options.Models, nil
}

func loadUsageEventSourceFilterOptions(c *gin.Context, usageIdentityProvider service.UsageIdentityProvider) ([]usageSourceFilterOption, error) {
	identities, err := loadUsageResolutionData(c, usageIdentityProvider)
	if err != nil {
		return nil, err
	}
	return buildUsageSourceFilterOptions(identities), nil
}

// Source 筛选项从活跃身份生成，避免把 usage_events.source 当成可选项暴露给页面。
func buildUsageSourceFilterOptions(identities []entities.UsageIdentity) []usageSourceFilterOption {
	if len(identities) == 0 {
		return []usageSourceFilterOption{}
	}
	options := make([]usageSourceFilterOption, 0, len(identities))
	seen := make(map[string]struct{}, len(identities))
	for _, identity := range identities {
		// Source 下拉只展示活跃且有流量的身份，避免已删除身份继续出现在筛选项里。
		if identity.IsDeleted || identity.TotalRequests == 0 {
			continue
		}
		option, ok := usageSourceFilterOptionFromIdentity(identity)
		if !ok {
			continue
		}
		if _, exists := seen[option.Value]; exists {
			continue
		}
		seen[option.Value] = struct{}{}
		options = append(options, option)
	}
	return options
}

func usageSourceFilterOptionFromIdentity(identity entities.UsageIdentity) (usageSourceFilterOption, bool) {
	switch identity.AuthType {
	case entities.UsageIdentityAuthTypeAuthFile, entities.UsageIdentityAuthTypeAIProvider:
		value := usageEventSourceFilterValue(identity)
		if value == "" {
			return usageSourceFilterOption{}, false
		}
		label := strings.TrimSpace(identity.Name)
		displayName := helper.UsageIdentityDisplayName(identity)
		if identity.AuthType == entities.UsageIdentityAuthTypeAuthFile {
			label = safeAuthIdentityDisplayName(label, identity.Identity)
			displayName = safeAuthIdentityDisplayName(displayName, identity.Identity)
		} else {
			sensitiveValue := firstNonEmptyString(identity.LookupKey, identity.Identity)
			label = safeAIProviderDisplayValue(label, sensitiveValue, displayName)
			displayName = safeAIProviderDisplayValue(displayName, sensitiveValue, label)
		}
		return usageSourceFilterOption{Value: value, Label: label, DisplayName: displayName}, true
	default:
		return usageSourceFilterOption{}, false
	}
}

func usageEventSourceFilterValue(identity entities.UsageIdentity) string {
	if identity.ID == 0 || strings.TrimSpace(identity.Identity) == "" {
		return ""
	}
	return "identity:" + int64ToString(identity.ID)
}
