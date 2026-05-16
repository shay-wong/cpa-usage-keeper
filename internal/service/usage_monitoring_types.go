package service

import "time"

type UsageMonitoringRecentRequest struct {
	Timestamp time.Time
	Failed    bool
}

type UsageMonitoringKPI struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TotalTokens     int64
	InputTokens     int64
	OutputTokens    int64
	CachedTokens    int64
	ReasoningTokens int64
	RPM             float64
	TPM             float64
	TotalCost       float64
	CostAvailable   bool
}

type UsageMonitoringModelDistributionItem struct {
	Model           string
	TotalRequests   int64
	SuccessCount    int64
	FailureCount    int64
	TotalTokens     int64
	InputTokens     int64
	OutputTokens    int64
	CachedTokens    int64
	ReasoningTokens int64
	SuccessRate     float64
}

type UsageMonitoringTrendPoint struct {
	Bucket          string
	Requests        int64
	Tokens          int64
	InputTokens     int64
	OutputTokens    int64
	CachedTokens    int64
	ReasoningTokens int64
	Cost            float64
}

type UsageMonitoringHourlyModelPoint struct {
	Hour   string
	Models []UsageMonitoringHourlyModelStat
}

type UsageMonitoringHourlyModelStat struct {
	Model        string
	Requests     int64
	Tokens       int64
	SuccessCount int64
	FailureCount int64
}

type UsageMonitoringChannelModelStat struct {
	Model           string
	Requests        int64
	Success         int64
	Failed          int64
	SuccessRate     float64
	TotalTokens     int64
	LastRequestTime *time.Time
	RecentRequests  []UsageMonitoringRecentRequest
}

type UsageMonitoringChannelStat struct {
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
	SuccessRate     float64
	LastRequestTime *time.Time
	RecentRequests  []UsageMonitoringRecentRequest
	Models          []UsageMonitoringChannelModelStat
}

type UsageMonitoringFailureModelStat struct {
	Model          string
	Success        int64
	Failure        int64
	Total          int64
	SuccessRate    float64
	LastTimestamp  *time.Time
	RecentRequests []UsageMonitoringRecentRequest
}

type UsageMonitoringFailureStat struct {
	Source       string
	AuthIndex    string
	FailedCount  int64
	LastFailTime *time.Time
	Models       []UsageMonitoringFailureModelStat
}

type UsageMonitoringRequestLog struct {
	ID              int64
	Timestamp       time.Time
	Model           string
	Source          string
	AuthIndex       string
	Failed          bool
	LatencyMS       int64
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CachedTokens    int64
	TotalTokens     int64
}

type UsageMonitoringSnapshot struct {
	KPIs              UsageMonitoringKPI
	ModelDistribution []UsageMonitoringModelDistributionItem
	DailyTrend        []UsageMonitoringTrendPoint
	HourlyModelTrend  []UsageMonitoringHourlyModelPoint
	HourlyTokenTrend  []UsageMonitoringTrendPoint
	ChannelStats      []UsageMonitoringChannelStat
	FailureAnalysis   []UsageMonitoringFailureStat
	RequestLogs       []UsageMonitoringRequestLog
}
