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
// JSON_PARSE_MAX_CHARS 限制同步 JSON.parse 的输入规模，避免超大 raw log 阻塞主线程。
const JSON_PARSE_MAX_CHARS = 2 * 1024 * 1024;

export type RequestDetailSectionKind = 'json' | 'http' | 'raw';

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

interface RequestDetailNamedSection {
  title: string;
  content: string;
}

interface ParsedSectionedResponse {
  status?: string;
  headers: RequestDetailHeaderPair[];
  body?: string;
}

const prettyPrintJSON = (value: string): string | null => {
  if (value.length > JSON_PARSE_MAX_CHARS) return null;

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


const splitNamedSections = (content: string): RequestDetailNamedSection[] => {
  const normalized = content.replace(/\r\n/g, '\n');
  const matches = [...normalized.matchAll(/^===\s*([^=]+?)\s*===\s*$/gm)];
  return matches.map((match, index) => {
    const start = (match.index ?? 0) + match[0].length;
    const end = matches[index + 1]?.index ?? normalized.length;
    return {
      title: match[1].trim().toUpperCase(),
      content: normalized.slice(start, end).trim(),
    };
  });
};

const getNamedSection = (sections: RequestDetailNamedSection[], title: string): string | undefined => {
  return sections.find((section) => section.title === title)?.content;
};

const parseSectionedResponse = (content: string | undefined): ParsedSectionedResponse => {
  if (!content) return { headers: [] };

  const normalized = content.replace(/\r\n/g, '\n');
  const separator = normalized.indexOf('\n\n');
  const head = separator >= 0 ? normalized.slice(0, separator) : normalized;
  const body = separator >= 0 ? normalized.slice(separator + 2) : '';
  const lines = head.split('\n').map((line) => line.trim()).filter(Boolean);
  const statusLine = lines.find((line) => /^Status:/i.test(line));
  const status = statusLine?.replace(/^Status:\s*/i, '').trim();
  const headerLines = lines.filter((line) => !/^Status:/i.test(line));

  return {
    status,
    headers: parseHeaderBlock(headerLines),
    body: body ? bodyForDisplay(body) : undefined,
  };
};

const getJSONModelName = (body: string | undefined): string | undefined => {
  if (!body || body.length > JSON_PARSE_MAX_CHARS) return undefined;

  try {
    const parsed = JSON.parse(body) as unknown;
    if (typeof parsed === 'object' && parsed !== null && 'model' in parsed) {
      const model = (parsed as { model?: unknown }).model;
      return typeof model === 'string' ? model : undefined;
    }
  } catch {
    return undefined;
  }
  return undefined;
};

const parseSectionedLog = (raw: string): RequestDetailViewModel | null => {
  const sections = splitNamedSections(raw);
  const requestInfo = getNamedSection(sections, 'REQUEST INFO');
  const requestBody = getNamedSection(sections, 'REQUEST BODY');
  const requestHeaders = parseHeaderBlock((getNamedSection(sections, 'HEADERS') ?? '').split('\n'));
  const response = parseSectionedResponse(getNamedSection(sections, 'RESPONSE'));
  if (!requestInfo && !requestBody && requestHeaders.length === 0 && !response.body) return null;

  const requestInfoPairs = parseHeaderBlock((requestInfo ?? '').split('\n'));
  const model = getJSONModelName(requestBody) ?? firstMatch(raw, MODEL_PATTERN);

  return {
    kind: 'http',
    method: getHeaderValue(requestInfoPairs, 'Method')?.toUpperCase(),
    path: getHeaderValue(requestInfoPairs, 'URL'),
    status: response.status,
    duration: firstMatch(raw, DURATION_PATTERN),
    model,
    requestHeaders,
    responseHeaders: response.headers,
    requestBody: requestBody ? bodyForDisplay(requestBody) : undefined,
    responseBody: response.body,
    raw,
    parsedSummary: 'sectioned',
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

  const sectionedLog = parseSectionedLog(raw);
  if (sectionedLog) return sectionedLog;

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
