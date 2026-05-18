import React from 'react';
import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { RequestDetailStructuredView, RequestEventsDetailsCard } from './RequestEventsDetailsCard';
import { requestDetailBodyToJsonValue } from './RequestDetailJsonViewer';
import { buildRequestDetailViewModel } from './requestDetailViewModel';
import type { UsageEvent } from '@/lib/types';

const events: UsageEvent[] = [
  {
    id: '101',
    timestamp: '2026-04-23T02:00:00.000Z',
    model: 'claude-sonnet',
    source: 'Provider A',
    source_raw: 'source-a',
    source_type: 'openai',
    auth_index: '1',
    failed: false,
    latency_ms: 120,
    tokens: {
      input_tokens: 100,
      output_tokens: 60,
      reasoning_tokens: 20,
      cached_tokens: 20,
      total_tokens: 200,
    },
  },
];

const renderCard = (props: Partial<React.ComponentProps<typeof RequestEventsDetailsCard>> = {}) =>
  renderToStaticMarkup(
    <RequestEventsDetailsCard
      events={events}
      loading={false}
      page={1}
      pageSize={20}
      pageSizeOptions={[20, 50, 100, 500, 1000]}
      totalCount={120}
      totalPages={6}
      modelOptions={['claude-sonnet', 'claude-opus']}
      sourceOptions={[{ value: 'source-a', label: 'Provider A' }, { value: 'source-b', label: 'Provider B' }]}
      modelFilter="__all__"
      sourceFilter="__all__"
      resultFilter="__all__"
      modelPrices={{}}
      onPageChange={() => undefined}
      onPageSizeChange={() => undefined}
      onModelFilterChange={() => undefined}
      onSourceFilterChange={() => undefined}
      onResultFilterChange={() => undefined}
      {...props}
    />,
  );

const countOccurrences = (text: string, value: string) => text.split(value).length - 1;

describe('RequestEventsDetailsCard pagination', () => {
  it('renders total events, current page, page size options, and disabled page buttons', () => {
    const html = renderCard();

    expect(html).toContain('120 total events');
    expect(html).toContain('1 / 6');
    expect(html).toContain('20');
    expect(html).toContain('50');
    expect(html).toContain('100');
    expect(html).toContain('500');
    expect(html).toContain('1000');
    expect(html).toContain('Previous');
    expect(html).toContain('Next');
    expect(html).toContain('disabled');
  });

  it('formats timestamps with compact numeric date and time', () => {
    const html = renderCard({
      events: [{ ...events[0], timestamp: '2026-05-13T00:38:19+08:00' }],
    });

    expect(html).toContain('2026/05/13 00:38:19');
    expect(html).not.toContain('5/13/2026, 12:38:19 AM');
  });

  it('renders cache rate after cached tokens with two decimal places', () => {
    const html = renderCard({
      events: [{ ...events[0], tokens: { ...events[0].tokens, input_tokens: 100, cached_tokens: 25 } }],
    });

    expect(html.indexOf('<th>Cached</th>')).toBeLessThan(html.indexOf('<th>Cache Rate</th>'));
    expect(html.indexOf('<th>Cache Rate</th>')).toBeLessThan(html.indexOf('<th>Total Tokens</th>'));
    expect(html).toContain('<td>25</td><td>25.00%</td><td>200</td>');
  });

  it('uses Claude token semantics for cache rate', () => {
    const html = renderCard({
      events: [{
        ...events[0],
        source_type: 'claude',
        tokens: { ...events[0].tokens, input_tokens: 400, cached_tokens: 600, total_tokens: 500 },
      }],
    });

    expect(html).toContain('<td>600</td><td>60.00%</td><td>500</td>');
    expect(html).not.toContain('150.00%');
  });

  it('shows a dash for cache rate when input tokens are zero', () => {
    const html = renderCard({
      events: [{ ...events[0], tokens: { ...events[0].tokens, input_tokens: 0, cached_tokens: 25 } }],
    });

    expect(html).toContain('<td>0</td><td>60</td><td>20</td><td>25</td><td>-</td><td>200</td>');
  });

  it('stacks source value above source tags', () => {
    const html = renderCard({
      events: [{ ...events[0], isDelete: true }],
    });

    expect(html).toContain('_requestEventsSourceStack_');
    expect(html).toContain('_requestEventsSourceValue_');
    expect(html).toContain('_requestEventsSourceTags_');
    expect(html).toContain('_requestEventsDeletedTag_');
    expect(html).toContain('Provider A');
    expect(html).toContain('openai');
    expect(html).toContain('Deleted');
  });

  it('uses backend source values while showing resolved source labels', () => {
    const html = renderCard({
      sourceFilter: 'source-a',
      sourceOptions: [{ value: 'source-a', label: 'Provider A', displayName: 'Provider A(Team Prefix)' }, { value: 'source-b', label: 'Provider B' }],
    });

    expect(countOccurrences(html, 'Provider A(Team Prefix)')).toBeGreaterThanOrEqual(1);
    expect(html).toContain('aria-label="Source"><span class="_triggerText_c80422 ">Provider A(Team Prefix)</span>');
  });

  it('uses backend model and source options instead of current page grouping', () => {
    const html = renderCard({ modelFilter: 'claude-opus', sourceFilter: 'source-b' });

    expect(html).toContain('aria-label="Model"><span class="_triggerText_c80422 ">claude-opus</span>');
    expect(html).toContain('aria-label="Source"><span class="_triggerText_c80422 ">Provider B</span>');
  });

  it('renders a Result filter and no Credential filter control', () => {
    const html = renderCard({ resultFilter: 'failed' });

    expect(html).toContain('aria-label="Result"');
    expect(html).toContain('Failure');
    expect(html).not.toContain('aria-label="Credential"');
  });

  it('keeps selected filters visible when backend options do not include them', () => {
    const html = renderCard({
      modelFilter: 'claude-haiku',
      sourceFilter: 'source-c',
    });

    expect(html).toContain('claude-haiku');
    expect(html).toContain('source-c');
  });

  it('falls back to a computed page count when metadata is not populated', () => {
    const html = renderCard({ totalPages: 0, totalCount: 120, pageSize: 20 });

    expect(html).toContain('1 / 6');
  });

  it('shows total count in the title and uses the shared pager footer', () => {
    const html = renderCard();

    expect(html).toContain('_requestEventsFiltersGroup_');
    expect(html).toContain('_requestEventsTitleRow_');
    expect(html).toContain('_requestEventsCountBadge_');
    expect(html).toContain('120 total events');
    expect(html).toContain('_requestEventsPaginationFooter_');
    expect(html).toContain('_requestEventsPaginationControls_');
    expect(html).toContain('_requestEventsPageSizeControl_');
    expect(html).toContain('Size');
    expect(html).not.toContain('Rows per page');
    expect(html).toContain('_requestEventsPaginationPage_');
    expect(html).toContain('_requestEventsPagerButton_');
    expect(html).toContain('<select');
    expect(html).toContain('value="20"');
    expect(html).toContain('_requestEventsActions_');
    expect(html).not.toContain('_requestEventsPaginationItem_');
    expect(html).not.toContain('_requestEventsPageSizeSelectCompact_');
    expect(html).not.toContain('_usagePillShell_');
    expect(html).not.toContain('_requestEventsTableMeta_');
    expect(html).not.toContain('_requestEventsCountGroup_');
    expect(html).not.toContain('_requestEventsLimitHint_');
  });

  it('hides export buttons while keeping clear filters available', () => {
    const html = renderCard({ modelFilter: 'claude-sonnet' });

    expect(html).toContain('Clear Filters');
    expect(html).not.toContain('Export CSV');
    expect(html).not.toContain('Export JSON');
  });


  it('marks rows with request ids as clickable detail entries', () => {
    const html = renderCard({
      events: [{ ...events[0], request_id: 'req-101' }],
    });

    expect(html).toContain('role="button"');
    expect(html).toContain('tabindex="0"');
    expect(html).toContain('View request detail for req-101');
    expect(html).toContain('_requestEventsClickableRow_');
  });

  it('does not mark rows without request ids as clickable', () => {
    const html = renderCard();

    expect(html).not.toContain('role="button"');
    expect(html).not.toContain('tabindex="0"');
    expect(html).not.toContain('View request detail');
    expect(html).not.toContain('_requestEventsClickableRow_');
  });

  it('separates parsed and raw JSON request detail pages', () => {
    const content = JSON.stringify({
      request: { method: 'POST', path: '/v1/messages' },
      body: { model: 'claude-sonnet', messages: [{ role: 'user', content: 'hello' }] },
    });
    const model = buildRequestDetailViewModel(content);
    const html = renderToStaticMarkup(
      <RequestDetailStructuredView
        detail={{ request_id: 'req-json', content, fetched_at: '2026-04-23T02:00:00.000Z', cached: true }}
        model={model}
        t={(key) => key}
      />,
    );

    expect(html).toContain('_requestEventsDetailPageTabs_');
    expect(html).toContain('_requestEventsDetailPageTabActive_');
    expect(html).toContain('request_events_detail_parsed_page');
    expect(html).toContain('request_events_detail_raw_page');
    expect(html).toContain('_requestEventsDetailTrafficTabs_');
    expect(html).toContain('request_events_detail_request_section');
    expect(html).toContain('request_events_detail_response_section');
    expect(html).toContain('request_events_detail_request_headers');
    expect(html).toContain('request_events_detail_request_body');
    expect(html).toContain('_requestEventsDetailJsonBlock_');
    expect(html).toContain('_requestEventsDetailJsonExplorer_');
    expect(html).toContain('_requestEventsDetailJsonViewer_');
    expect(html).toContain('_requestEventsDetailJsonToggle_');
    expect(html).toContain('_requestEventsDetailJsonBracket_');
    expect(html).toContain('aria-expanded="true"');
    expect(html).toContain('usage_stats.request_events_detail_json_copy_node');
    expect(html).not.toContain('request_events_detail_raw_log');
    expect(html).not.toContain('<details');
    expect(html).not.toContain('{&quot;request&quot;:{&quot;method&quot;');
  });

  it('renders request headers and body through collapsible JSON viewers', () => {
    const content = [
      'POST /v1/messages HTTP/1.1',
      'Content-Type: application/json',
      'X-Model: claude-sonnet',
      '',
      JSON.stringify({ prompt: 'hello', stream: false }),
      '',
      'HTTP/1.1 200 OK',
      'X-Request-ID: req-upstream',
      '',
      'plain response body',
    ].join('\n');
    const model = buildRequestDetailViewModel(content);
    const html = renderToStaticMarkup(
      <RequestDetailStructuredView
        detail={{ request_id: 'req-log', content, fetched_at: '2026-04-23T02:00:00.000Z', cached: true }}
        model={model}
        t={(key) => key}
      />,
    );

    expect(countOccurrences(html, '_requestEventsDetailJsonViewer_')).toBeGreaterThanOrEqual(2);
    expect(html).toContain('request_headers');
    expect(html).toContain('request_body');
    expect(html).toContain('Content-Type');
    expect(html).toContain('prompt');
    expect(html).toContain('stream');
    expect(html).toContain('request_events_detail_response_section');
    expect(html).not.toContain('_requestEventsDetailLogSections_');
    expect(html).not.toContain('POST /v1/messages HTTP/1.1');
    expect(html).not.toContain('plain response body');
  });

  it('renders raw request details through a non-wrapping readonly textarea', () => {
    const content = 'raw log line '.repeat(20);
    const model = buildRequestDetailViewModel(content);
    const html = renderToStaticMarkup(
      <RequestDetailStructuredView
        detail={{ request_id: 'req-raw', content, fetched_at: '2026-04-23T02:00:00.000Z', cached: true }}
        model={model}
        t={(key) => key}
      />,
    );

    expect(html).toContain('<textarea');
    expect(html).toContain('wrap="off"');
    expect(html).toMatch(/<textarea[^>]*readonly/i);
    expect(html).toContain('raw log line raw log line');
    expect(html).not.toContain('<pre');
  });

  it('summarizes response event-stream data lines into human-readable JSON values', () => {
    const value = requestDetailBodyToJsonValue([
      'event: response.created',
      'data: {"type":"response.created","response":{"id":"resp_1","object":"response","created_at":1779080877,"status":"in_progress","model":"gpt-4.1","instructions":"large system prompt","input":[{"role":"user","content":"hello"}]}}',
      '',
      'event: response.output_text.delta',
      'data: {"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"hello world"}',
      '',
      'event: response.completed',
      'data: {"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[{"type":"tool_use","arguments":"large tool payload"}],"usage":{"input_tokens":1,"output_tokens":2}}}',
    ].join('\n'));

    expect(value).toEqual({
      stream_type: 'event-stream',
      event_count: 3,
      output_text: 'hello world',
      events: [
        {
          event: 'response.created',
          data: {
            type: 'response.created',
            response: {
              id: 'resp_1',
              object: 'response',
              status: 'in_progress',
              model: 'gpt-4.1',
              created_at: 1779080877,
            },
          },
        },
        {
          event: 'response.output_text.delta',
          data: { type: 'response.output_text.delta', output_index: 0, content_index: 0, delta: 'hello world' },
        },
        {
          event: 'response.completed',
          data: {
            type: 'response.completed',
            response: { id: 'resp_1', status: 'completed', usage: { input_tokens: 1, output_tokens: 2 } },
          },
        },
      ],
    });
    expect(JSON.stringify(value)).not.toContain('large system prompt');
    expect(JSON.stringify(value)).not.toContain('large tool payload');
  });


  it('keeps oversized JSON bodies as raw strings in the parsed viewer', () => {
    const content = `{"payload":"${'x'.repeat(2 * 1024 * 1024 + 1)}"}`;
    const value = requestDetailBodyToJsonValue(content);

    expect(value).toBe(content);
  });

  it('shows per-event cost when model pricing exists', () => {
    const html = renderCard({
      modelPrices: {
        'claude-sonnet': { prompt: 15, completion: 75, cache: 1.5 },
      },
    });

    expect(html).toContain('Total Cost');
    expect(html).toContain('$0.0057');
  });
});
