import type { UsageTimeRange } from '@/lib/types';
import type { UsageMonitoringResponse } from '@/lib/usageMonitoringTypes';

export type MonitoringCenterViewModel = UsageMonitoringResponse;

export interface MonitoringCenterQuery {
  range: UsageTimeRange;
  start?: string;
  end?: string;
  filterError?: string;
}
