import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError } from '@/lib/api';
import { fetchUsageMonitoring } from '@/lib/usageMonitoringApi';
import type { MonitoringCenterQuery, MonitoringCenterViewModel } from './types';

interface UseMonitoringCenterDataOptions extends MonitoringCenterQuery {
  enabled: boolean;
  onAuthRequired?: () => void;
}

interface MonitoringCenterState {
  data: MonitoringCenterViewModel | null;
  loading: boolean;
  error: string;
  refresh: () => Promise<void>;
}

export function useMonitoringCenterData({ range, start, end, filterError, enabled, onAuthRequired }: UseMonitoringCenterDataOptions): MonitoringCenterState {
  const [data, setData] = useState<MonitoringCenterViewModel | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const controllerRef = useRef<AbortController | null>(null);

  const refresh = useCallback(async () => {
    if (!enabled) return;
    if (filterError || (range === 'custom' && (!start || !end))) {
      controllerRef.current?.abort();
      controllerRef.current = null;
      setData(null);
      setError(filterError ?? '');
      setLoading(false);
      return;
    }

    controllerRef.current?.abort();
    const controller = new AbortController();
    controllerRef.current = controller;

    setLoading(true);
    setError('');
    try {
      const response = await fetchUsageMonitoring(range, start, end, controller.signal);
      if (controllerRef.current !== controller) return;
      setData(response);
    } catch (err: unknown) {
      if (controller.signal.aborted) return;
      if (controllerRef.current === controller) {
        setData(null);
      }
      if (err instanceof ApiError && err.status === 401) {
        onAuthRequired?.();
        return;
      }
      setError(err instanceof Error ? err.message : 'Failed to load usage monitoring');
    } finally {
      if (controllerRef.current === controller) {
        setLoading(false);
        controllerRef.current = null;
      }
    }
  }, [enabled, end, filterError, onAuthRequired, range, start]);

  useEffect(() => {
    if (!enabled) {
      controllerRef.current?.abort();
      controllerRef.current = null;
      setLoading(false);
      return;
    }
    void refresh();
    return () => {
      controllerRef.current?.abort();
      controllerRef.current = null;
    };
  }, [enabled, refresh]);

  return { data, loading, error, refresh };
}
