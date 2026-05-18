import { describe, expect, it } from 'vitest';
import { buildRequestDetailViewModel } from './requestDetailViewModel';

describe('buildRequestDetailViewModel', () => {
  it('pretty prints JSON detail content', () => {
    const model = buildRequestDetailViewModel('{"request":{"model":"claude-sonnet"},"ok":true}');

    expect(model.kind).toBe('json');
    expect(model.requestBody).toContain('"model": "claude-sonnet"');
    expect(model.requestBody).toContain('"ok": true');
  });

  it('extracts request and response sections from HTTP transcript content', () => {
    const model = buildRequestDetailViewModel([
      'POST /v1/messages HTTP/1.1',
      'Content-Type: application/json',
      'X-Model: claude-sonnet',
      '',
      '{"prompt":"hello"}',
      '',
      'HTTP/1.1 200 OK',
      'Content-Type: application/json',
      '',
      '{"id":"msg_1"}',
      'duration=42ms',
    ].join('\n'));

    expect(model.kind).toBe('http');
    expect(model.method).toBe('POST');
    expect(model.path).toBe('/v1/messages');
    expect(model.status).toBe('200 OK');
    expect(model.model).toBe('claude-sonnet');
    expect(model.duration).toBe('42ms');
    expect(model.requestHeaders).toContainEqual({ key: 'Content-Type', value: 'application/json' });
    expect(model.requestBody).toContain('"prompt": "hello"');
    expect(model.responseBody).toContain('"id": "msg_1"');
  });


  it('extracts CPAMC sectioned logs into request and response JSON-ready fields', () => {
    const model = buildRequestDetailViewModel([
      '=== REQUEST INFO ===',
      'URL: /v1/responses',
      'Method: POST',
      '',
      '=== HEADERS ===',
      'Content-Type: application/json',
      'X-App: cli',
      '',
      '=== REQUEST BODY ===',
      '{"model":"gpt-5.5","input":"hello"}',
      '',
      '=== RESPONSE ===',
      'Status: 200',
      'Content-Type: text/event-stream',
      '',
      'event: done',
    ].join('\n'));

    expect(model.kind).toBe('http');
    expect(model.method).toBe('POST');
    expect(model.path).toBe('/v1/responses');
    expect(model.status).toBe('200');
    expect(model.model).toBe('gpt-5.5');
    expect(model.requestHeaders).toContainEqual({ key: 'Content-Type', value: 'application/json' });
    expect(model.responseHeaders).toContainEqual({ key: 'Content-Type', value: 'text/event-stream' });
    expect(model.requestBody).toContain('"model": "gpt-5.5"');
    expect(model.responseBody).toBe('event: done');
  });

  it('falls back to raw mode for unknown text', () => {
    const model = buildRequestDetailViewModel('plain upstream log');

    expect(model.kind).toBe('raw');
    expect(model.raw).toBe('plain upstream log');
  });

  it('keeps HTML-looking content as text data', () => {
    const model = buildRequestDetailViewModel('<script>alert(1)</script>');

    expect(model.kind).toBe('raw');
    expect(model.raw).toBe('<script>alert(1)</script>');
  });


  it('does not synchronously parse oversized raw JSON before sectioned parsing', () => {
    const content = `{"payload":"${'x'.repeat(2 * 1024 * 1024 + 1)}"}`;
    const model = buildRequestDetailViewModel(content);

    expect(model.kind).toBe('raw');
    expect(model.raw).toBe(content);
  });

  it('does not parse oversized sectioned JSON bodies just to extract model names', () => {
    const largeBody = `{"model":"gpt-5.5","payload":"${'x'.repeat(2 * 1024 * 1024 + 1)}"}`;
    const model = buildRequestDetailViewModel([
      '=== REQUEST INFO ===',
      'URL: /v1/responses',
      'Method: POST',
      '',
      '=== REQUEST BODY ===',
      largeBody,
      '',
      '=== RESPONSE ===',
      'Status: 200',
    ].join('\n'));

    expect(model.kind).toBe('http');
    expect(model.model).toBeUndefined();
    expect(model.requestBody).toBe(largeBody);
  });

  it('keeps parsing sectioned logs even when the raw log is large', () => {
    const largeBody = 'x'.repeat(5 * 1024 * 1024 + 1);
    const model = buildRequestDetailViewModel([
      '=== REQUEST INFO ===',
      'URL: /v1/responses',
      'Method: POST',
      '',
      '=== HEADERS ===',
      'Content-Type: application/json',
      '',
      '=== REQUEST BODY ===',
      largeBody,
      '',
      '=== RESPONSE ===',
      'Status: 200',
    ].join('\n'));

    expect(model.kind).toBe('http');
    expect(model.parsedSummary).toBe('sectioned');
    expect(model.requestBody).toBe(largeBody);
  });
});
