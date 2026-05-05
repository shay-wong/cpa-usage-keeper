import { ApiError, apiPath } from './api'
import type { UsageTimeRange } from './types'
import type { UsageMonitoringResponse } from './usageMonitoringTypes'

async function parseUsageMonitoringError(response: Response, fallback: string): Promise<never> {
  let message = fallback
  try {
    const payload = await response.json() as { error?: string }
    if (payload.error) {
      message = payload.error
    }
  } catch {
    message = fallback
  }
  throw new ApiError(message, response.status)
}

export async function fetchUsageMonitoring(range: UsageTimeRange, start?: string, end?: string, signal?: AbortSignal, logLimit?: number): Promise<UsageMonitoringResponse> {
  const params = new URLSearchParams()
  params.set('range', range)
  if (start) {
    params.set('start', start)
  }
  if (end) {
    params.set('end', end)
  }
  if (typeof logLimit === 'number' && Number.isFinite(logLimit) && logLimit > 0) {
    params.set('log_limit', String(Math.floor(logLimit)))
  }
  const query = params.toString()
  const response = await fetch(`${apiPath('/usage/monitoring')}${query ? `?${query}` : ''}`, {
    credentials: 'include',
    signal,
  })
  if (!response.ok) {
    await parseUsageMonitoringError(response, `Failed to load usage monitoring: ${response.status}`)
  }
  return response.json()
}
