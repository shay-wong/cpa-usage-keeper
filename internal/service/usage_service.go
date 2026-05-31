package service

import (
	"context"

	servicedto "cpa-usage-keeper/internal/service/dto"
)

type UsageProvider interface {
	GetUsageOverview(context.Context, servicedto.UsageFilter) (*servicedto.UsageOverviewSnapshot, error)
	ListUsageEvents(context.Context, servicedto.UsageFilter) (*servicedto.UsageEventsPage, error)
	ListUsageEventFilterOptions(context.Context, servicedto.UsageFilter) (*servicedto.UsageEventFilterOptions, error)
	GetUsageEventRequestDetail(context.Context, string) (*servicedto.UsageEventRequestDetail, error)
	GetAnalysis(context.Context, servicedto.UsageFilter) (*servicedto.AnalysisSnapshot, error)
}
