import { useId } from 'react';
import { Bar, Line } from 'react-chartjs-2';
import { useTranslation } from 'react-i18next';
import { EmptyState } from '@/components/ui/EmptyState';
import { LoadingSpinner } from '@/components/ui/LoadingSpinner';
import type { UsageMonitoringResponse } from '@/lib/usageMonitoringTypes';
import { formatCompactNumber, formatDurationMs, formatPerMinuteValue, formatUsd } from '@/utils/usage';
import styles from './MonitoringCenterTab.module.scss';

interface MonitoringCenterTabProps {
  data: UsageMonitoringResponse | null;
  loading: boolean;
}

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      display: true,
      position: 'bottom' as const,
    },
  },
  scales: {
    x: {
      grid: { display: false },
    },
    y: {
      beginAtZero: true,
      grid: { color: 'rgba(148, 163, 184, 0.18)' },
    },
  },
};

function formatRate(value: number): string {
  return `${Math.max(0, Math.min(100, value || 0)).toFixed(1)}%`;
}

function formatDateTime(value: string | null | undefined, locale: string, timeZone?: string): string {
  if (!value) return '-';
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return value;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: 'short',
    timeStyle: 'medium',
    timeZone: timeZone || undefined,
  }).format(new Date(timestamp));
}

type Translate = (key: string, options?: Record<string, unknown>) => string;

function statusClass(failed: boolean): string {
  return failed ? styles.statusFailed : styles.statusSuccess;
}

function RequestDots({ requests, t, locale, timeZone }: { requests: Array<{ timestamp: string; failed: boolean }>; t: Translate; locale: string; timeZone?: string }) {
  if (!requests.length) {
    return <span className={styles.muted}>-</span>;
  }
  return (
    <ol className={styles.requestDots} aria-label={t('usage_stats.monitoring_recent_requests_count', { count: requests.length })}>
      {requests.map((request, index) => (
        <li key={`${request.timestamp}-${index}`}>
          <span
            className={`${styles.requestDot} ${statusClass(request.failed)}`.trim()}
            aria-hidden="true"
          >
            {request.failed ? '!' : '✓'}
          </span>
          <span className={styles.srOnly}>{t('usage_stats.monitoring_recent_request_item', {
            time: formatDateTime(request.timestamp, locale, timeZone),
            result: request.failed ? t('usage_stats.failure') : t('usage_stats.success'),
          })}</span>
        </li>
      ))}
    </ol>
  );
}

function monitoringModelSummary(items: UsageMonitoringResponse['model_distribution'], t: Translate): string {
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

function monitoringDailyTrendSummary(items: UsageMonitoringResponse['daily_trend'], t: Translate): string {
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

export function MonitoringCenterTab({ data, loading }: MonitoringCenterTabProps) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? 'zh-CN' : 'en-US';
  const timeZone = data?.timezone;
  const modelDistributionTitleId = useId();
  const modelDistributionSummaryId = useId();
  const dailyTrendTitleId = useId();
  const dailyTrendSummaryId = useId();
  const kpis = data?.kpis;
  const hasData = Boolean(kpis && kpis.total_requests > 0);

  const dailyTrend = data?.daily_trend ?? [];
  const dailyTrendSummary = monitoringDailyTrendSummary(dailyTrend, t);
  const dailyTrendData = {
    labels: dailyTrend.map((point) => point.date),
    datasets: [
      {
        label: t('usage_stats.requests_count'),
        data: dailyTrend.map((point) => point.requests),
        borderColor: '#7c3aed',
        backgroundColor: 'rgba(124, 58, 237, 0.18)',
        tension: 0.35,
        fill: true,
      },
      {
        label: t('usage_stats.tokens_count'),
        data: dailyTrend.map((point) => point.tokens),
        borderColor: '#0ea5e9',
        backgroundColor: 'rgba(14, 165, 233, 0.12)',
        tension: 0.35,
        fill: true,
      },
    ],
  };

  const modelDistribution = data?.model_distribution ?? [];
  const modelDistributionSummary = monitoringModelSummary(modelDistribution, t);
  const modelDistributionData = {
    labels: modelDistribution.slice(0, 8).map((item) => item.model),
    datasets: [{
      label: t('usage_stats.requests_count'),
      data: modelDistribution.slice(0, 8).map((item) => item.total_requests),
      backgroundColor: 'rgba(99, 102, 241, 0.72)',
      borderRadius: 8,
    }],
  };

  if (loading && !data) {
    return (
      <div className={styles.loadingState} aria-busy="true">
        <LoadingSpinner size={28} />
        <span>{t('common.loading')}</span>
      </div>
    );
  }

  if (!loading && !hasData) {
    return (
      <EmptyState
        title={t('usage_stats.monitoring_empty_title')}
        description={t('usage_stats.monitoring_empty_desc')}
      />
    );
  }

  return (
    <section className={styles.monitoringShell} aria-busy={loading}>
      <div className={styles.kpiGrid}>
        <div className={styles.kpiCard}>
          <span className={styles.kpiLabel}>{t('usage_stats.total_requests')}</span>
          <strong>{formatCompactNumber(kpis?.total_requests ?? 0)}</strong>
          <span className={styles.kpiMeta}>{t('usage_stats.success_requests')}: {formatCompactNumber(kpis?.success_requests ?? 0)} · {t('usage_stats.failed_requests')}: {formatCompactNumber(kpis?.failed_requests ?? 0)}</span>
        </div>
        <div className={styles.kpiCard}>
          <span className={styles.kpiLabel}>{t('usage_stats.total_tokens')}</span>
          <strong>{formatCompactNumber(kpis?.total_tokens ?? 0)}</strong>
          <span className={styles.kpiMeta}>{t('usage_stats.input_tokens')}: {formatCompactNumber(kpis?.input_tokens ?? 0)} · {t('usage_stats.output_tokens')}: {formatCompactNumber(kpis?.output_tokens ?? 0)}</span>
        </div>
        <div className={styles.kpiCard}>
          <span className={styles.kpiLabel}>{t('usage_stats.rpm')} / {t('usage_stats.tpm')}</span>
          <strong>{formatPerMinuteValue(kpis?.rpm ?? 0)} / {formatPerMinuteValue(kpis?.tpm ?? 0)}</strong>
          <span className={styles.kpiMeta}>{t('usage_stats.cached_tokens')}: {formatCompactNumber(kpis?.cached_tokens ?? 0)} · {t('usage_stats.reasoning_tokens')}: {formatCompactNumber(kpis?.reasoning_tokens ?? 0)}</span>
        </div>
        <div className={styles.kpiCard}>
          <span className={styles.kpiLabel}>{t('usage_stats.total_cost')}</span>
          <strong>{formatUsd(kpis?.total_cost ?? 0)}</strong>
          <span className={styles.kpiMeta}>{kpis?.cost_available ? t('usage_stats.monitoring_cost_available') : t('usage_stats.cost_need_price')}</span>
        </div>
      </div>

      <div className={styles.chartGrid}>
        <div className={styles.panel}>
          <div className={styles.panelHeader}>
            <span id={modelDistributionTitleId}>{t('usage_stats.monitoring_model_distribution')}</span>
          </div>
          <p id={modelDistributionSummaryId} className={styles.srOnly}>{modelDistributionSummary}</p>
          <div className={styles.chartBox} role="img" aria-labelledby={modelDistributionTitleId} aria-describedby={modelDistributionSummaryId}>
            <Bar data={modelDistributionData} options={chartOptions} />
          </div>
          <table className={styles.srOnlyTable} aria-label={t('usage_stats.monitoring_model_distribution')}>
            <thead>
              <tr>
                <th>{t('usage_stats.model_name')}</th>
                <th>{t('usage_stats.requests_count')}</th>
                <th>{t('usage_stats.success_rate')}</th>
              </tr>
            </thead>
            <tbody>
              {modelDistribution.slice(0, 8).map((item) => (
                <tr key={item.model}>
                  <td>{item.model}</td>
                  <td>{item.total_requests}</td>
                  <td>{formatRate(item.success_rate)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className={styles.panel}>
          <div className={styles.panelHeader}>
            <span id={dailyTrendTitleId}>{t('usage_stats.monitoring_daily_trend')}</span>
          </div>
          <p id={dailyTrendSummaryId} className={styles.srOnly}>{dailyTrendSummary}</p>
          <div className={styles.chartBox} role="img" aria-labelledby={dailyTrendTitleId} aria-describedby={dailyTrendSummaryId}>
            <Line data={dailyTrendData} options={chartOptions} />
          </div>
          <table className={styles.srOnlyTable} aria-label={t('usage_stats.monitoring_daily_trend')}>
            <thead>
              <tr>
                <th>{t('usage_stats.time')}</th>
                <th>{t('usage_stats.requests_count')}</th>
                <th>{t('usage_stats.tokens_count')}</th>
              </tr>
            </thead>
            <tbody>
              {dailyTrend.map((point) => (
                <tr key={point.date}>
                  <td>{point.date}</td>
                  <td>{point.requests}</td>
                  <td>{point.tokens}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className={styles.panel}>
        <div className={styles.panelHeader}>
          <span>{t('usage_stats.monitoring_channels')}</span>
          <small>{t('usage_stats.monitoring_recent_requests')}</small>
        </div>
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>{t('usage_stats.source_name')}</th>
                <th>{t('usage_stats.requests_count')}</th>
                <th>{t('usage_stats.success_rate')}</th>
                <th>{t('usage_stats.total_tokens')}</th>
                <th>{t('usage_stats.last_request')}</th>
                <th>{t('usage_stats.monitoring_recent_requests')}</th>
              </tr>
            </thead>
            <tbody>
              {(data?.channel_stats ?? []).map((channel) => (
                <tr key={channel.source_key || channel.source}>
                  <td>
                    <div className={styles.cellTitle}>{channel.source}</div>
                    {channel.source_type && <div className={styles.cellMeta}>{channel.source_type}</div>}
                  </td>
                  <td>{formatCompactNumber(channel.total_requests)}</td>
                  <td>{formatRate(channel.success_rate)}</td>
                  <td>{formatCompactNumber(channel.total_tokens)}</td>
                  <td>{formatDateTime(channel.last_request_time, locale, timeZone)}</td>
                  <td><RequestDots requests={channel.recent_requests} t={t} locale={locale} timeZone={timeZone} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className={styles.splitGrid}>
        <div className={styles.panel}>
          <div className={styles.panelHeader}>
            <span>{t('usage_stats.monitoring_failures')}</span>
          </div>
          <div className={styles.failureList}>
            {(data?.failure_analysis ?? []).length === 0 && <div className={styles.emptyInline}>{t('usage_stats.monitoring_no_failures')}</div>}
            {(data?.failure_analysis ?? []).map((failure) => (
              <div className={styles.failureItem} key={failure.source_key || failure.source}>
                <div>
                  <strong>{failure.source}</strong>
                  <span>{formatCompactNumber(failure.failed_count)} {t('usage_stats.failure')}</span>
                </div>
                <small>{t('usage_stats.last_failure')}: {formatDateTime(failure.last_fail_time, locale, timeZone)}</small>
                <div className={styles.modelBadges}>
                  {failure.models.slice(0, 4).map((model) => (
                    <span key={model.model}>{model.model}: {model.failure}/{model.total}</span>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className={styles.panel}>
          <div className={styles.panelHeader}>
            <span>{t('usage_stats.monitoring_request_logs')}</span>
          </div>
          <div className={styles.logList}>
            {(data?.request_logs ?? []).slice(0, 10).map((log) => (
              <div className={styles.logItem} key={log.id ?? `${log.timestamp}-${log.model}-${log.source}`}>
                <span className={`${styles.logStatus} ${statusClass(log.failed)}`.trim()}>{log.failed ? t('usage_stats.failure') : t('usage_stats.success')}</span>
                <div>
                  <strong>{log.model}</strong>
                  <small>{log.source} · {formatDateTime(log.timestamp, locale, timeZone)}</small>
                </div>
                <span>{formatCompactNumber(log.tokens.total_tokens)} {t('usage_stats.tokens_count')}</span>
                <span>{formatDurationMs(log.latency_ms)}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
