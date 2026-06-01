import React, { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { Select } from '@/components/ui/Select';
import { ApiError, fetchUsageEventRequestDetail } from '@/lib/api';
import type { UsageEvent, UsageEventAttempt, UsageEventRequestDetailResponse, UsageSourceFilterOption } from '@/lib/types';
import { buildRequestDetailViewModel, type RequestDetailViewModel } from './requestDetailViewModel';
import {
  RequestDetailJsonBlock,
  requestDetailBodyToJsonValue,
  requestDetailHeadersToJsonValue,
  type RequestDetailJsonLabels,
} from './RequestDetailJsonViewer';
import {
  calculateCacheRate,
  formatDurationMs,
  formatUsd,
  LATENCY_SOURCE_FIELD,
  normalizeAuthIndex,
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

export type RequestEventAttemptRow = {
  id: string;
  timestamp: string;
  timestampLabel: string;
  source: string;
  sourceType: string;
  failed: boolean;
  latencyMs: number | null;
  totalTokens: number;
};

export type RequestEventTileRow = {
  id: string;
  usageEventID: string;
  requestID: string;
  timestamp: string;
  timestampMs: number;
  timestampLabel: string;
  apiKey: string;
  model: string;
  reasoningEffort: string;
  requestType: string;
  endpoint: string;
  sourceRaw: string;
  source: string;
  sourceTitle?: string;
  sourceType: string;
  authIndex: string;
  isDelete: boolean;
  failed: boolean;
  latencyMs: number | null;
  ttftMs: number | null;
  inputTokens: number;
  outputTokens: number;
  reasoningTokens: number;
  cachedTokens: number;
  totalTokens: number;
  cacheRate: string;
  cost: number | null;
  costAvailable: boolean;
  attempts: RequestEventAttemptRow[];
  attemptCount: number;
};

type RequestEventsTranslate = (key: string, options?: Record<string, unknown>) => string;

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

const mapRequestEventAttempts = (
  attempts: UsageEventAttempt[] | undefined,
): RequestEventAttemptRow[] => {
  if (!Array.isArray(attempts)) return [];
  return attempts.map((attempt, index) => {
    const timestamp = String(attempt.timestamp ?? '').trim();
    const source = String(attempt.source ?? '').trim() || '-';
    const sourceType = String(attempt.source_type ?? '').trim();
    const latencyMs = Number.isFinite(attempt.latency_ms) ? attempt.latency_ms : null;
    return {
      id: String(attempt.id ?? '').trim() || `${timestamp}-${index}`,
      timestamp,
      timestampLabel: formatRequestEventTimestamp(timestamp),
      source,
      sourceType,
      failed: attempt.failed === true,
      latencyMs,
      totalTokens: Math.max(toNumber(attempt.total_tokens), 0),
    };
  });
};

export const formatRequestEventTimestamp = (timestamp: string): string => {
  const match = timestamp.match(/^(\d{4})-(\d{2})-(\d{2})[T\s](\d{2}):(\d{2}):(\d{2})/);
  if (!match) return timestamp || '-';
  return `${match[1]}/${match[2]}/${match[3]} ${match[4]}:${match[5]}:${match[6]}`;
};

export const formatCacheRateForSource = (cachedTokens: number, inputTokens: number, sourceType?: string): string => {
  const rate = calculateCacheRate({ inputTokens, cachedTokens, sourceType });
  return rate === null ? '-' : `${rate.toFixed(2)}%`;
};

export const getRequestDetailErrorKey = (error: unknown): string => {
  if (error instanceof ApiError && error.status === 413) {
    return 'usage_stats.request_events_detail_too_large';
  }
  if (error instanceof ApiError && error.status === 404) {
    return 'usage_stats.request_events_detail_missing';
  }
  return 'usage_stats.request_events_detail_load_failed';
};

const formatTTFTMs = (ttftMs: number | null): string => {
  if (ttftMs === null || ttftMs <= 0) {
    return '-';
  }
  return formatDurationMs(ttftMs);
};

const parseRequestEndpoint = (rawEndpoint: unknown): { requestType: string; endpoint: string } => {
  const raw = String(rawEndpoint ?? '').trim().replace(/\s+/g, ' ');
  if (!raw) {
    return { requestType: '-', endpoint: '-' };
  }
  const [first, ...rest] = raw.split(' ');
  const upperMethod = first.toUpperCase();
  const hasMethod = ['GET', 'POST'].includes(upperMethod);
  const requestType = upperMethod === 'POST' ? 'SSE' : upperMethod === 'GET' ? 'WS' : '-';
  const path = hasMethod ? rest.join(' ').trim() : raw;
  const normalizedPath = path.startsWith('/v1/') ? path.slice(3) : path === '/v1' ? '/' : path;
  return { requestType, endpoint: normalizedPath || '-' };
};

type RequestDetailContentPage = 'parsed' | 'raw';
type RequestDetailTrafficSide = 'request' | 'response';

interface RequestDetailViewState {
  requestId: string;
  page: RequestDetailContentPage;
  trafficSide: RequestDetailTrafficSide;
}
export interface RequestDetailStructuredViewProps {
  detail: UsageEventRequestDetailResponse;
  model: RequestDetailViewModel;
  t: (key: string) => string;
}

export function RequestDetailStructuredView({ detail, model, t }: RequestDetailStructuredViewProps) {
  const showStructuredSections = model.kind === 'json' || model.kind === 'http';
  const defaultPage: RequestDetailContentPage = showStructuredSections ? 'parsed' : 'raw';
  const [viewState, setViewState] = useState<RequestDetailViewState>(() => ({
    requestId: detail.request_id,
    page: defaultPage,
    trafficSide: 'request',
  }));
  const isCurrentDetailViewState = viewState.requestId === detail.request_id;
  const selectedPage = showStructuredSections && isCurrentDetailViewState ? viewState.page : defaultPage;
  const activeTrafficSide = isCurrentDetailViewState ? viewState.trafficSide : 'request';
  const summaryItems = [
    { label: t('usage_stats.request_events_detail_method'), value: model.method },
    { label: t('usage_stats.request_events_detail_path'), value: model.path },
    { label: t('usage_stats.request_events_detail_status'), value: model.status },
    { label: t('usage_stats.request_events_detail_duration'), value: model.duration },
    { label: t('usage_stats.model_name'), value: model.model },
  ].filter((item) => item.value);
  const requestHeadersJson = useMemo(() => requestDetailHeadersToJsonValue(model.requestHeaders), [model.requestHeaders]);
  const responseHeadersJson = useMemo(() => requestDetailHeadersToJsonValue(model.responseHeaders), [model.responseHeaders]);
  const requestBodyJson = useMemo(() => requestDetailBodyToJsonValue(model.requestBody), [model.requestBody]);
  const responseBodyJson = useMemo(() => requestDetailBodyToJsonValue(model.responseBody), [model.responseBody]);
  const jsonLabels = useMemo<RequestDetailJsonLabels>(() => ({
    collapseNode: t('usage_stats.request_events_detail_json_collapse_node'),
    copiedNode: t('usage_stats.request_events_detail_json_copied_node'),
    copyNode: t('usage_stats.request_events_detail_json_copy_node'),
    expandNode: t('usage_stats.request_events_detail_json_expand_node'),
    jsonString: t('usage_stats.request_events_detail_json_string'),
    parseString: t('usage_stats.request_events_detail_json_parse_string'),
    rawString: t('usage_stats.request_events_detail_json_raw_string'),
  }), [t]);

  const pageTabs: Array<{ id: RequestDetailContentPage; label: string }> = showStructuredSections
    ? [
        { id: 'parsed', label: t('usage_stats.request_events_detail_parsed_page') },
        { id: 'raw', label: t('usage_stats.request_events_detail_raw_page') },
      ]
    : [{ id: 'raw', label: t('usage_stats.request_events_detail_raw_page') }];
  const trafficTabs: Array<{ id: RequestDetailTrafficSide; label: string }> = [
    { id: 'request', label: t('usage_stats.request_events_detail_request_section') },
    { id: 'response', label: t('usage_stats.request_events_detail_response_section') },
  ];
  const activeTrafficBlocks = useMemo(() => (activeTrafficSide === 'request'
    ? [
        { title: t('usage_stats.request_events_detail_request_headers'), rootName: 'request_headers', value: requestHeadersJson },
        { title: t('usage_stats.request_events_detail_request_body'), rootName: 'request_body', value: requestBodyJson },
      ]
    : [
        { title: t('usage_stats.request_events_detail_response_headers'), rootName: 'response_headers', value: responseHeadersJson },
        { title: t('usage_stats.request_events_detail_response_body'), rootName: 'response_body', value: responseBodyJson },
      ]), [activeTrafficSide, requestBodyJson, requestHeadersJson, responseBodyJson, responseHeadersJson, t]);

  return (
    <div className={styles.requestEventsDetailContent}>
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
      <div className={styles.requestEventsDetailPageTabs} role="tablist" aria-label={t('usage_stats.request_events_detail_title')}>
        {pageTabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={selectedPage === tab.id}
            className={`${styles.requestEventsDetailPageTab} ${selectedPage === tab.id ? styles.requestEventsDetailPageTabActive : ''}`}
            onClick={() => setViewState({ requestId: detail.request_id, page: tab.id, trafficSide: activeTrafficSide })}
          >
            {tab.label}
          </button>
        ))}
      </div>
      {selectedPage === 'parsed' && showStructuredSections ? (
        <section className={styles.requestEventsDetailSection} role="tabpanel">
          <div className={styles.requestEventsDetailTrafficTabs} role="tablist" aria-label={t('usage_stats.request_events_detail_parsed_page')}>
            {trafficTabs.map((tab) => (
              <button
                key={tab.id}
                type="button"
                role="tab"
                aria-selected={activeTrafficSide === tab.id}
                className={`${styles.requestEventsDetailTrafficTab} ${activeTrafficSide === tab.id ? styles.requestEventsDetailTrafficTabActive : ''}`}
                onClick={() => setViewState({ requestId: detail.request_id, page: selectedPage, trafficSide: tab.id })}
              >
                {tab.label}
              </button>
            ))}
          </div>
          <div className={styles.requestEventsDetailJsonGrid}>
            {activeTrafficBlocks.map((block) => (
              <RequestDetailJsonBlock key={block.rootName} labels={jsonLabels} title={block.title} rootName={block.rootName} value={block.value} />
            ))}
          </div>
        </section>
      ) : (
        <section className={styles.requestEventsDetailSection} role="tabpanel">
          <h5>{t('usage_stats.request_events_detail_raw_log')}</h5>
          <textarea
            key={detail.request_id}
            className={styles.requestEventsDetailRawLog}
            aria-label={t('usage_stats.request_events_detail_raw_log')}
            readOnly
            spellCheck={false}
            wrap="off"
            defaultValue={detail.content}
          />
        </section>
      )}
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

interface RequestEventTableRowProps {
  row: RequestEventTileRow;
  canOpenDetail: boolean;
  showLatency: boolean;
  showTTFT?: boolean;
  showCost?: boolean;
  t: RequestEventsTranslate;
  onOpenDetail?: (row: RequestEventTileRow) => void;
}

export function RequestEventTableRow({
  row,
  canOpenDetail,
  showLatency,
  showTTFT = false,
  showCost = true,
  t,
  onOpenDetail,
}: RequestEventTableRowProps) {
  const isInteractive = canOpenDetail && Boolean(onOpenDetail);

  const handleKeyDown = (event: React.KeyboardEvent<HTMLTableRowElement>) => {
    if (!isInteractive) return;
    if (event.key !== 'Enter' && event.key !== ' ') return;

    event.preventDefault();
    onOpenDetail?.(row);
  };

  return (
    <tr
      className={isInteractive ? styles.requestEventsClickableRow : undefined}
      role={isInteractive ? 'button' : undefined}
      tabIndex={isInteractive ? 0 : undefined}
      aria-label={isInteractive ? t('usage_stats.request_events_view_detail', { requestId: row.requestID }) : undefined}
      onClick={isInteractive ? () => onOpenDetail?.(row) : undefined}
      onKeyDown={handleKeyDown}
    >
      <td title={row.timestamp} className={styles.requestEventsTimestamp}>
        {row.timestampLabel}
      </td>
      <td className={styles.requestEventsAPIKeyCell} title={row.apiKey}>{row.apiKey}</td>
      <td className={styles.requestEventsSourceCell} title={row.sourceTitle ?? row.source}>
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
      <td className={styles.modelCell}>{row.model}</td>
      <td>{row.reasoningEffort}</td>
      <td>
        <RequestEventResultCell row={row} t={t} />
      </td>
      {showTTFT && (
        <td className={styles.durationCell}>{formatTTFTMs(row.ttftMs)}</td>
      )}
      {showLatency && (
        <td className={styles.durationCell}>{formatDurationMs(row.latencyMs)}</td>
      )}
      <td>{row.requestType}</td>
      <td className={styles.requestEventsEndpointCell} title={row.endpoint}>{row.endpoint}</td>
      <td>{row.inputTokens.toLocaleString()}</td>
      <td>{row.outputTokens.toLocaleString()}</td>
      <td>{row.reasoningTokens.toLocaleString()}</td>
      <td>{row.cachedTokens.toLocaleString()}</td>
      <td>{row.cacheRate}</td>
      <td>{row.totalTokens.toLocaleString()}</td>
      {showCost && (
        <td title={row.costAvailable ? undefined : t('usage_stats.cost_need_price')}>
          {row.costAvailable && row.cost !== null ? formatUsd(row.cost) : '-'}
        </td>
      )}
    </tr>
  );
}

function RequestEventResultCell({ row, t }: { row: RequestEventTileRow; t: RequestEventsTranslate }) {
  const retryPanelId = useId();
  const retrySummaryRef = useRef<HTMLButtonElement | null>(null);
  const retryPanelRef = useRef<HTMLDivElement | null>(null);
  const [isRetryPanelOpen, setIsRetryPanelOpen] = useState(false);
  const [retryPanelPosition, setRetryPanelPosition] = useState({ left: 0, top: 0, transform: 'none' });
  const displayAttemptCount = Math.max(row.attemptCount, row.attempts.length);
  const hasRetryProcess = displayAttemptCount > 1 && row.attempts.length > 0;
  const resultClassName = row.failed
    ? styles.requestEventsResultFailed
    : styles.requestEventsResultSuccess;
  const resultLabel = row.failed ? t('usage_stats.failure') : t('usage_stats.success');

  const updateRetryPanelPosition = useCallback(() => {
    if (typeof window === 'undefined' || !retrySummaryRef.current) return;
    const rect = retrySummaryRef.current.getBoundingClientRect();
    const margin = 12;
    const panelWidth = 360;
    const panelHeight = Math.min(320, 120 + row.attempts.length * 72);
    const maxLeft = Math.max(margin, window.innerWidth - panelWidth - margin);
    const left = Math.min(Math.max(rect.left, margin), maxLeft);
    const opensAbove = rect.bottom + panelHeight + margin > window.innerHeight && rect.top > panelHeight;
    setRetryPanelPosition({
      left,
      top: opensAbove ? rect.top - 8 : rect.bottom + 8,
      transform: opensAbove ? 'translateY(-100%)' : 'none',
    });
  }, [row.attempts.length]);

  useEffect(() => {
    if (!isRetryPanelOpen || typeof document === 'undefined') return undefined;
    updateRetryPanelPosition();
    const closeOnOutsidePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) return;
      if (retrySummaryRef.current?.contains(target) || retryPanelRef.current?.contains(target)) return;
      setIsRetryPanelOpen(false);
    };
    const reposition = () => updateRetryPanelPosition();
    document.addEventListener('pointerdown', closeOnOutsidePointerDown);
    window.addEventListener('resize', reposition);
    window.addEventListener('scroll', reposition, true);
    return () => {
      document.removeEventListener('pointerdown', closeOnOutsidePointerDown);
      window.removeEventListener('resize', reposition);
      window.removeEventListener('scroll', reposition, true);
    };
  }, [isRetryPanelOpen, row.attempts.length, updateRetryPanelPosition]);

  if (!hasRetryProcess) {
    return <span className={resultClassName}>{resultLabel}</span>;
  }

  const handleRetryDetailsClick = (event: React.MouseEvent<HTMLDivElement>) => {
    event.stopPropagation();
  };
  const handleRetrySummaryClick = (event: React.MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    updateRetryPanelPosition();
    setIsRetryPanelOpen((open) => !open);
  };
  const handleRetrySummaryKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
    if (event.key !== 'Enter' && event.key !== ' ' && event.key !== 'Escape') return;
    event.stopPropagation();
    if (event.key === 'Escape') {
      setIsRetryPanelOpen(false);
    }
  };
  const retryPanel = (
    <div
      id={retryPanelId}
      ref={retryPanelRef}
      className={styles.requestEventsRetryPanel}
      role="group"
      aria-label={t('usage_stats.request_events_retry_process')}
      hidden={!isRetryPanelOpen}
      style={typeof document === 'undefined' ? undefined : retryPanelPosition}
      onClick={(event) => event.stopPropagation()}
    >
      <div className={styles.requestEventsRetryHeader}>
        <span>{t('usage_stats.request_events_retry_process')}</span>
        <strong>{t('usage_stats.request_events_attempt_count', { count: displayAttemptCount })}</strong>
      </div>
      <div className={styles.requestEventsRetryList}>
        {row.attempts.map((attempt) => (
          <div key={attempt.id} className={styles.requestEventsRetryAttempt}>
            <span className={attempt.failed ? styles.requestEventsResultFailed : styles.requestEventsResultSuccess}>
              {attempt.failed ? t('usage_stats.failure') : t('usage_stats.success')}
            </span>
            <span className={styles.requestEventsRetryAttemptMeta}>
              <strong>{attempt.source}</strong>
              <span>{attempt.timestampLabel}</span>
              <span>{formatDurationMs(attempt.latencyMs)}</span>
              <span>{attempt.totalTokens.toLocaleString()}</span>
            </span>
          </div>
        ))}
      </div>
    </div>
  );

  return (
    <div className={styles.requestEventsRetryDetails} onClick={handleRetryDetailsClick}>
      <span className={resultClassName}>{resultLabel}</span>
      <button
        ref={retrySummaryRef}
        type="button"
        className={styles.requestEventsRetrySummary}
        aria-expanded={isRetryPanelOpen}
        aria-controls={retryPanelId}
        onClick={handleRetrySummaryClick}
        onKeyDown={handleRetrySummaryKeyDown}
      >
        <span className={styles.requestEventsRetryIndicator}>{t('usage_stats.request_events_retry_indicator')}</span>
      </button>
      {typeof document === 'undefined' ? retryPanel : createPortal(retryPanel, document.body)}
    </div>
  );
}
interface RequestEventsTableProps {
  rows: RequestEventTileRow[];
  canOpenDetail: (row: RequestEventTileRow) => boolean;
  showLatency: boolean;
  showTTFT?: boolean;
  showCost?: boolean;
  latencyHint?: string;
  ttftHint?: string;
  t: RequestEventsTranslate;
  onOpenDetail?: (row: RequestEventTileRow) => void;
  footer?: React.ReactNode;
}

export function RequestEventsTable({
  rows,
  canOpenDetail,
  showLatency,
  showTTFT = false,
  showCost = true,
  latencyHint,
  ttftHint,
  t,
  onOpenDetail,
  footer,
}: RequestEventsTableProps) {
  return (
    <div className={styles.requestEventsTableFrame}>
      <div className={styles.requestEventsTableScroll}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>{t('usage_stats.request_events_timestamp')}</th>
              <th>{t('usage_stats.api_key_filter')}</th>
              <th>{t('usage_stats.request_events_source')}</th>
              <th>{t('usage_stats.model_name')}</th>
              <th title={t('usage_stats.reasoning_effort_hint')}>{t('usage_stats.reasoning_effort')}</th>
              <th>{t('usage_stats.request_events_result')}</th>
              {showTTFT && <th title={ttftHint}>{t('usage_stats.ttft')}</th>}
              {showLatency && <th title={latencyHint}>{t('usage_stats.latency')}</th>}
              <th>{t('usage_stats.request_type')}</th>
              <th>{t('usage_stats.request_endpoint')}</th>
              <th>{t('usage_stats.input_tokens')}</th>
              <th>{t('usage_stats.output_tokens')}</th>
              <th className={styles.requestEventsReasoningHeader}>{t('usage_stats.reasoning_tokens')}</th>
              <th>{t('usage_stats.cached_tokens')}</th>
              <th>{t('usage_stats.cache_rate')}</th>
              <th>{t('usage_stats.total_tokens')}</th>
              {showCost && <th>{t('usage_stats.total_cost')}</th>}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <RequestEventTableRow
                key={row.id}
                row={row}
                canOpenDetail={canOpenDetail(row)}
                showLatency={showLatency}
                showTTFT={showTTFT}
                showCost={showCost}
                t={t}
                onOpenDetail={onOpenDetail}
              />
            ))}
          </tbody>
        </table>
      </div>
      {footer ? <div className={styles.requestEventsTableFooter}>{footer}</div> : null}
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
  const [selectedRow, setSelectedRow] = useState<RequestEventTileRow | null>(null);
  const [requestDetail, setRequestDetail] = useState<UsageEventRequestDetailResponse | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailErrorKey, setDetailErrorKey] = useState<string | null>(null);
  const requestDetailControllerRef = useRef<AbortController | null>(null);
  const ttftHint = t('usage_stats.ttft_hint');

  const rows = useMemo<RequestEventTileRow[]>(() => {
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
      const apiKey = String(event.api_key ?? '').trim() || '-';
      const model = String(event.model ?? '').trim() || '-';
      const reasoningEffort = String(event.reasoning_effort ?? '').trim() || '-';
      const endpointFields = parseRequestEndpoint(event.endpoint);
      const inputTokens = Math.max(toNumber(event.tokens?.input_tokens), 0);
      const outputTokens = Math.max(toNumber(event.tokens?.output_tokens), 0);
      const reasoningTokens = Math.max(toNumber(event.tokens?.reasoning_tokens), 0);
      const cachedTokens = Math.max(toNumber(event.tokens?.cached_tokens), 0);
      const totalTokens = Math.max(toNumber(event.tokens?.total_tokens), 0);
      const latencyMs = Number.isFinite(event.latency_ms) ? event.latency_ms : null;
      const ttftMs = Number.isFinite(event.ttft_ms) ? event.ttft_ms as number : null;
      // 费用由后端按当前价格配置运行时计算，前端只负责展示可用/不可用状态。
      const costAvailable = event.cost_available === true;
      const cost = costAvailable ? Math.max(toNumber(event.cost_usd), 0) : null;

      return {
        id: usageEventID || `${timestamp}-${model}-${sourceRaw || source}-${authIndex}-${index}`,
        usageEventID,
        requestID,
        timestamp,
        timestampMs: Number.isNaN(timestampMs) ? 0 : timestampMs,
        timestampLabel: formatRequestEventTimestamp(timestamp),
        apiKey,
        model,
        reasoningEffort,
        requestType: endpointFields.requestType,
        endpoint: endpointFields.endpoint,
        sourceRaw: sourceRaw || '-',
        source,
        sourceType,
        authIndex,
        isDelete: event.isDelete === true,
        failed: event.failed === true,
        latencyMs,
        ttftMs,
        inputTokens,
        outputTokens,
        reasoningTokens,
        cachedTokens,
        totalTokens,
        cacheRate: formatCacheRateForSource(cachedTokens, inputTokens, sourceType),
        cost,
        costAvailable,
        attempts: mapRequestEventAttempts(event.attempts),
        attemptCount: Math.max(toNumber(event.attempt_count), 1),
      };
    });
  }, [events]);

  const hasLatencyData = useMemo(() => rows.some((row) => row.latencyMs !== null), [rows]);
  const hasTTFTData = true;

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

  const canOpenRequestDetail = (row: RequestEventTileRow): boolean => Boolean(row.usageEventID && row.requestID);

  const handleOpenRequestDetail = (row: RequestEventTileRow) => {
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
        <RequestEventsTable
          rows={rows}
          canOpenDetail={canOpenRequestDetail}
          showLatency={hasLatencyData}
          showTTFT={hasTTFTData}
          latencyHint={latencyHint}
          ttftHint={ttftHint}
          t={t}
          onOpenDetail={handleOpenRequestDetail}
          footer={(
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
          )}
        />
      )}
    </Card>
  );
}
