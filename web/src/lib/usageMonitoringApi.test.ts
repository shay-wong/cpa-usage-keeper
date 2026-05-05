import { afterEach, describe, expect, it, vi } from 'vitest';
import { fetchUsageMonitoring } from './usageMonitoringApi';

describe('fetchUsageMonitoring', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('loads usage monitoring with range and log limit query params', async () => {
    vi.stubGlobal('window', { __APP_BASE_PATH__: undefined });
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      json: async () => ({
        kpis: { total_requests: 0 },
        model_distribution: [],
        daily_trend: [],
        hourly_model_trend: [],
        hourly_token_trend: [],
        channel_stats: [],
        failure_analysis: [],
        request_logs: [],
        timezone: 'UTC',
      }),
    } as Response);
    const signal = new AbortController().signal;

    await fetchUsageMonitoring('custom', '2026-04-20T00:00:00Z', '2026-04-21T00:00:00Z', signal, 250);

    const [url, init] = fetchMock.mock.calls[0];
    const parsed = new URL(String(url), 'http://localhost');

    expect(parsed.pathname).toBe('/api/v1/usage/monitoring');
    expect(parsed.searchParams.get('range')).toBe('custom');
    expect(parsed.searchParams.get('start')).toBe('2026-04-20T00:00:00Z');
    expect(parsed.searchParams.get('end')).toBe('2026-04-21T00:00:00Z');
    expect(parsed.searchParams.get('log_limit')).toBe('250');
    expect(init).toMatchObject({ credentials: 'include', signal });
  });
});
