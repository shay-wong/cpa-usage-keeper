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

export interface FetchUsageMonitoringOptions {
  apiKeyId?: string
  query?: string
  model?: string
  source?: string
  result?: string
  logLimit?: number
}

export async function fetchUsageMonitoring(
  range: UsageTimeRange,
  start?: string,
  end?: string,
  signal?: AbortSignal,
  options?: FetchUsageMonitoringOptions,
): Promise<UsageMonitoringResponse> {
  const params = new URLSearchParams()
  params.set('range', range)
  if (start) {
    params.set('start', start)
  }
  if (end) {
    params.set('end', end)
  }
  const selectedApiKeyId = options?.apiKeyId?.trim()
  if (selectedApiKeyId) {
    params.set('api_key_id', selectedApiKeyId)
  }
  const queryText = options?.query?.trim()
  if (queryText) {
    params.set('query', queryText)
  }
  const model = options?.model?.trim()
  if (model) {
    params.set('model', model)
  }
  const source = options?.source?.trim()
  if (source) {
    params.set('source', source)
  }
  const result = options?.result?.trim()
  if (result) {
    params.set('result', result)
  }
  if (typeof options?.logLimit === 'number' && Number.isFinite(options.logLimit) && options.logLimit > 0) {
    params.set('log_limit', String(Math.floor(options.logLimit)))
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
