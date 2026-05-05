package repository

import "time"

type UsageMonitoringSourceTargetRecord struct {
	Source    string
	AuthIndex string
}

type UsageMonitoringSourceModelTargetRecord struct {
	Source    string
	AuthIndex string
	Model     string
}

type UsageMonitoringRecentRequestRecord struct {
	Source      string
	AuthIndex   string
	Model       string
	Timestamp   time.Time
	Failed      bool
	ModelScoped bool
}

type UsageMonitoringHourlyModelStatRecord struct {
	Hour         string
	Model        string
	Requests     int64
	Tokens       int64
	SuccessCount int64
	FailureCount int64
}

type UsageMonitoringChannelStatRecord struct {
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
	LastRequestTime time.Time
}

type UsageMonitoringChannelModelStatRecord struct {
	Source          string
	AuthIndex       string
	Model           string
	Requests        int64
	Success         int64
	Failed          int64
	TotalTokens     int64
	LastRequestTime time.Time
}

type UsageMonitoringFailureStatRecord struct {
	Source       string
	AuthIndex    string
	FailedCount  int64
	LastFailTime time.Time
}

type UsageMonitoringFailureModelStatRecord struct {
	Source        string
	AuthIndex     string
	Model         string
	Success       int64
	Failure       int64
	Total         int64
	LastTimestamp time.Time
}
