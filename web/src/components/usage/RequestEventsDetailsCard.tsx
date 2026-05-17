import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { Select } from '@/components/ui/Select';
import { ApiError, fetchUsageEventRequestDetail } from '@/lib/api';
import type { UsageEvent, UsageEventRequestDetailResponse, UsageSourceFilterOption } from '@/lib/types';
import { buildRequestDetailViewModel, type RequestDetailHeaderPair, type RequestDetailViewModel } from './requestDetailViewModel';
import {
  calculateCacheRate,
  calculateCost,
  formatDurationMs,
  formatUsd,
  LATENCY_SOURCE_FIELD,
  normalizeAuthIndex,
  type ModelPrice,
} from '@/utils/usage';
import styles from '@/pages/UsagePage.module.scss';

const ALL_FILTER = '__all__';

type SelectOption = { value: string; label: string };

const appendSelectedOption = (
  options: SelectOption[],
  selectedValue: string,
  selectedLabel = selectedValue
) => {
  if (selectedValue === ALL_FILTER || options.some((option) => option.value === selectedValue)) {
    return options;
  }
  return [...options, { value: selectedValue, label: selectedLabel }];
};

type RequestEventRow = {
  id: string;
  usageEventID: string;
  requestID: string;
  timestamp: string;
  timestampMs: number;
  timestampLabel: string;
  model: string;
  sourceRaw: string;
  source: string;
  sourceType: string;
  authIndex: string;
  isDelete: boolean;
  failed: boolean;
  latencyMs: number | null;
  inputTokens: number;
  outputTokens: number;
  reasoningTokens: number;
  cachedTokens: number;
  totalTokens: number;
  cacheRate: string;
  cost: number;
  hasPrice: boolean;
};

export interface RequestEventsDetailsCardProps {
  events: UsageEvent[];
  loading: boolean;
  page: number;
  pageSize: number;
  pageSizeOptions: readonly number[];
  totalCount: number;
  totalPages: number;
  modelOptions: string[];
  sourceOptions: UsageSourceFilterOption[];
  modelFilter: string;
  sourceFilter: string;
  resultFilter: string;
  modelPrices: Record<string, ModelPrice>;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
  onModelFilterChange: (model: string) => void;
  onSourceFilterChange: (source: string) => void;
  onResultFilterChange: (result: string) => void;
}

const toNumber = (value: unknown): number => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return 0;
  return parsed;
};

const formatRequestEventTimestamp = (timestamp: string): string => {
  const match = timestamp.match(/^(\d{4})-(\d{2})-(\d{2})[T\s](\d{2}):(\d{2}):(\d{2})/);
  if (!match) return timestamp || '-';
  return `${match[1]}/${match[2]}/${match[3]} ${match[4]}:${match[5]}:${match[6]}`;
};

const formatCacheRateForSource = (cachedTokens: number, inputTokens: number, sourceType?: string): string => {
  const rate = calculateCacheRate({ inputTokens, cachedTokens, sourceType });
  return rate === null ? '-' : `${rate.toFixed(2)}%`;
};

const getRequestDetailErrorKey = (error: unknown): string => {
  if (error instanceof ApiError && error.status === 413) {
    return 'usage_stats.request_events_detail_too_large';
  }
  if (error instanceof ApiError && error.status === 404) {
    return 'usage_stats.request_events_detail_missing';
  }
  return 'usage_stats.request_events_detail_load_failed';
};

interface RequestDetailBodyBlockProps {
  title: string;
  value?: string;
}

function RequestDetailBodyBlock({ title, value }: RequestDetailBodyBlockProps) {
  if (!value) return null;
  return (
    <div className={styles.requestEventsDetailBodyBlock}>
      <span>{title}</span>
      <pre>{value}</pre>
    </div>
  );
}

interface RequestDetailHeadersProps {
  title: string;
  headers: RequestDetailHeaderPair[];
}

function RequestDetailHeaders({ title, headers }: RequestDetailHeadersProps) {
  if (headers.length === 0) return null;
  return (
    <div className={styles.requestEventsDetailHeadersBlock}>
      <span>{title}</span>
      <dl>
        {headers.map((header, index) => (
          <React.Fragment key={`${header.key}:${header.value}:${index}`}>
            <dt>{header.key}</dt>
            <dd>{header.value}</dd>
          </React.Fragment>
        ))}
      </dl>
    </div>
  );
}

interface RequestDetailStructuredViewProps {
  detail: UsageEventRequestDetailResponse;
  model: RequestDetailViewModel;
  t: (key: string) => string;
}

function RequestDetailStructuredView({ detail, model, t }: RequestDetailStructuredViewProps) {
  const summaryItems = [
    { label: t('usage_stats.request_events_detail_method'), value: model.method },
    { label: t('usage_stats.request_events_detail_path'), value: model.path },
    { label: t('usage_stats.request_events_detail_status'), value: model.status },
    { label: t('usage_stats.request_events_detail_duration'), value: model.duration },
    { label: t('usage_stats.model_name'), value: model.model },
  ].filter((item) => item.value);
  const showStructuredSections = model.kind === 'json' || model.kind === 'http';

  return (
    <div className={styles.requestEventsDetailContent}>
      {model.kind === 'oversized' && (
        <div className={styles.requestEventsDetailNotice}>{t('usage_stats.request_events_detail_parse_skipped')}</div>
      )}
      {summaryItems.length > 0 && (
        <div className={styles.requestEventsDetailSummaryStrip}>
          {summaryItems.map((item) => (
            <div key={item.label} className={styles.requestEventsDetailSummaryChip}>
              <span>{item.label}</span>
              <strong>{item.value}</strong>
            </div>
          ))}
        </div>
      )}
      {showStructuredSections && (
        <div className={styles.requestEventsDetailSectionGrid}>
          <section className={styles.requestEventsDetailSection}>
            <h5>{t('usage_stats.request_events_detail_request_section')}</h5>
            <RequestDetailHeaders title={t('usage_stats.request_events_detail_headers')} headers={model.requestHeaders} />
            <RequestDetailBodyBlock title={t('usage_stats.request_events_detail_body')} value={model.requestBody} />
          </section>
          <section className={styles.requestEventsDetailSection}>
            <h5>{t('usage_stats.request_events_detail_response_section')}</h5>
            <RequestDetailHeaders title={t('usage_stats.request_events_detail_headers')} headers={model.responseHeaders} />
            <RequestDetailBodyBlock title={t('usage_stats.request_events_detail_body')} value={model.responseBody} />
          </section>
        </div>
      )}
      <section className={styles.requestEventsDetailSection}>
        <h5>{t('usage_stats.request_events_detail_raw_log')}</h5>
        <pre className={styles.requestEventsDetailRawLog}>{detail.content}</pre>
      </section>
    </div>
  );
}

function RequestEventsTitle({ title, subtitle, eyebrow, totalLabel }: { title: string; subtitle: string; eyebrow: string; totalLabel: string }) {
  return (
    <div className={styles.sectionTitleBlock}>
      <span className={styles.sectionEyebrow}>{eyebrow}</span>
      <div className={styles.requestEventsTitleRow}>
        <h3 className={styles.sectionTitle}>{title}</h3>
        <span className={styles.requestEventsCountBadge}>{totalLabel}</span>
      </div>
      <p className={styles.sectionSubtitle}>{subtitle}</p>
    </div>
  );
}

export function RequestEventsDetailsCard({
  events,
  loading,
  page,
  pageSize,
  pageSizeOptions,
  totalCount,
  totalPages,
  modelOptions: backendModelOptions,
  sourceOptions: backendSourceOptions,
  modelFilter,
  sourceFilter,
  resultFilter,
  modelPrices,
  onPageChange,
  onPageSizeChange,
  onModelFilterChange,
  onSourceFilterChange,
  onResultFilterChange,
}: RequestEventsDetailsCardProps) {
  const { t } = useTranslation();
  const latencyHint = t('usage_stats.latency_unit_hint', {
    field: LATENCY_SOURCE_FIELD,
    unit: t('usage_stats.duration_unit_ms'),
  });
  const [selectedRow, setSelectedRow] = useState<RequestEventRow | null>(null);
  const [requestDetail, setRequestDetail] = useState<UsageEventRequestDetailResponse | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailErrorKey, setDetailErrorKey] = useState<string | null>(null);
  const requestDetailControllerRef = useRef<AbortController | null>(null);

  const rows = useMemo<RequestEventRow[]>(() => {
    return events.map((event, index) => {
      const timestamp = event.timestamp;
      const timestampMs = Date.parse(timestamp);
      const usageEventID = event.id ? String(event.id) : '';
      const requestID = String(event.request_id ?? '').trim();
      const sourceRaw = String(event.source_raw ?? '').trim() || String(event.source ?? '').trim();
      const authIndexRaw = event.auth_index as unknown;
      const authIndex =
        authIndexRaw === null || authIndexRaw === undefined || authIndexRaw === ''
          ? '-'
          : normalizeAuthIndex(authIndexRaw) || '-';
      const source = String(event.source ?? '').trim() || '-';
      const sourceType = String(event.source_type ?? '').trim();
      const model = String(event.model ?? '').trim() || '-';
      const inputTokens = Math.max(toNumber(event.tokens?.input_tokens), 0);
      const outputTokens = Math.max(toNumber(event.tokens?.output_tokens), 0);
      const reasoningTokens = Math.max(toNumber(event.tokens?.reasoning_tokens), 0);
      const cachedTokens = Math.max(toNumber(event.tokens?.cached_tokens), 0);
      const totalTokens = Math.max(toNumber(event.tokens?.total_tokens), 0);
      const latencyMs = Number.isFinite(event.latency_ms) ? event.latency_ms : null;
      const pricing = modelPrices[model];
      const cost = calculateCost({
        timestamp,
        source,
        source_raw: sourceRaw,
        source_type: sourceType,
        auth_index: authIndex,
        failed: event.failed === true,
        latency_ms: latencyMs ?? 0,
        tokens: {
          input_tokens: inputTokens,
          output_tokens: outputTokens,
          reasoning_tokens: reasoningTokens,
          cached_tokens: cachedTokens,
          total_tokens: totalTokens,
        },
        __modelName: model,
      }, modelPrices);

      return {
        id: usageEventID || `${timestamp}-${model}-${sourceRaw || source}-${authIndex}-${index}`,
        usageEventID,
        requestID,
        timestamp,
        timestampMs: Number.isNaN(timestampMs) ? 0 : timestampMs,
        timestampLabel: formatRequestEventTimestamp(timestamp),
        model,
        sourceRaw: sourceRaw || '-',
        source,
        sourceType,
        authIndex,
        isDelete: event.isDelete === true,
        failed: event.failed === true,
        latencyMs,
        inputTokens,
        outputTokens,
        reasoningTokens,
        cachedTokens,
        totalTokens,
        cacheRate: formatCacheRateForSource(cachedTokens, inputTokens, sourceType),
        cost,
        hasPrice: Boolean(pricing),
      };
    });
  }, [events, modelPrices]);

  const hasLatencyData = useMemo(() => rows.some((row) => row.latencyMs !== null), [rows]);

  const modelOptions = useMemo(() => {
    const options = [
      { value: ALL_FILTER, label: t('usage_stats.filter_all') },
      ...backendModelOptions.map((model) => ({ value: model, label: model })),
    ];
    return appendSelectedOption(options, modelFilter);
  }, [backendModelOptions, modelFilter, t]);

  const sourceOptions = useMemo(() => {
    const options = [
      { value: ALL_FILTER, label: t('usage_stats.filter_all') },
      ...backendSourceOptions.map((source) => ({ value: source.value, label: source.displayName || source.label || source.value })),
    ];
    const selectedSource = backendSourceOptions.find((source) => source.value === sourceFilter);
    const selectedLabel = selectedSource?.displayName || selectedSource?.label;
    return appendSelectedOption(options, sourceFilter, selectedLabel || sourceFilter);
  }, [backendSourceOptions, sourceFilter, t]);

  const resultOptions = useMemo(
    () => [
      { value: ALL_FILTER, label: t('usage_stats.filter_all') },
      { value: 'success', label: t('usage_stats.success') },
      { value: 'failed', label: t('usage_stats.failure') },
    ],
    [t]
  );

  const modelOptionSet = useMemo(
    () => new Set(modelOptions.map((option) => option.value)),
    [modelOptions]
  );
  const sourceOptionSet = useMemo(
    () => new Set(sourceOptions.map((option) => option.value)),
    [sourceOptions]
  );
  const resultOptionSet = useMemo(
    () => new Set(resultOptions.map((option) => option.value)),
    [resultOptions]
  );

  const effectiveModelFilter = modelOptionSet.has(modelFilter) ? modelFilter : ALL_FILTER;
  const effectiveSourceFilter = sourceOptionSet.has(sourceFilter) ? sourceFilter : ALL_FILTER;
  const effectiveResultFilter = resultOptionSet.has(resultFilter) ? resultFilter : ALL_FILTER;

  const hasActiveFilters =
    modelFilter !== ALL_FILTER ||
    sourceFilter !== ALL_FILTER ||
    resultFilter !== ALL_FILTER;

  const computedTotalPages = pageSize > 0 ? Math.ceil(totalCount / pageSize) : 0;
  const safeTotalPages = Math.max(totalPages, computedTotalPages, rows.length > 0 ? 1 : 0);
  const safePage = safeTotalPages > 0 ? Math.min(Math.max(page, 1), safeTotalPages) : 0;
  const pageLabel = safeTotalPages > 0 ? `${safePage} / ${safeTotalPages}` : t('usage_stats.request_events_page_empty');

  const requestDetailViewModel = useMemo(
    () => requestDetail ? buildRequestDetailViewModel(requestDetail.content) : null,
    [requestDetail]
  );

  useEffect(() => {
    return () => requestDetailControllerRef.current?.abort();
  }, []);

  const canOpenRequestDetail = (row: RequestEventRow): boolean => Boolean(row.usageEventID && row.requestID);

  const handleOpenRequestDetail = (row: RequestEventRow) => {
    if (!canOpenRequestDetail(row)) return;

    requestDetailControllerRef.current?.abort();
    const controller = new AbortController();
    requestDetailControllerRef.current = controller;
    setSelectedRow(row);
    setRequestDetail(null);
    setDetailErrorKey(null);
    setDetailLoading(true);

    fetchUsageEventRequestDetail(row.usageEventID, controller.signal)
      .then((detail) => {
        setRequestDetail(detail);
      })
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setDetailErrorKey(getRequestDetailErrorKey(error));
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setDetailLoading(false);
        }
      });
  };

  const handleRequestRowKeyDown = (event: React.KeyboardEvent<HTMLTableRowElement>, row: RequestEventRow) => {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    if (!canOpenRequestDetail(row)) return;

    event.preventDefault();
    handleOpenRequestDetail(row);
  };

  const handleBackToRequestEvents = () => {
    requestDetailControllerRef.current?.abort();
    requestDetailControllerRef.current = null;
    setSelectedRow(null);
    setRequestDetail(null);
    setDetailErrorKey(null);
    setDetailLoading(false);
  };

  const handleClearFilters = () => {
    onModelFilterChange(ALL_FILTER);
    onSourceFilterChange(ALL_FILTER);
    onResultFilterChange(ALL_FILTER);
  };

  const renderRequestDetail = () => {
    if (!selectedRow) return null;

    const displayedRequestID = requestDetail?.request_id || selectedRow.requestID;

    return (
      <div className={styles.requestEventsDetailPanel}>
        <div className={styles.requestEventsDetailHeader}>
          <Button
            variant="ghost"
            size="sm"
            className={styles.usagePillAction}
            onClick={handleBackToRequestEvents}
          >
            {t('usage_stats.request_events_back_to_list')}
          </Button>
          <div>
            <h4 className={styles.requestEventsDetailTitle}>{t('usage_stats.request_events_detail_title')}</h4>
            <p className={styles.sectionSubtitle}>{selectedRow.timestampLabel}</p>
          </div>
        </div>

        <div className={styles.requestEventsDetailMetaGrid}>
          <div className={styles.requestEventsDetailMetaItem}>
            <span>{t('usage_stats.request_events_detail_request_id')}</span>
            <code>{displayedRequestID}</code>
          </div>
          <div className={styles.requestEventsDetailMetaItem}>
            <span>{t('usage_stats.request_events_detail_fetched_at')}</span>
            <strong>{requestDetail?.fetched_at || '-'}</strong>
          </div>
          <div className={styles.requestEventsDetailMetaItem}>
            <span>{t('usage_stats.request_events_detail_cached')}</span>
            <strong>
              {requestDetail ? (requestDetail.cached ? t('usage_stats.request_events_detail_cached_yes') : t('usage_stats.request_events_detail_cached_no')) : '-'}
            </strong>
          </div>
        </div>

        {detailLoading ? (
          <div className={styles.hint}>{t('common.loading')}</div>
        ) : detailErrorKey ? (
          <div className={styles.requestEventsDetailError}>{t(detailErrorKey)}</div>
        ) : (
          requestDetail && requestDetailViewModel ? (
            <RequestDetailStructuredView
              detail={requestDetail}
              model={requestDetailViewModel}
              t={(key) => t(key)}
            />
          ) : null
        )}
      </div>
    );
  };

  return (
    <Card
      className={styles.requestEventsCard}
      title={
        <RequestEventsTitle
          eyebrow={t('usage_stats.request_events_eyebrow')}
          title={t('usage_stats.request_events_title')}
          subtitle={t('usage_stats.request_events_subtitle')}
          totalLabel={t('usage_stats.request_events_total_count', { count: totalCount })}
        />
      }
      extra={
        <div className={styles.requestEventsActions}>
          <Button
            variant="ghost"
            size="sm"
            className={styles.usagePillAction}
            onClick={handleClearFilters}
            disabled={!hasActiveFilters}
          >
            {t('usage_stats.clear_filters')}
          </Button>
        </div>
      }
    >
      <div className={styles.requestEventsToolbar}>
        <div className={styles.requestEventsFiltersGroup}>
          <label className={styles.requestEventsFilterItem}>
            <span className={styles.requestEventsFilterLabel}>
              {t('usage_stats.request_events_filter_model')}
            </span>
            <Select
              value={effectiveModelFilter}
              options={modelOptions}
              onChange={onModelFilterChange}
              className={`${styles.requestEventsSelect} ${styles.usagePillControl}`}
              ariaLabel={t('usage_stats.request_events_filter_model')}
              fullWidth={false}
            />
          </label>
          <label className={styles.requestEventsFilterItem}>
            <span className={styles.requestEventsFilterLabel}>
              {t('usage_stats.request_events_filter_source')}
            </span>
            <Select
              value={effectiveSourceFilter}
              options={sourceOptions}
              onChange={onSourceFilterChange}
              className={`${styles.requestEventsSelect} ${styles.usagePillControl}`}
              ariaLabel={t('usage_stats.request_events_filter_source')}
              fullWidth={false}
            />
          </label>
          <label className={styles.requestEventsFilterItem}>
            <span className={styles.requestEventsFilterLabel}>
              {t('usage_stats.request_events_filter_result')}
            </span>
            <Select
              value={effectiveResultFilter}
              options={resultOptions}
              onChange={onResultFilterChange}
              className={`${styles.requestEventsResultSelect} ${styles.usagePillControl}`}
              ariaLabel={t('usage_stats.request_events_filter_result')}
              fullWidth={false}
            />
          </label>
        </div>
      </div>

      {selectedRow ? (
        renderRequestDetail()
      ) : loading && rows.length === 0 ? (
        <div className={styles.hint}>{t('common.loading')}</div>
      ) : rows.length === 0 ? (
        <EmptyState
          title={t('usage_stats.request_events_empty_title')}
          description={t('usage_stats.request_events_empty_desc')}
        />
      ) : (
        <>
          <div className={styles.requestEventsTableWrapper}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>{t('usage_stats.request_events_timestamp')}</th>
                  <th>{t('usage_stats.model_name')}</th>
                  <th>{t('usage_stats.request_events_source')}</th>
                  <th>{t('usage_stats.request_events_result')}</th>
                  {hasLatencyData && <th title={latencyHint}>{t('usage_stats.time')}</th>}
                  <th>{t('usage_stats.input_tokens')}</th>
                  <th>{t('usage_stats.output_tokens')}</th>
                  <th className={styles.requestEventsReasoningHeader}>{t('usage_stats.reasoning_tokens')}</th>
                  <th>{t('usage_stats.cached_tokens')}</th>
                  <th>{t('usage_stats.cache_rate')}</th>
                  <th>{t('usage_stats.total_tokens')}</th>
                  <th>{t('usage_stats.total_cost')}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => {
                  const canOpenDetail = canOpenRequestDetail(row);
                  return (
                    <tr
                      key={row.id}
                      className={canOpenDetail ? styles.requestEventsClickableRow : undefined}
                      role={canOpenDetail ? 'button' : undefined}
                      tabIndex={canOpenDetail ? 0 : undefined}
                      aria-label={canOpenDetail ? t('usage_stats.request_events_view_detail', { requestId: row.requestID }) : undefined}
                      onClick={canOpenDetail ? () => handleOpenRequestDetail(row) : undefined}
                      onKeyDown={canOpenDetail ? (event) => handleRequestRowKeyDown(event, row) : undefined}
                    >
                    <td title={row.timestamp} className={styles.requestEventsTimestamp}>
                      {row.timestampLabel}
                    </td>
                    <td className={styles.modelCell}>{row.model}</td>
                    <td className={styles.requestEventsSourceCell} title={row.source}>
                      <span className={styles.requestEventsSourceStack}>
                        <span className={styles.requestEventsSourceValue}>{row.source}</span>
                        {(row.isDelete || row.sourceType) && (
                          <span className={styles.requestEventsSourceTags}>
                            {row.sourceType && (
                              <span className={styles.credentialType}>{row.sourceType}</span>
                            )}
                            {row.isDelete && (
                              <span className={styles.requestEventsDeletedTag}>{t('usage_stats.deleted')}</span>
                            )}
                          </span>
                        )}
                      </span>
                    </td>
                    <td>
                      <span
                        className={
                          row.failed
                            ? styles.requestEventsResultFailed
                            : styles.requestEventsResultSuccess
                        }
                      >
                        {row.failed ? t('usage_stats.failure') : t('usage_stats.success')}
                      </span>
                    </td>
                    {hasLatencyData && (
                      <td className={styles.durationCell}>{formatDurationMs(row.latencyMs)}</td>
                    )}
                    <td>{row.inputTokens.toLocaleString()}</td>
                    <td>{row.outputTokens.toLocaleString()}</td>
                    <td>{row.reasoningTokens.toLocaleString()}</td>
                    <td>{row.cachedTokens.toLocaleString()}</td>
                    <td>{row.cacheRate}</td>
                    <td>{row.totalTokens.toLocaleString()}</td>
                    <td title={row.hasPrice ? undefined : t('usage_stats.cost_need_price')}>
                      {row.hasPrice ? formatUsd(row.cost) : '-'}
                    </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <div className={styles.requestEventsPaginationFooter}>
            <div className={styles.requestEventsPaginationControls}>
              <label className={styles.requestEventsPageSizeControl}>
                <span>{t('usage_stats.request_events_rows_per_page')}</span>
                <select value={pageSize} onChange={(event) => onPageSizeChange(Number(event.target.value))} disabled={loading}>
                  {pageSizeOptions.map((option) => <option key={option} value={option}>{option}</option>)}
                </select>
              </label>
              <button type="button" className={styles.requestEventsPagerButton} onClick={() => onPageChange(page - 1)} disabled={loading || safePage <= 1}>
                {t('usage_stats.request_events_previous_page')}
              </button>
              <span className={styles.requestEventsPaginationPage}>{pageLabel}</span>
              <button type="button" className={styles.requestEventsPagerButton} onClick={() => onPageChange(page + 1)} disabled={loading || safeTotalPages === 0 || safePage >= safeTotalPages}>
                {t('usage_stats.request_events_next_page')}
              </button>
            </div>
          </div>
        </>
      )}
    </Card>
  );
}
