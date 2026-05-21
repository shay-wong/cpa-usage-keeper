import { useTranslation } from 'react-i18next';

import { MonitoringCenterTab, useMonitoringCenterData } from '@/components/usage';
import type { ModelPrice, UsageTimeRange } from '@/utils/usage';

type MonitoringQueryState = {
  start?: string;
  end?: string;
};

type DevMonitoringData = ReturnType<typeof useMonitoringCenterData>;

type UseDevMonitoringCenterDataOptions = {
  range: UsageTimeRange;
  queryState: MonitoringQueryState;
  filterError?: string;
  enabled: boolean;
  onAuthRequired?: () => void;
};

export const useDevMonitoringCenterData = ({
  range,
  queryState,
  filterError,
  enabled,
  onAuthRequired,
}: UseDevMonitoringCenterDataOptions): DevMonitoringData => useMonitoringCenterData({
  range,
  start: queryState.start,
  end: queryState.end,
  filterError,
  enabled,
  onAuthRequired,
});

type DevMonitoringCenterTabProps = Pick<DevMonitoringData, 'data' | 'loading' | 'error'> & {
  lastUpdatedAt: Date | null;
  modelPrices: Record<string, ModelPrice>;
  query?: string;
};

export function DevMonitoringCenterTab({ data, loading, error, lastUpdatedAt, modelPrices, query = '' }: DevMonitoringCenterTabProps) {
  const { t } = useTranslation();
  return (
    <MonitoringCenterTab
      data={data}
      loading={loading}
      error={error === 'AUTH_REQUIRED' ? t('auth.session_expired') : error}
      lastUpdatedAt={lastUpdatedAt}
      modelPrices={modelPrices}
      query={query}
    />
  );
}
