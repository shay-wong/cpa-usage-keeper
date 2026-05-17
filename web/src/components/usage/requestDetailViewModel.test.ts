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

  it('skips structured parsing for oversized content', () => {
    const model = buildRequestDetailViewModel('x'.repeat(1024 * 1024 + 1));

    expect(model.kind).toBe('oversized');
    expect(model.parsedSummary).toBe('oversized');
  });
});
