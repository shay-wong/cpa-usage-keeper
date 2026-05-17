// MAX_STRUCTURED_DETAIL_BYTES 限制前端结构化解析体积，避免大日志在浏览器中触发明显卡顿。
const MAX_STRUCTURED_DETAIL_BYTES = 1024 * 1024;
// HEADER_LINE_PATTERN 只识别常见 `Key: value` 头部行，未知格式保留 raw fallback。
const HEADER_LINE_PATTERN = /^([^:\s][^:]*):\s*(.*)$/;
// HTTP_REQUEST_LINE_PATTERN 覆盖常见 HTTP request line，供详情页提取方法和路径。
const HTTP_REQUEST_LINE_PATTERN = /^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(\S+)(?:\s+HTTP\/\d(?:\.\d)?)?$/i;
// HTTP_STATUS_LINE_PATTERN 覆盖常见 HTTP status line，供详情页提取响应状态。
const HTTP_STATUS_LINE_PATTERN = /^HTTP\/\d(?:\.\d)?\s+(\d{3})(?:\s+(.*))?$/i;
// DURATION_PATTERN 从松散日志文本里提取常见耗时字段，失败时不影响 raw 展示。
const DURATION_PATTERN = /(?:duration|latency|elapsed)[=:]\s*([\d.]+\s*(?:ms|s|m))/i;
// MODEL_PATTERN 从松散日志文本里提取模型字段，优先级低于 HTTP header。
const MODEL_PATTERN = /(?:model|x-model)[:=]\s*([\w.:-]+)/i;

export type RequestDetailSectionKind = 'json' | 'http' | 'raw' | 'oversized';

export interface RequestDetailHeaderPair {
  key: string;
  value: string;
}

export interface RequestDetailViewModel {
  kind: RequestDetailSectionKind;
  method?: string;
  path?: string;
  status?: string;
  duration?: string;
  model?: string;
  requestHeaders: RequestDetailHeaderPair[];
  responseHeaders: RequestDetailHeaderPair[];
  requestBody?: string;
  responseBody?: string;
  raw: string;
  parsedSummary: string;
}

interface ParsedHttpBlock {
  line: string;
  headers: RequestDetailHeaderPair[];
  body: string;
}

const byteLength = (value: string): number => new TextEncoder().encode(value).length;

const prettyPrintJSON = (value: string): string | null => {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return null;
  }
};

const bodyForDisplay = (value: string): string => {
  const trimmed = value.trim();
  const json = prettyPrintJSON(trimmed);
  if (json !== null) return json;
  const [firstLine, ...rest] = trimmed.split('\n');
  const firstLineJSON = prettyPrintJSON(firstLine.trim());
  if (firstLineJSON !== null) {
    return rest.length > 0 ? `${firstLineJSON}\n${rest.join('\n').trim()}` : firstLineJSON;
  }
  return trimmed;
};

const parseHeaderBlock = (lines: string[]): RequestDetailHeaderPair[] => {
  const headers: RequestDetailHeaderPair[] = [];
  for (const line of lines) {
    const match = line.match(HEADER_LINE_PATTERN);
    if (!match) continue;
    headers.push({ key: match[1].trim(), value: match[2].trim() });
  }
  return headers;
};

const parseHttpBlock = (block: string): ParsedHttpBlock => {
  const normalized = block.replace(/\r\n/g, '\n');
  const separator = normalized.indexOf('\n\n');
  const head = separator >= 0 ? normalized.slice(0, separator) : normalized;
  const body = separator >= 0 ? normalized.slice(separator + 2) : '';
  const lines = head.split('\n').map((line) => line.trim()).filter(Boolean);
  return {
    line: lines[0] ?? '',
    headers: parseHeaderBlock(lines.slice(1)),
    body: bodyForDisplay(body),
  };
};

const getHeaderValue = (headers: RequestDetailHeaderPair[], key: string): string | undefined => {
  const matched = headers.find((header) => header.key.toLowerCase() === key.toLowerCase());
  return matched?.value;
};

const firstMatch = (content: string, pattern: RegExp): string | undefined => {
  const match = content.match(pattern);
  return match?.[1]?.trim();
};

export const buildRequestDetailViewModel = (content: string): RequestDetailViewModel => {
  const raw = content ?? '';
  if (byteLength(raw) > MAX_STRUCTURED_DETAIL_BYTES) {
    return {
      kind: 'oversized',
      requestHeaders: [],
      responseHeaders: [],
      raw,
      parsedSummary: 'oversized',
    };
  }

  const json = prettyPrintJSON(raw.trim());
  if (json !== null) {
    return {
      kind: 'json',
      requestHeaders: [],
      responseHeaders: [],
      requestBody: json,
      raw,
      parsedSummary: 'json',
    };
  }

  const normalized = raw.replace(/\r\n/g, '\n').trim();
  const blocks = normalized.split(/\n{2,}(?=(?:HTTP\/\d(?:\.\d)?\s+\d{3}|GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\b)/i);
  const requestBlock = blocks.find((block) => HTTP_REQUEST_LINE_PATTERN.test(block.split('\n')[0]?.trim() ?? ''));
  const responseBlock = blocks.find((block) => HTTP_STATUS_LINE_PATTERN.test(block.split('\n')[0]?.trim() ?? ''));

  if (requestBlock || responseBlock) {
    const request = requestBlock ? parseHttpBlock(requestBlock) : undefined;
    const response = responseBlock ? parseHttpBlock(responseBlock) : undefined;
    const requestLine = request?.line.match(HTTP_REQUEST_LINE_PATTERN);
    const responseLine = response?.line.match(HTTP_STATUS_LINE_PATTERN);
    const status = responseLine ? `${responseLine[1]}${responseLine[2] ? ` ${responseLine[2]}` : ''}` : undefined;
    const model = getHeaderValue(request?.headers ?? [], 'x-model') ?? getHeaderValue(response?.headers ?? [], 'x-model') ?? firstMatch(raw, MODEL_PATTERN);

    return {
      kind: 'http',
      method: requestLine?.[1]?.toUpperCase(),
      path: requestLine?.[2],
      status,
      duration: firstMatch(raw, DURATION_PATTERN),
      model,
      requestHeaders: request?.headers ?? [],
      responseHeaders: response?.headers ?? [],
      requestBody: request?.body,
      responseBody: response?.body,
      raw,
      parsedSummary: 'http',
    };
  }

  return {
    kind: 'raw',
    requestHeaders: [],
    responseHeaders: [],
    raw,
    parsedSummary: 'raw',
  };
};
