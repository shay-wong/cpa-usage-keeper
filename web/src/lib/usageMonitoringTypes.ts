import type { UsageEventTokens } from './types'

export interface UsageMonitoringRecentRequest {
  timestamp: string
  failed: boolean
}

export interface UsageMonitoringKPI {
  total_requests: number
  success_requests: number
  failed_requests: number
  total_tokens: number
  input_tokens: number
  output_tokens: number
  cached_tokens: number
  reasoning_tokens: number
  rpm: number
  tpm: number
  total_cost: number
  cost_available: boolean
}

export interface UsageMonitoringModelDistributionItem {
  model: string
  total_requests: number
  success_count: number
  failure_count: number
  total_tokens: number
  input_tokens: number
  output_tokens: number
  cached_tokens: number
  reasoning_tokens: number
  success_rate: number
}

export interface UsageMonitoringDailyTrendPoint {
  date: string
  requests: number
  tokens: number
  input_tokens: number
  output_tokens: number
  cached_tokens: number
  reasoning_tokens: number
  cost: number
}

export interface UsageMonitoringHourlyModelStat {
  model: string
  requests: number
  tokens: number
  success_count: number
  failure_count: number
}

export interface UsageMonitoringHourlyModelPoint {
  hour: string
  models: UsageMonitoringHourlyModelStat[]
}

export interface UsageMonitoringHourlyTokenPoint {
  hour: string
  input_tokens: number
  output_tokens: number
  cached_tokens: number
  reasoning_tokens: number
  total_tokens: number
  cost: number
}

export interface UsageMonitoringChannelModelStat {
  model: string
  requests: number
  success: number
  failed: number
  success_rate: number
  total_tokens: number
  last_request_time: string | null
  recent_requests: UsageMonitoringRecentRequest[]
}

export interface UsageMonitoringChannelStat {
  source: string
  source_type?: string
  source_key?: string
  total_requests: number
  success_requests: number
  failed_requests: number
  total_tokens: number
  input_tokens: number
  output_tokens: number
  cached_tokens: number
  reasoning_tokens: number
  success_rate: number
  last_request_time: string | null
  recent_requests: UsageMonitoringRecentRequest[]
  models: UsageMonitoringChannelModelStat[]
}

export interface UsageMonitoringFailureModelStat {
  model: string
  success: number
  failure: number
  total: number
  success_rate: number
  last_timestamp: string | null
  recent_requests: UsageMonitoringRecentRequest[]
}

export interface UsageMonitoringFailureStat {
  source: string
  source_type?: string
  source_key?: string
  failed_count: number
  last_fail_time: string | null
  models: UsageMonitoringFailureModelStat[]
}

export interface UsageMonitoringRequestLog {
  id?: number
  timestamp: string
  model: string
  reasoning_effort?: string
  source: string
  source_type?: string
  source_key?: string
  failed: boolean
  latency_ms: number
  tokens: UsageEventTokens
}

export interface UsageMonitoringResponse {
  kpis: UsageMonitoringKPI
  model_distribution: UsageMonitoringModelDistributionItem[]
  daily_trend: UsageMonitoringDailyTrendPoint[]
  hourly_model_trend: UsageMonitoringHourlyModelPoint[]
  hourly_token_trend: UsageMonitoringHourlyTokenPoint[]
  channel_stats: UsageMonitoringChannelStat[]
  failure_analysis: UsageMonitoringFailureStat[]
  request_logs: UsageMonitoringRequestLog[]
  timezone: string
  range_start?: string
  range_end?: string
}
