import { useId, useMemo, useState } from 'react';
import {
  ArcElement,
  BarController,
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  Filler,
  Legend,
  LineController,
  LineElement,
  LinearScale,
  PointElement,
  Title,
  Tooltip,
  type TooltipItem,
} from 'chart.js';
import { Chart, Doughnut } from 'react-chartjs-2';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/Button';
import { EmptyState } from '@/components/ui/EmptyState';
import { LoadingSpinner } from '@/components/ui/LoadingSpinner';
import type {
  UsageMonitoringModelDistributionItem,
  UsageMonitoringResponse,
} from '@/lib/usageMonitoringTypes';
import { useThemeStore } from '@/stores';
import { formatCompactNumber, formatDurationMs, formatPerMinuteValue, formatUsd } from '@/utils/usage';
import styles from './MonitoringCenterTab.module.scss';

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  BarController,
  LineController,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler,
);

interface MonitoringCenterTabProps {
  data: UsageMonitoringResponse | null;
  loading: boolean;
  error?: string;
  lastUpdatedAt?: Date | null;
}

type Translate = (key: string, options?: Record<string, unknown>) => string;
type DistributionMetric = 'requests' | 'tokens';
type HourlyWindowMode = '6h' | '12h' | '24h' | 'custom';

const DONUT_COLORS = ['#2563eb', '#9333ea', '#0891b2', '#f97316', '#16a34a', '#ef4444', '#db2777', '#eab308'];
const HOURLY_MODEL_COLORS = ['#2563eb', '#9333ea', '#0891b2', '#16a34a', '#f97316'];
const HOURLY_WINDOW_OPTIONS: ReadonlyArray<{ value: HourlyWindowMode; labelKey: string }> = [
  { value: '6h', labelKey: 'usage_stats.monitoring_hourly_6h' },
  { value: '12h', labelKey: 'usage_stats.monitoring_hourly_12h' },
  { value: '24h', labelKey: 'usage_stats.monitoring_hourly_24h' },
  { value: 'custom', labelKey: 'usage_stats.monitoring_hourly_custom' },
];

const CHART_GRID_COLOR = 'rgba(148, 163, 184, 0.14)';
const CHART_GRID_COLOR_STRONG = 'rgba(148, 163, 184, 0.18)';

function getChartThemeColors(isDark: boolean) {
  const rootStyles = window.getComputedStyle(document.documentElement);
  const textPrimary = rootStyles.getPropertyValue('--text-primary').trim() || (isDark ? '#f6f4f1' : '#2d2a26');
  const textSecondary = rootStyles.getPropertyValue('--text-secondary').trim() || (isDark ? '#c9c3bb' : '#6d6760');
  const textTertiary = rootStyles.getPropertyValue('--text-tertiary').trim() || (isDark ? '#9c958d' : '#a29c95');
  const backgroundPrimary = rootStyles.getPropertyValue('--bg-primary').trim() || (isDark ? '#1d1b18' : '#ffffff');
  const borderColor = rootStyles.getPropertyValue('--border-color').trim() || (isDark ? '#3a3530' : '#e5e7eb');

  return {
    textPrimary,
    textSecondary,
    textTertiary,
    legendText: isDark ? textPrimary : textSecondary,
    backgroundPrimary,
    borderColor,
    tooltipBackground: isDark ? 'rgba(29, 27, 24, 0.96)' : 'rgba(255, 255, 255, 0.98)',
    tooltipTitle: textPrimary,
    tooltipBody: textSecondary,
    tooltipBorder: isDark ? 'rgba(246, 244, 241, 0.14)' : 'rgba(17, 24, 39, 0.10)',
  };
}

function normalizeQuery(value: string): string {
  return value.trim().toLowerCase();
}

function includesQuery(values: Array<string | null | undefined>, query: string): boolean {
  if (!query) return true;
  return values.some((value) => value?.toLowerCase().includes(query));
}

function formatRate(value: number): string {
  return `${Math.max(0, Math.min(100, value || 0)).toFixed(1)}%`;
}

function formatFullNumber(value: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(value || 0);
}

function getRateClass(rate: number): string {
  if (rate >= 97) return styles.rateHigh;
  if (rate >= 85) return styles.rateMedium;
  return styles.rateLow;
}

function formatDateTime(value: string | null | undefined, locale: string, timeZone?: string): string {
  if (!value) return '-';
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return value;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: 'short',
    timeStyle: 'short',
    timeZone: timeZone || undefined,
  }).format(new Date(timestamp));
}

function formatDateLabel(value: string | null | undefined, locale: string, timeZone?: string): string {
  if (!value) return '-';
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return value;
  return new Intl.DateTimeFormat(locale, {
    month: 'short',
    day: 'numeric',
    timeZone: timeZone || undefined,
  }).format(new Date(timestamp));
}

function formatHourLabel(value: string, locale: string, timeZone?: string): string {
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return value;
  return new Intl.DateTimeFormat(locale, {
    month: 'numeric',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    timeZone: timeZone || undefined,
  }).format(new Date(timestamp));
}

function formatTrendDateLabel(value: string): string {
  const [, month, day] = value.split('-');
  if (!month || !day) return value;
  return `${Number(month)}/${Number(day)}`;
}

function toDateInputValue(value: string | null | undefined, timeZone?: string): string {
  if (!value) return '';
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return '';
  const date = new Date(timestamp);
  if (!timeZone) return date.toISOString().slice(0, 10);
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(date);
  const year = parts.find((part) => part.type === 'year')?.value;
  const month = parts.find((part) => part.type === 'month')?.value;
  const day = parts.find((part) => part.type === 'day')?.value;
  return year && month && day ? `${year}-${month}-${day}` : '';
}

function getTodayDateInputValue(timeZone?: string): string {
  const todayIso = new Date().toISOString();
  return toDateInputValue(todayIso, timeZone) || todayIso.slice(0, 10);
}

function getDayKey(value: string, timeZone?: string): string {
  return toDateInputValue(value, timeZone) || value.slice(0, 10);
}

function getWindowStartMs(points: Array<{ hour: string }>, mode: HourlyWindowMode): number {
  const latest = points.reduce((current, point) => {
    const timestamp = Date.parse(point.hour);
    return Number.isFinite(timestamp) ? Math.max(current, timestamp) : current;
  }, 0);
  const hours = mode === '6h' ? 6 : mode === '12h' ? 12 : 24;
  return latest > 0 ? latest - (hours - 1) * 60 * 60 * 1000 : 0;
}

function filterHourlyPoints<T extends { hour: string }>(points: T[], mode: HourlyWindowMode, day: string, timeZone?: string): T[] {
  if (mode === 'custom') {
    return day ? points.filter((point) => getDayKey(point.hour, timeZone) === day) : points.slice(-24);
  }
  const windowStartMs = getWindowStartMs(points, mode);
  if (!windowStartMs) return points.slice(-24);
  return points.filter((point) => {
    const timestamp = Date.parse(point.hour);
    return Number.isFinite(timestamp) && timestamp >= windowStartMs;
  });
}

function summarizeModelDistribution(items: UsageMonitoringModelDistributionItem[], t: Translate): string {
  if (!items.length) {
    return t('usage_stats.monitoring_model_distribution_empty');
  }
  const topModel = items[0];
  return t('usage_stats.monitoring_model_distribution_summary', {
    model: topModel.model,
    requests: formatCompactNumber(topModel.total_requests),
    successRate: formatRate(topModel.success_rate),
  });
}

function summarizeDailyTrend(items: UsageMonitoringResponse['daily_trend'], t: Translate): string {
  if (!items.length) {
    return t('usage_stats.monitoring_daily_trend_empty');
  }
  const first = items[0];
  const last = items[items.length - 1];
  const peak = items.reduce((currentPeak, item) => (item.requests > currentPeak.requests ? item : currentPeak), first);
  return t('usage_stats.monitoring_daily_trend_summary', {
    count: items.length,
    start: first.date,
    end: last.date,
    peakDate: peak.date,
    peakRequests: formatCompactNumber(peak.requests),
    peakTokens: formatCompactNumber(peak.tokens),
  });
}

function RequestDots({
  requests,
  t,
  locale,
  timeZone,
}: {
  requests: Array<{ timestamp: string; failed: boolean }>;
  t: Translate;
  locale: string;
  timeZone?: string;
}) {
  if (!requests.length) {
    return <span className={styles.muted}>-</span>;
  }

  return (
    <ol className={styles.statusBars} aria-label={t('usage_stats.monitoring_recent_requests_count', { count: requests.length })}>
      {requests.map((request, index) => (
        <li key={`${request.timestamp}-${index}`}>
          <span className={`${styles.statusBar} ${request.failed ? styles.statusFailed : styles.statusSuccess}`.trim()} aria-hidden="true" />
          <span className={styles.srOnly}>
            {t('usage_stats.monitoring_recent_request_item', {
              time: formatDateTime(request.timestamp, locale, timeZone),
              result: request.failed ? t('usage_stats.failure') : t('usage_stats.success'),
            })}
          </span>
        </li>
      ))}
    </ol>
  );
}

function EmptyInline({ message }: { message: string }) {
  return <div className={styles.emptyInline}>{message}</div>;
}

export function MonitoringCenterTab({
  data,
  loading,
  error,
  lastUpdatedAt,
}: MonitoringCenterTabProps) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? 'zh-CN' : 'en-US';
  const resolvedTheme = useThemeStore((state) => state.resolvedTheme);
  const isDark = resolvedTheme === 'dark';
  const chartThemeColors = useMemo(() => getChartThemeColors(isDark), [isDark]);
  const [queryInput, setQueryInput] = useState('');
  const [appliedQuery, setAppliedQuery] = useState('');
  const [distributionMetric, setDistributionMetric] = useState<DistributionMetric>('requests');
  const [hourlyModelWindowMode, setHourlyModelWindowMode] = useState<HourlyWindowMode>('24h');
  const [hourlyModelDay, setHourlyModelDay] = useState(() => getTodayDateInputValue());
  const [hourlyTokenWindowMode, setHourlyTokenWindowMode] = useState<HourlyWindowMode>('24h');
  const [hourlyTokenDay, setHourlyTokenDay] = useState(() => getTodayDateInputValue());
  const timeZone = data?.timezone;
  const modelDistributionTitleId = useId();
  const modelDistributionSummaryId = useId();
  const dailyTrendTitleId = useId();
  const dailyTrendSummaryId = useId();
  const kpis = data?.kpis;
  const hasData = Boolean(kpis && kpis.total_requests > 0);
  const hasError = Boolean(error);
  const successRate = kpis && kpis.total_requests > 0 ? (kpis.success_requests / kpis.total_requests) * 100 : 0;
  const rangeStartMs = data?.range_start ? Date.parse(data.range_start) : NaN;
  const rangeEndMs = data?.range_end ? Date.parse(data.range_end) : NaN;
  const rpdDays = Number.isFinite(rangeStartMs) && Number.isFinite(rangeEndMs) && rangeEndMs >= rangeStartMs
    ? Math.max(1, Math.ceil((rangeEndMs - rangeStartMs) / 86_400_000))
    : Math.max(1, data?.daily_trend.length ?? 1);
  const averageRpd = (kpis?.total_requests ?? 0) / rpdDays;
  const normalizedQuery = normalizeQuery(appliedQuery);

  const modelDistribution = useMemo(() => {
    const items = data?.model_distribution ?? [];
    return normalizedQuery ? items.filter((item) => includesQuery([item.model], normalizedQuery)) : items;
  }, [data?.model_distribution, normalizedQuery]);

  const channelStats = useMemo(() => {
    const items = data?.channel_stats ?? [];
    return normalizedQuery
      ? items.filter((item) =>
          includesQuery(
            [item.source, item.source_type, item.source_key, ...item.models.map((model) => model.model)],
            normalizedQuery,
          ),
        )
      : items;
  }, [data?.channel_stats, normalizedQuery]);

  const failureAnalysis = useMemo(() => {
    const items = data?.failure_analysis ?? [];
    return normalizedQuery
      ? items.filter((item) =>
          includesQuery(
            [item.source, item.source_type, item.source_key, ...item.models.map((model) => model.model)],
            normalizedQuery,
          ),
        )
      : items;
  }, [data?.failure_analysis, normalizedQuery]);

  const requestLogs = useMemo(() => {
    const items = data?.request_logs ?? [];
    return normalizedQuery
      ? items.filter((item) => includesQuery([item.source, item.source_type, item.source_key, item.model], normalizedQuery))
      : items;
  }, [data?.request_logs, normalizedQuery]);

  const hourlyModelTrend = useMemo(() => {
    const items = data?.hourly_model_trend ?? [];
    const filteredByQuery = normalizedQuery
      ? items
          .map((point) => ({
            ...point,
            models: point.models.filter((model) => includesQuery([model.model], normalizedQuery)),
          }))
          .filter((point) => point.models.length > 0)
      : items;
    const fallbackDay = hourlyModelDay || toDateInputValue(filteredByQuery[filteredByQuery.length - 1]?.hour, timeZone);
    return filterHourlyPoints(filteredByQuery, hourlyModelWindowMode, fallbackDay, timeZone);
  }, [data?.hourly_model_trend, hourlyModelDay, hourlyModelWindowMode, normalizedQuery, timeZone]);

  const applySearch = () => {
    setAppliedQuery(queryInput);
  };

  const dailyTrend = data?.daily_trend ?? [];
  const dailyTrendSummary = summarizeDailyTrend(dailyTrend, t);
  const dailyTrendData = {
    labels: dailyTrend.map((point) => formatTrendDateLabel(point.date)),
    datasets: [
      {
        type: 'line' as const,
        label: t('usage_stats.requests_count'),
        data: dailyTrend.map((point) => point.requests),
        borderColor: '#3b82f6',
        backgroundColor: '#3b82f6',
        borderWidth: 3,
        fill: false,
        tension: 0.35,
        yAxisID: 'y1',
        order: 0,
        pointRadius: 3,
        pointBackgroundColor: '#3b82f6',
      },
      {
        type: 'bar' as const,
        label: t('usage_stats.input_tokens'),
        data: dailyTrend.map((point) => point.input_tokens / 1000),
        backgroundColor: 'rgba(34, 197, 94, 0.7)',
        borderColor: 'rgba(34, 197, 94, 0.7)',
        borderWidth: 1,
        borderRadius: 0,
        yAxisID: 'y',
        order: 1,
        stack: 'tokens',
      },
      {
        type: 'bar' as const,
        label: t('usage_stats.output_tokens'),
        data: dailyTrend.map((point) => point.output_tokens / 1000),
        backgroundColor: 'rgba(249, 115, 22, 0.7)',
        borderColor: 'rgba(249, 115, 22, 0.7)',
        borderWidth: 1,
        borderRadius: 4,
        yAxisID: 'y',
        order: 1,
        stack: 'tokens',
      },
      {
        type: 'bar' as const,
        label: t('usage_stats.reasoning_tokens'),
        data: dailyTrend.map((point) => point.reasoning_tokens / 1000),
        backgroundColor: 'rgba(168, 85, 247, 0.7)',
        borderColor: 'rgba(168, 85, 247, 0.7)',
        borderWidth: 1,
        borderRadius: 4,
        yAxisID: 'y',
        order: 1,
        stack: 'tokens',
      },
      {
        type: 'bar' as const,
        label: t('usage_stats.cached_tokens'),
        data: dailyTrend.map((point) => point.cached_tokens / 1000),
        backgroundColor: 'rgba(14, 165, 233, 0.7)',
        borderColor: 'rgba(14, 165, 233, 0.7)',
        borderWidth: 1,
        borderRadius: 4,
        yAxisID: 'y',
        order: 1,
        stack: 'tokens',
      },
    ],
  };

  const modelDistributionSummary = summarizeModelDistribution(modelDistribution, t);
  const modelDistributionSlice = modelDistribution.slice(0, 8);
  const modelDistributionTotal = modelDistributionSlice.reduce(
    (sum, item) => sum + (distributionMetric === 'requests' ? item.total_requests : item.total_tokens),
    0,
  );
  const modelDistributionData = {
    labels: modelDistributionSlice.map((item) => item.model),
    datasets: [
      {
        data: modelDistributionSlice.map((item) => distributionMetric === 'requests' ? item.total_requests : item.total_tokens),
        backgroundColor: DONUT_COLORS.slice(0, modelDistributionSlice.length),
        borderColor: 'rgba(255, 255, 255, 0.9)',
        borderWidth: 2,
        hoverOffset: 6,
      },
    ],
  };

  const topHourlyModels = useMemo(() => {
    const totals = new Map<string, number>();
    hourlyModelTrend.forEach((point) => {
      point.models.forEach((model) => {
        totals.set(model.model, (totals.get(model.model) ?? 0) + model.requests);
      });
    });
    return [...totals.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, 5)
      .map(([model]) => model);
  }, [hourlyModelTrend]);

  const hourlyModelData = {
    labels: hourlyModelTrend.map((point) => formatHourLabel(point.hour, locale, timeZone)),
    datasets: [
      ...topHourlyModels.map((model, index) => ({
        type: 'bar' as const,
        label: model,
        data: hourlyModelTrend.map((point) => point.models.find((item) => item.model === model)?.requests ?? 0),
        backgroundColor: HOURLY_MODEL_COLORS[index % HOURLY_MODEL_COLORS.length],
        borderRadius: 6,
        stack: 'models',
        yAxisID: 'y',
        order: 1,
      })),
      {
        type: 'line' as const,
        label: t('usage_stats.requests_count'),
        data: hourlyModelTrend.map((point) => point.models.reduce((sum, model) => sum + model.requests, 0)),
        borderColor: '#e11d48',
        backgroundColor: '#e11d48',
        borderWidth: 3,
        tension: 0.35,
        pointRadius: 3,
        yAxisID: 'y1',
        order: 0,
      },
    ],
  };

  const hourlyTokenTrend = useMemo(() => {
    const items = data?.hourly_token_trend ?? [];
    const fallbackDay = hourlyTokenDay || toDateInputValue(items[items.length - 1]?.hour, timeZone);
    return filterHourlyPoints(items, hourlyTokenWindowMode, fallbackDay, timeZone);
  }, [data?.hourly_token_trend, hourlyTokenDay, hourlyTokenWindowMode, timeZone]);
  const hourlyTokenData = {
    labels: hourlyTokenTrend.map((point) => formatHourLabel(point.hour, locale, timeZone)),
    datasets: [
      {
        type: 'bar' as const,
        label: t('usage_stats.monitoring_total_tokens'),
        data: hourlyTokenTrend.map((point) => point.total_tokens),
        backgroundColor: 'rgba(59, 130, 246, 0.24)',
        borderColor: 'rgba(59, 130, 246, 0.45)',
        borderWidth: 1,
        borderRadius: 6,
        yAxisID: 'y',
        order: 1,
      },
      {
        type: 'line' as const,
        label: t('usage_stats.input_tokens'),
        data: hourlyTokenTrend.map((point) => point.input_tokens),
        borderColor: '#ec4899',
        backgroundColor: '#ec4899',
        borderWidth: 2,
        tension: 0.35,
        pointRadius: 2,
        yAxisID: 'y',
        order: 0,
      },
      {
        type: 'line' as const,
        label: t('usage_stats.output_tokens'),
        data: hourlyTokenTrend.map((point) => point.output_tokens),
        borderColor: '#3b82f6',
        backgroundColor: '#3b82f6',
        borderWidth: 2,
        tension: 0.35,
        pointRadius: 2,
        yAxisID: 'y',
        order: 0,
      },
      {
        type: 'line' as const,
        label: t('usage_stats.cached_tokens'),
        data: hourlyTokenTrend.map((point) => point.cached_tokens),
        borderColor: '#10b981',
        backgroundColor: '#10b981',
        borderWidth: 2,
        tension: 0.35,
        pointRadius: 2,
        yAxisID: 'y',
        order: 0,
      },
      {
        type: 'line' as const,
        label: t('usage_stats.reasoning_tokens'),
        data: hourlyTokenTrend.map((point) => point.reasoning_tokens),
        borderColor: '#f97316',
        backgroundColor: '#f97316',
        borderWidth: 2,
        tension: 0.35,
        pointRadius: 2,
        yAxisID: 'y',
        order: 0,
      },
    ],
  };

  const lineOptions = {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    plugins: {
      legend: {
        display: true,
        position: 'bottom' as const,
        labels: {
          color: chartThemeColors.legendText,
          usePointStyle: true,
          padding: 16,
          font: { size: 11 },
          generateLabels: (chart: ChartJS) =>
            chart.data.datasets.map((dataset, index) => {
              const isLine = dataset.type === 'line';
              return {
                text: dataset.label ?? '',
                fillStyle: dataset.backgroundColor as string,
                fontColor: chartThemeColors.legendText,
                strokeStyle: dataset.borderColor as string,
                lineWidth: 0,
                hidden: !chart.isDatasetVisible(index),
                datasetIndex: index,
                pointStyle: isLine ? ('circle' as const) : ('rect' as const),
              };
            }),
        },
      },
      tooltip: {
        backgroundColor: chartThemeColors.tooltipBackground,
        titleColor: chartThemeColors.tooltipTitle,
        bodyColor: chartThemeColors.tooltipBody,
        borderColor: chartThemeColors.tooltipBorder,
        borderWidth: 1,
        padding: 12,
        callbacks: {
          label: (context: TooltipItem<'bar' | 'line'>) => {
            const label = context.dataset.label ?? '';
            const value = typeof context.raw === 'number' ? context.raw : Number(context.raw ?? 0);
            if (context.dataset.yAxisID === 'y1') {
              return `${label}: ${value.toLocaleString()}`;
            }
            return `${label}: ${value.toFixed(1)}K`;
          },
        },
      },
    },
    scales: {
      x: {
        grid: { color: CHART_GRID_COLOR },
        ticks: { color: chartThemeColors.textTertiary, font: { size: 11 } },
      },
      y: {
        type: 'linear' as const,
        position: 'left' as const,
        stacked: true,
        grid: { color: CHART_GRID_COLOR },
        ticks: {
          color: chartThemeColors.textTertiary,
          font: { size: 11 },
          callback: (value: string | number) => `${value}K`,
        },
        title: {
          display: true,
          text: 'Tokens (K)',
          color: chartThemeColors.textTertiary,
          font: { size: 11 },
        },
      },
      y1: {
        type: 'linear' as const,
        position: 'right' as const,
        grid: { drawOnChartArea: false },
        ticks: { color: chartThemeColors.textTertiary, font: { size: 11 } },
        title: {
          display: true,
          text: t('usage_stats.requests_count'),
          color: chartThemeColors.textTertiary,
          font: { size: 11 },
        },
      },
    },
  };

  const doughnutOptions = {
    responsive: true,
    maintainAspectRatio: false,
    cutout: '68%',
    plugins: {
      legend: { display: false },
    },
  };

  const hourlyMixedOptions = {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    plugins: {
      legend: {
        position: 'bottom' as const,
        labels: {
          color: chartThemeColors.legendText,
          usePointStyle: true,
          padding: 16,
          font: { size: 11 },
        },
      },
      tooltip: {
        backgroundColor: chartThemeColors.tooltipBackground,
        titleColor: chartThemeColors.tooltipTitle,
        bodyColor: chartThemeColors.tooltipBody,
        borderColor: chartThemeColors.tooltipBorder,
        borderWidth: 1,
        padding: 12,
      },
    },
    scales: {
      x: {
        stacked: true,
        grid: { display: false },
        ticks: { color: chartThemeColors.textTertiary, font: { size: 11 } },
      },
      y: {
        stacked: false,
        beginAtZero: true,
        grid: { color: CHART_GRID_COLOR_STRONG },
        ticks: { color: chartThemeColors.textTertiary, font: { size: 11 } },
      },
      y1: {
        type: 'linear' as const,
        position: 'right' as const,
        beginAtZero: true,
        grid: { drawOnChartArea: false },
        ticks: { color: chartThemeColors.textTertiary, font: { size: 11 } },
      },
    },
  };


  const activeRangeSummary = data?.range_start && data?.range_end
    ? t('usage_stats.monitoring_range_summary', {
        start: formatDateLabel(data.range_start, locale, timeZone),
        end: formatDateLabel(data.range_end, locale, timeZone),
        timezone: data.timezone,
      })
    : null;
  const activeModelCount = modelDistribution.length;
  const averageCostPerSuccess = (kpis?.cost_available && (kpis?.success_requests ?? 0) > 0)
    ? (kpis?.total_cost ?? 0) / (kpis?.success_requests ?? 0)
    : null;

  const renderHourlyControls = ({
    windowMode,
    day,
    onWindowModeChange,
    onDayChange,
  }: {
    windowMode: HourlyWindowMode;
    day: string;
    onWindowModeChange: (mode: HourlyWindowMode) => void;
    onDayChange: (day: string) => void;
  }) => (
    <div className={styles.hourlyControls}>
      <div className={styles.segmentedControl} role="group" aria-label={t('usage_stats.monitoring_hourly_window')}>
        {HOURLY_WINDOW_OPTIONS.map((option) => (
          <button
            key={option.value}
            type="button"
            className={`${styles.segmentButton} ${windowMode === option.value ? styles.segmentButtonActive : ''}`.trim()}
            onClick={() => {
              if (option.value === 'custom' && !day) {
                onDayChange(getTodayDateInputValue(timeZone));
              }
              onWindowModeChange(option.value);
            }}
          >
            {t(option.labelKey)}
          </button>
        ))}
      </div>
      {windowMode === 'custom' && (
        <span className={styles.hourlyDayInputWrap}>
          <input
            type="date"
            className={styles.hourlyDayInput}
            value={day}
            onChange={(event) => onDayChange(event.target.value)}
            aria-label={t('usage_stats.monitoring_hourly_custom')}
          />
          <span className={styles.hourlyDayIcon} aria-hidden="true" />
        </span>
      )}
    </div>
  );

  return (
    <section className={styles.monitoringShell} aria-busy={loading}>
      <div className={styles.header}>
        <span className={styles.headerWatermark} aria-hidden="true">MONITORING</span>
        <div className={styles.headerMeta}>
          <h2 className={styles.pageTitle}>{t('usage_stats.monitoring_title')}</h2>
          <p className={styles.pageSubtitle}>{t('usage_stats.monitoring_subtitle')}</p>
          <div className={styles.headerStats}>
            {activeRangeSummary && <span>{activeRangeSummary}</span>}
            {lastUpdatedAt && (
              <span>
                {t('usage_stats.last_updated')}: {lastUpdatedAt.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' })}
              </span>
            )}
          </div>
        </div>
        <div className={styles.headerActions}>
          {normalizedQuery && (
            <span className={styles.queryBadge}>
              {t('usage_stats.monitoring_query_results', { count: channelStats.length + failureAnalysis.length + requestLogs.length })}
            </span>
          )}
        </div>
      </div>

      <div className={styles.filters}>
        <div className={styles.queryControls}>
          <label className={styles.filterLabel} htmlFor="monitoring-search-input">
            {t('usage_stats.monitoring_search_label')}
          </label>
          <input
            id="monitoring-search-input"
            type="text"
            className={styles.filterInput}
            placeholder={t('usage_stats.monitoring_search_placeholder')}
            value={queryInput}
            onChange={(event) => setQueryInput(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                applySearch();
              }
            }}
          />
          <Button variant="secondary" size="sm" onClick={applySearch}>
            {t('usage_stats.monitoring_search_apply')}
          </Button>
        </div>
      </div>

      {hasError && <div className={styles.errorBox}>{error}</div>}

      {loading && !data ? (
        <div className={styles.loadingState} aria-busy="true">
          <LoadingSpinner size={28} />
          <span>{t('common.loading')}</span>
        </div>
      ) : !hasError && !hasData ? (
        <EmptyState
          title={t('usage_stats.monitoring_empty_title')}
          description={t('usage_stats.monitoring_empty_desc')}
        />
      ) : !hasError ? (
        <>
          <div className={styles.kpiGrid}>
            <div className={styles.kpiCard}>
              <div className={styles.kpiTitleRow}>
                <span className={styles.kpiLabel}>{t('usage_stats.total_requests')}</span>
                <span className={styles.kpiTag}>{t('usage_stats.requests_count')}</span>
              </div>
              <strong className={styles.kpiValue}>{formatFullNumber(kpis?.total_requests ?? 0)}</strong>
              <span className={styles.kpiMeta}>
                <span>{t('usage_stats.success_requests')}: {formatFullNumber(kpis?.success_requests ?? 0)}</span>
                <span>{t('usage_stats.failed_requests')}: {formatFullNumber(kpis?.failed_requests ?? 0)}</span>
              </span>
            </div>

            <div className={`${styles.kpiCard} ${styles.kpiCardSuccess}`.trim()}>
              <div className={styles.kpiTitleRow}>
                <span className={styles.kpiLabel}>{t('usage_stats.success_rate')}</span>
                <span className={styles.kpiTag}>{t('usage_stats.service_health')}</span>
              </div>
              <strong className={styles.kpiValue}>{formatRate(successRate)}</strong>
              <span className={styles.kpiMeta}>
                <span>{formatFullNumber(kpis?.success_requests ?? 0)} / {formatFullNumber(kpis?.total_requests ?? 0)}</span>
              </span>
            </div>

            <div className={`${styles.kpiCard} ${styles.kpiCardPurple}`.trim()}>
              <div className={styles.kpiTitleRow}>
                <span className={styles.kpiLabel}>{t('usage_stats.total_tokens')}</span>
                <span className={styles.kpiTag}>{t('usage_stats.tokens_count')}</span>
              </div>
              <strong className={styles.kpiValue}>{formatCompactNumber(kpis?.total_tokens ?? 0)}</strong>
              <span className={styles.kpiMeta}>
                <span>{t('usage_stats.input_tokens')}: {formatCompactNumber(kpis?.input_tokens ?? 0)}</span>
                <span>{t('usage_stats.output_tokens')}: {formatCompactNumber(kpis?.output_tokens ?? 0)}</span>
              </span>
            </div>

            <div className={`${styles.kpiCard} ${styles.kpiCardCyan}`.trim()}>
              <div className={styles.kpiTitleRow}>
                <span className={styles.kpiLabel}>{t('usage_stats.total_cost')}</span>
                <span className={styles.kpiTag}>{t('usage_stats.model_price_settings')}</span>
              </div>
              <strong className={styles.kpiValue}>{formatUsd(kpis?.total_cost ?? 0)}</strong>
              <span className={styles.kpiMeta}>
                <span>{kpis?.cost_available ? t('usage_stats.monitoring_cost_available') : t('usage_stats.cost_need_price')}</span>
              </span>
            </div>

            <div className={`${styles.kpiCard} ${styles.kpiCardOrange}`.trim()}>
              <div className={styles.kpiTitleRow}>
                <span className={styles.kpiLabel}>{t('usage_stats.rpm')}</span>
                <span className={styles.kpiTag}>{t('usage_stats.range_filter')}</span>
              </div>
              <strong className={styles.kpiValue}>{formatPerMinuteValue(kpis?.rpm ?? 0)}</strong>
              <span className={styles.kpiMeta}>
                <span>{t('usage_stats.requests_count')} / min</span>
              </span>
            </div>

            <div className={`${styles.kpiCard} ${styles.kpiCardCyan}`.trim()}>
              <div className={styles.kpiTitleRow}>
                <span className={styles.kpiLabel}>{t('usage_stats.tpm')}</span>
                <span className={styles.kpiTag}>{t('usage_stats.tokens_count')}</span>
              </div>
              <strong className={styles.kpiValue}>{formatPerMinuteValue(kpis?.tpm ?? 0)}</strong>
              <span className={styles.kpiMeta}>
                <span>{t('usage_stats.cached_tokens')}: {formatCompactNumber(kpis?.cached_tokens ?? 0)}</span>
              </span>
            </div>

            <div className={`${styles.kpiCard} ${styles.kpiCardPink}`.trim()}>
              <div className={styles.kpiTitleRow}>
                <span className={styles.kpiLabel}>{t('usage_stats.monitoring_rpd')}</span>
                <span className={styles.kpiTag}>{t('usage_stats.by_day')}</span>
              </div>
              <strong className={styles.kpiValue}>{formatCompactNumber(averageRpd)}</strong>
              <span className={styles.kpiMeta}>
                <span>{t('usage_stats.monitoring_rpd_window', { count: rpdDays })}</span>
              </span>
            </div>

            <div className={`${styles.kpiCard} ${styles.kpiCardRed}`.trim()}>
              <div className={styles.kpiTitleRow}>
                <span className={styles.kpiLabel}>{t('usage_stats.monitoring_active_models')}</span>
                <span className={styles.kpiTag}>{t('usage_stats.model_name')}</span>
              </div>
              <strong className={styles.kpiValue}>{formatFullNumber(activeModelCount)}</strong>
              <span className={styles.kpiMeta}>
                <span>
                  {averageCostPerSuccess === null
                    ? t('usage_stats.cost_need_price')
                    : t('usage_stats.monitoring_avg_cost_per_success', { value: formatUsd(averageCostPerSuccess) })}
                </span>
              </span>
            </div>
          </div>

          <div className={styles.chartsGrid}>
            <article className={styles.chartCard}>
              <div className={styles.chartHeader}>
                <div>
                  <h3 id={modelDistributionTitleId} className={styles.chartTitle}>{t('usage_stats.monitoring_model_distribution')}</h3>
                  <p className={styles.chartSubtitle}>{modelDistributionSummary}</p>
                </div>
                <div className={`${styles.segmentedControl} ${styles.distributionMetricControl}`.trim()} role="group" aria-label={t('usage_stats.monitoring_distribution_metric')}>
                  <button
                    type="button"
                    className={`${styles.segmentButton} ${distributionMetric === 'requests' ? styles.segmentButtonActive : ''}`.trim()}
                    onClick={() => setDistributionMetric('requests')}
                  >
                    {t('usage_stats.requests_count')}
                  </button>
                  <button
                    type="button"
                    className={`${styles.segmentButton} ${distributionMetric === 'tokens' ? styles.segmentButtonActive : ''}`.trim()}
                    onClick={() => setDistributionMetric('tokens')}
                  >
                    {t('usage_stats.tokens_count')}
                  </button>
                </div>
              </div>

              {modelDistributionSlice.length > 0 ? (
                <div className={styles.distributionContent}>
                  <div className={styles.donutWrapper} role="img" aria-labelledby={modelDistributionTitleId} aria-describedby={modelDistributionSummaryId}>
                    <Doughnut data={modelDistributionData} options={doughnutOptions} />
                    <div className={styles.donutCenter}>
                      <div className={styles.donutLabel}>{distributionMetric === 'requests' ? t('usage_stats.requests_count') : t('usage_stats.tokens_count')}</div>
                      <div className={styles.donutValue}>{formatCompactNumber(modelDistributionTotal)}</div>
                    </div>
                    <p id={modelDistributionSummaryId} className={styles.srOnly}>{modelDistributionSummary}</p>
                  </div>
                  <div className={styles.legendList}>
                    {modelDistributionSlice.map((item, index) => {
                      const value = distributionMetric === 'requests' ? item.total_requests : item.total_tokens;
                      const percentage = modelDistributionTotal > 0 ? (value / modelDistributionTotal) * 100 : 0;
                      return (
                        <div className={styles.legendItem} key={item.model}>
                          <span className={styles.legendDot} style={{ backgroundColor: DONUT_COLORS[index % DONUT_COLORS.length] }} aria-hidden="true" />
                          <span className={styles.legendName}>{item.model}</span>
                          <span className={styles.legendValue}>{formatCompactNumber(value)}</span>
                          <span className={styles.legendPercent}>{formatRate(percentage)}</span>
                        </div>
                      );
                    })}
                  </div>
                </div>
              ) : (
                <EmptyInline message={t('usage_stats.monitoring_no_matching_data')} />
              )}
            </article>

            <article className={styles.chartCard}>
              <div className={styles.chartHeader}>
                <div>
                  <h3 id={dailyTrendTitleId} className={styles.chartTitle}>{t('usage_stats.monitoring_daily_trend')}</h3>
                  <p className={styles.chartSubtitle}>{dailyTrendSummary}</p>
                </div>
              </div>
              {dailyTrend.length > 0 ? (
                <div className={styles.chartContent} role="img" aria-labelledby={dailyTrendTitleId} aria-describedby={dailyTrendSummaryId}>
                  <Chart type="bar" data={dailyTrendData} options={lineOptions} />
                  <p id={dailyTrendSummaryId} className={styles.srOnly}>{dailyTrendSummary}</p>
                </div>
              ) : (
                <EmptyInline message={t('usage_stats.monitoring_daily_trend_empty')} />
              )}
            </article>
          </div>

          <div className={styles.hourlyGrid}>
            <article className={styles.chartCard}>
              <div className={styles.chartHeader}>
                <div>
                  <h3 className={styles.chartTitle}>{t('usage_stats.monitoring_hourly_models')}</h3>
                  <p className={styles.chartSubtitle}>{t('usage_stats.monitoring_query_results', { count: topHourlyModels.length })}</p>
                </div>
                {renderHourlyControls({
                  windowMode: hourlyModelWindowMode,
                  day: hourlyModelDay,
                  onWindowModeChange: setHourlyModelWindowMode,
                  onDayChange: setHourlyModelDay,
                })}
              </div>
              {hourlyModelTrend.length > 0 && topHourlyModels.length > 0 ? (
                <div className={styles.chartContentTall}>
                  <Chart type="bar" data={hourlyModelData} options={hourlyMixedOptions} />
                </div>
              ) : (
                <EmptyInline message={t('usage_stats.monitoring_hourly_models_empty')} />
              )}
            </article>

            <article className={styles.chartCard}>
              <div className={styles.chartHeader}>
                <div>
                  <h3 className={styles.chartTitle}>{t('usage_stats.monitoring_hourly_tokens')}</h3>
                  <p className={styles.chartSubtitle}>{t('usage_stats.monitoring_total_tokens')}</p>
                </div>
                {renderHourlyControls({
                  windowMode: hourlyTokenWindowMode,
                  day: hourlyTokenDay,
                  onWindowModeChange: setHourlyTokenWindowMode,
                  onDayChange: setHourlyTokenDay,
                })}
              </div>
              {hourlyTokenTrend.length > 0 ? (
                <div className={styles.chartContentTall}>
                  <Chart type="bar" data={hourlyTokenData} options={hourlyMixedOptions} />
                </div>
              ) : (
                <EmptyInline message={t('usage_stats.monitoring_hourly_tokens_empty')} />
              )}
            </article>
          </div>

          <article className={styles.chartCard}>
            <div className={styles.chartHeader}>
              <div>
                <h3 className={styles.chartTitle}>{t('usage_stats.monitoring_channels')}</h3>
                <p className={styles.chartSubtitle}>{t('usage_stats.monitoring_recent_requests')}</p>
              </div>
            </div>

            {channelStats.length > 0 ? (
              <div className={styles.tableWrapper}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>{t('usage_stats.source_name')}</th>
                      <th>{t('usage_stats.monitoring_table_models')}</th>
                      <th>{t('usage_stats.requests_count')}</th>
                      <th>{t('usage_stats.success_rate')}</th>
                      <th>{t('usage_stats.total_tokens')}</th>
                      <th>{t('usage_stats.last_request')}</th>
                      <th>{t('usage_stats.monitoring_table_recent_status')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {channelStats.map((channel) => (
                      <tr key={channel.source_key || channel.source}>
                        <td>
                          <div className={styles.cellTitle}>{channel.source}</div>
                          {channel.source_type && <div className={styles.cellMeta}>{channel.source_type}</div>}
                        </td>
                        <td>
                          <div className={styles.modelTagList}>
                            {channel.models.slice(0, 4).map((model) => (
                              <span className={styles.modelTag} key={`${channel.source}-${model.model}`}>
                                {model.model}
                              </span>
                            ))}
                            {channel.models.length > 4 && <span className={styles.modelTag}>+{channel.models.length - 4}</span>}
                          </div>
                        </td>
                        <td>{formatCompactNumber(channel.total_requests)}</td>
                        <td className={getRateClass(channel.success_rate)}>{formatRate(channel.success_rate)}</td>
                        <td>{formatCompactNumber(channel.total_tokens)}</td>
                        <td>{formatDateTime(channel.last_request_time, locale, timeZone)}</td>
                        <td>
                          <RequestDots requests={channel.recent_requests} t={t} locale={locale} timeZone={timeZone} />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <EmptyInline message={t('usage_stats.monitoring_no_matching_data')} />
            )}
          </article>

          <div className={styles.statsGrid}>
            <article className={styles.chartCard}>
              <div className={styles.chartHeader}>
                <div>
                  <h3 className={styles.chartTitle}>{t('usage_stats.monitoring_failures')}</h3>
                  <p className={styles.chartSubtitle}>{t('usage_stats.monitoring_no_failures')}</p>
                </div>
              </div>

              {failureAnalysis.length > 0 ? (
                <div className={styles.tableWrapper}>
                  <table className={styles.table}>
                    <thead>
                      <tr>
                        <th>{t('usage_stats.source_name')}</th>
                        <th>{t('usage_stats.monitoring_table_models')}</th>
                        <th>{t('usage_stats.monitoring_table_failed_requests')}</th>
                        <th>{t('usage_stats.last_failure')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {failureAnalysis.map((failure) => (
                        <tr key={failure.source_key || failure.source}>
                          <td>
                            <div className={styles.cellTitle}>{failure.source}</div>
                            {failure.source_type && <div className={styles.cellMeta}>{failure.source_type}</div>}
                          </td>
                          <td>
                            <div className={styles.modelTagList}>
                              {failure.models.slice(0, 4).map((model) => (
                                <span className={styles.modelTag} key={`${failure.source}-${model.model}`}>
                                  {model.model}
                                </span>
                              ))}
                              {failure.models.length > 4 && <span className={styles.modelTag}>+{failure.models.length - 4}</span>}
                            </div>
                          </td>
                          <td>
                            <span className={`${styles.statusPill} ${styles.statusPillFailed}`.trim()}>
                              {formatCompactNumber(failure.failed_count)}
                            </span>
                          </td>
                          <td>{formatDateTime(failure.last_fail_time, locale, timeZone)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <EmptyInline message={normalizedQuery ? t('usage_stats.monitoring_no_matching_data') : t('usage_stats.monitoring_no_failures')} />
              )}
            </article>

            <article className={styles.chartCard}>
              <div className={styles.chartHeader}>
                <div>
                  <h3 className={styles.chartTitle}>{t('usage_stats.monitoring_request_logs')}</h3>
                  <p className={styles.chartSubtitle}>{t('usage_stats.monitoring_recent_requests')}</p>
                </div>
              </div>

              {requestLogs.length > 0 ? (
                <div className={styles.logList}>
                  {requestLogs.slice(0, 12).map((log) => (
                    <div className={styles.logItem} key={log.id ?? `${log.timestamp}-${log.model}-${log.source}`}>
                      <span className={`${styles.statusPill} ${log.failed ? styles.statusPillFailed : styles.statusPillSuccess}`.trim()}>
                        {log.failed ? t('usage_stats.failure') : t('usage_stats.success')}
                      </span>
                      <div className={styles.logMeta}>
                        <strong>{log.model}</strong>
                        <span>{log.source}</span>
                        <span>{formatDateTime(log.timestamp, locale, timeZone)}</span>
                      </div>
                      <span>{formatCompactNumber(log.tokens.total_tokens)} {t('usage_stats.tokens_count')}</span>
                      <span>{formatDurationMs(log.latency_ms)}</span>
                    </div>
                  ))}
                </div>
              ) : (
                <EmptyInline message={t('usage_stats.monitoring_request_logs_empty')} />
              )}
            </article>
          </div>
        </>
      ) : null}
    </section>
  );
}
