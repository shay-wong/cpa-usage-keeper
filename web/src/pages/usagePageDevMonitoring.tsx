import { useTranslation } from 'react-i18next';

import { MonitoringCenterTab, useMonitoringCenterData } from '@/components/usage';
import type { UsageTimeRange } from '@/utils/usage';

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
  apiKeyId?: string;
  query?: string;
};

export const useDevMonitoringCenterData = ({
  range,
  queryState,
  filterError,
  enabled,
  onAuthRequired,
  apiKeyId,
  query,
}: UseDevMonitoringCenterDataOptions): DevMonitoringData => useMonitoringCenterData({
  range,
  start: queryState.start,
  end: queryState.end,
  filterError,
  enabled,
  onAuthRequired,
  apiKeyId,
  query,
});

type DevMonitoringCenterTabProps = Pick<DevMonitoringData, 'data' | 'loading' | 'error'> & {
  lastUpdatedAt: Date | null;
  query?: string;
};

export function DevMonitoringCenterTab({ data, loading, error, lastUpdatedAt, query = '' }: DevMonitoringCenterTabProps) {
  const { t } = useTranslation();
  return (
    <MonitoringCenterTab
      data={data}
      loading={loading}
      error={error === 'AUTH_REQUIRED' ? t('auth.session_expired') : error}
      lastUpdatedAt={lastUpdatedAt}
      query={query}
    />
  );
}
