import React, { useMemo, useState } from 'react';
import styles from '@/pages/UsagePage.module.scss';
import type { RequestDetailHeaderPair } from './requestDetailViewModel';

export type JsonValue = null | boolean | number | string | JsonValue[] | { [key: string]: JsonValue };
type JsonObject = { [key: string]: JsonValue };

// JSON_TREE_MAX_DEPTH 控制树渲染深度，深层内容仍可在 raw log 中查看。
const JSON_TREE_MAX_DEPTH = 8;
// JSON_TREE_MAX_CHILDREN 控制单层展示数量，避免超大数组一次性撑爆 DOM。
const JSON_TREE_MAX_CHILDREN = 80;
// JSON_SCALAR_PREVIEW_LENGTH 控制长字符串展示长度，完整内容保留在 raw log。
const JSON_SCALAR_PREVIEW_LENGTH = 2_000;
// JSON_PARSE_MAX_CHARS 限制同步 JSON.parse 的输入规模，避免超大响应体阻塞 Parsed 页。
const JSON_PARSE_MAX_CHARS = 2 * 1024 * 1024;

interface ParsedJsonValue {
  value: JsonValue;
}

export interface RequestDetailJsonLabels {
  collapseNode: string;
  copiedNode: string;
  copyNode: string;
  expandNode: string;
  jsonString: string;
  parseString: string;
  rawString: string;
}

interface RequestDetailJsonContextValue {
  expandDepth: number | 'all';
  globalStringExpanded: boolean;
  hideArrayIndices: boolean;
  labels: RequestDetailJsonLabels;
}

const REQUEST_DETAIL_JSON_DEFAULT_LABELS: RequestDetailJsonLabels = {
  collapseNode: 'Collapse JSON node',
  copiedNode: 'Copied',
  copyNode: 'Copy JSON node',
  expandNode: 'Expand JSON node',
  jsonString: 'JSON',
  parseString: 'parse',
  rawString: 'raw',
};

const RequestDetailJsonContext = React.createContext<RequestDetailJsonContextValue>({
  expandDepth: 2,
  globalStringExpanded: false,
  hideArrayIndices: false,
  labels: REQUEST_DETAIL_JSON_DEFAULT_LABELS,
});

const parseJSONForDisplay = (value: string | undefined): ParsedJsonValue | null => {
  if (!value || value.length > JSON_PARSE_MAX_CHARS) return null;

  try {
    return { value: JSON.parse(value) as JsonValue };
  } catch {
    return null;
  }
};

const isJsonObject = (value: JsonValue): value is JsonObject => {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
};

const isJsonExpandable = (value: JsonValue): value is JsonValue[] | { [key: string]: JsonValue } => {
  return Array.isArray(value) || isJsonObject(value);
};

const getJsonEntries = (value: JsonValue): Array<readonly [string, JsonValue]> => {
  if (Array.isArray(value)) {
    return value.map((item, index) => [String(index), item] as const);
  }
  if (isJsonObject(value)) return Object.entries(value);
  return [];
};

const getJsonBracketPair = (value: JsonValue): readonly ['[' | '{', ']' | '}'] => {
  return Array.isArray(value) ? ['[', ']'] : ['{', '}'];
};

const getJsonItemSummary = (count: number): string => {
  return `${count} ${count === 1 ? 'item' : 'items'}`;
};

const formatJsonStringForDisplay = (value: string): string => {
  const normalized = normalizeJsonStringForDisplay(value);
  return normalized.length > JSON_SCALAR_PREVIEW_LENGTH
    ? `${normalized.slice(0, JSON_SCALAR_PREVIEW_LENGTH)}…`
    : normalized;
};

const normalizeJsonStringForDisplay = (value: string): string => {
  const lines = value.replace(/\t/g, '  ').split('\n');
  if (lines.length <= 1) return value;

  const indents = lines
    .slice(1)
    .filter((line) => line.trim().length > 0)
    .map((line) => line.match(/^\s+/u)?.[0].length ?? 0)
    .filter((indent) => indent > 0);
  if (!indents.length) return value;

  const minIndent = Math.min(...indents);
  return lines
    .map((line, index) => {
      if (index === 0 || line.trim().length === 0) return line;
      const dedented = line.replace(new RegExp(`^\\s{0,${minIndent}}`, 'u'), '');
      const extraIndent = dedented.match(/^\s+/u)?.[0].length ?? 0;
      return extraIndent > 2 ? `  ${dedented.trimStart()}` : dedented;
    })
    .join('\n');
};

const parseJSONStringValue = (value: string): ParsedJsonValue | null => {
  const trimmed = value.trim();
  if (!((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']')))) {
    return null;
  }
  return parseJSONForDisplay(trimmed);
};

interface EventStreamEntry {
  data: JsonValue;
  event?: string;
  id?: string;
  retry?: string;
}

// RESPONSE_EVENT_RESPONSE_SUMMARY_KEYS 只保留 response 对象中的轻量字段，完整 payload 保留在 Raw 页。
const RESPONSE_EVENT_RESPONSE_SUMMARY_KEYS = ['id', 'object', 'status', 'model', 'created_at', 'completed_at', 'error', 'incomplete_details', 'usage'] as const;

// RESPONSE_EVENT_DATA_SUMMARY_KEYS 保留流式事件定位与增量文本字段，避免展开 instructions/input/output 等大对象。
const RESPONSE_EVENT_DATA_SUMMARY_KEYS = ['type', 'sequence_number', 'output_index', 'content_index', 'item_id', 'call_id', 'delta', 'text', 'error'] as const;

const parseEventStreamDataLine = (line: string): string | null => {
  const match = line.match(/^data:\s?(.*)$/u);
  return match ? match[1] : null;
};

const parseEventStreamField = (lines: string[], field: string): string | undefined => {
  const prefix = `${field}:`;
  return lines.find((line) => line.startsWith(prefix))?.slice(prefix.length).trim();
};

const pickJsonFields = (value: JsonObject, keys: readonly string[]): JsonObject => {
  return Object.fromEntries(
    keys.flatMap((key) => {
      const fieldValue = value[key];
      return fieldValue === undefined ? [] : [[key, fieldValue] as const];
    })
  );
};

const parseEventStreamPayload = (value: string): JsonValue => {
  const parsedJson = parseJSONForDisplay(value.trim());
  return parsedJson?.value ?? value;
};

const summarizeResponseEventPayload = (value: JsonValue): JsonValue => {
  if (!isJsonObject(value)) return value;

  const response = value.response;
  const responseSummary = isJsonObject(response)
    ? pickJsonFields(response, RESPONSE_EVENT_RESPONSE_SUMMARY_KEYS)
    : undefined;
  const dataSummary = pickJsonFields(value, RESPONSE_EVENT_DATA_SUMMARY_KEYS);

  if (responseSummary) {
    return { ...dataSummary, response: responseSummary };
  }
  return Object.keys(dataSummary).length > 0 ? dataSummary : value;
};

const parseEventStreamBlock = (block: string): EventStreamEntry | null => {
  const lines = block.split('\n').map((line) => line.trimEnd()).filter(Boolean);
  const dataLines = lines.map(parseEventStreamDataLine).filter((line): line is string => line !== null);
  if (dataLines.length === 0) return null;

  const data = parseEventStreamPayload(dataLines.join('\n'));
  const event = parseEventStreamField(lines, 'event');
  const id = parseEventStreamField(lines, 'id');
  const retry = parseEventStreamField(lines, 'retry');

  return {
    ...(event ? { event } : {}),
    ...(id ? { id } : {}),
    ...(retry ? { retry } : {}),
    data,
  };
};

const eventStreamEntryToJsonValue = (entry: EventStreamEntry): JsonValue => ({
  ...(entry.event ? { event: entry.event } : {}),
  ...(entry.id ? { id: entry.id } : {}),
  ...(entry.retry ? { retry: entry.retry } : {}),
  data: summarizeResponseEventPayload(entry.data),
});

const getEventStreamOutputText = (entries: EventStreamEntry[]): string | undefined => {
  const chunks = entries.flatMap((entry) => {
    if (!isJsonObject(entry.data)) return [];
    const delta = entry.data.delta;
    if (typeof delta === 'string') return [delta];

    const text = entry.data.text;
    return typeof text === 'string' && entry.event?.includes('output_text') ? [text] : [];
  });
  const outputText = chunks.join('');
  return outputText || undefined;
};

const summarizeEventStreamEntries = (entries: EventStreamEntry[]): JsonValue => ({
  stream_type: 'event-stream',
  event_count: entries.length,
  ...(getEventStreamOutputText(entries) ? { output_text: getEventStreamOutputText(entries) } : {}),
  events: entries.map(eventStreamEntryToJsonValue),
});

const parseEventStreamDataForDisplay = (value: string | undefined): ParsedJsonValue | null => {
  if (!value || !/^data:/mu.test(value)) return null;

  const blocks = value.replace(/\r\n/g, '\n').split(/\n{2,}/u);
  const entries = blocks.map(parseEventStreamBlock).filter((entry): entry is EventStreamEntry => entry !== null);
  if (entries.length === 0) return null;

  return { value: summarizeEventStreamEntries(entries) };
};

export const requestDetailHeadersToJsonValue = (headers: RequestDetailHeaderPair[]): JsonValue => {
  return Object.fromEntries(headers.map((header) => [header.key, header.value]));
};

export const requestDetailBodyToJsonValue = (value: string | undefined): JsonValue => {
  const parsedJson = parseJSONForDisplay(value) ?? parseEventStreamDataForDisplay(value);
  return parsedJson?.value ?? value ?? '';
};

interface RequestDetailJsonViewerProps {
  value: JsonValue;
  rootName?: string;
  defaultExpanded?: boolean;
  expandDepth?: number | 'all';
  hideArrayIndices?: boolean;
  globalStringExpanded?: boolean;
  labels?: RequestDetailJsonLabels;
}

function RequestDetailJsonViewer({
  value,
  rootName = 'root',
  defaultExpanded = true,
  expandDepth = 'all',
  hideArrayIndices = true,
  globalStringExpanded = false,
  labels = REQUEST_DETAIL_JSON_DEFAULT_LABELS,
}: RequestDetailJsonViewerProps) {
  const contextValue = useMemo(
    () => ({ expandDepth, globalStringExpanded, hideArrayIndices, labels }),
    [expandDepth, globalStringExpanded, hideArrayIndices, labels]
  );

  return (
    <RequestDetailJsonContext.Provider value={contextValue}>
      <div className={styles.requestEventsDetailJsonViewer}>
        <RequestDetailJsonNode name={rootName} value={value} isRoot defaultExpanded={defaultExpanded} />
      </div>
    </RequestDetailJsonContext.Provider>
  );
}

interface RequestDetailJsonNodeProps {
  name: string;
  value: JsonValue;
  isRoot?: boolean;
  isArrayItem?: boolean;
  defaultExpanded?: boolean;
  level?: number;
}

function RequestDetailJsonNode({
  name,
  value,
  isRoot = false,
  isArrayItem = false,
  defaultExpanded = true,
  level = 0,
}: RequestDetailJsonNodeProps) {
  const { expandDepth, hideArrayIndices, labels } = React.useContext(RequestDetailJsonContext);
  const [isExpanded, setIsExpanded] = useState(defaultExpanded);
  const [isCopied, setIsCopied] = useState(false);
  const isExpandable = isJsonExpandable(value);
  const entries = isExpandable ? getJsonEntries(value) : [];
  const visibleEntries = entries.slice(0, JSON_TREE_MAX_CHILDREN);
  const truncatedCount = entries.length - visibleEntries.length;
  const [openBracket, closeBracket] = isExpandable ? getJsonBracketPair(value) : ['{', '}'];
  const childDefaultExpanded = !defaultExpanded
    ? false
    : expandDepth === 'all'
      ? true
      : Array.isArray(value) || level < expandDepth;
  const shouldHideName = isArrayItem && hideArrayIndices;
  const showName = !shouldHideName && name !== '';

  const handleCopy = (event: React.MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    if (typeof navigator === 'undefined' || !navigator.clipboard) return;

    void navigator.clipboard
      .writeText(JSON.stringify(value, null, 2))
      .then(() => {
        setIsCopied(true);
        window.setTimeout(() => setIsCopied(false), 1600);
      })
      .catch(() => undefined);
  };

  const handleToggle = () => {
    setIsExpanded((current) => !current);
  };

  return (
    <div className={`${styles.requestEventsDetailJsonNode} ${isRoot ? styles.requestEventsDetailJsonNodeRoot : ''}`}>
      <div className={styles.requestEventsDetailJsonRow}>
        {isExpandable ? (
          <button
            type="button"
            className={styles.requestEventsDetailJsonToggle}
            aria-expanded={isExpanded}
            aria-label={isExpanded ? labels.collapseNode : labels.expandNode}
            onClick={handleToggle}
          >
            {isExpanded ? '⌄' : '›'}
          </button>
        ) : (
          <span className={styles.requestEventsDetailJsonToggleSpacer} />
        )}
        {showName && <span className={styles.requestEventsDetailJsonKey}>{name}</span>}
        {isExpandable ? (
          <span className={styles.requestEventsDetailJsonBracket}>
            {openBracket}
            {!isExpanded && <span> {getJsonItemSummary(entries.length)} {closeBracket}</span>}
          </span>
        ) : (
          showName && <span className={styles.requestEventsDetailJsonColon}>:</span>
        )}
        {!isExpandable && (
          <div className={styles.requestEventsDetailJsonValueWrap}>
            <RequestDetailJsonValue value={value} />
          </div>
        )}
        <button
          type="button"
          className={styles.requestEventsDetailJsonCopyButton}
          aria-label={labels.copyNode}
          onClick={handleCopy}
        >
          {isCopied ? labels.copiedNode : labels.copyNode}
        </button>
      </div>
      {isExpandable && isExpanded && (
        <div className={styles.requestEventsDetailJsonChildren}>
          {level >= JSON_TREE_MAX_DEPTH ? (
            <span className={styles.requestEventsDetailJsonEmpty}>depth limited</span>
          ) : visibleEntries.length > 0 ? (
            <>
              {visibleEntries.map(([key, item]) => (
                <RequestDetailJsonNode
                  key={`${level}:${key}`}
                  name={key}
                  value={item}
                  isArrayItem={Array.isArray(value)}
                  defaultExpanded={childDefaultExpanded}
                  level={level + 1}
                />
              ))}
              {truncatedCount > 0 && (
                <span className={styles.requestEventsDetailJsonEmpty}>+{truncatedCount} more</span>
              )}
            </>
          ) : null}
          <div className={styles.requestEventsDetailJsonCloseBracket}>{closeBracket}</div>
        </div>
      )}
    </div>
  );
}

function RequestDetailJsonValue({ value }: { value: JsonValue }) {
  const { globalStringExpanded, labels } = React.useContext(RequestDetailJsonContext);
  const [localExpanded, setLocalExpanded] = useState<boolean | null>(null);
  const [showParsed, setShowParsed] = useState(false);
  const isExpanded = localExpanded ?? globalStringExpanded;
  const displayValue = useMemo(() => (typeof value === 'string' ? formatJsonStringForDisplay(value) : ''), [value]);
  const parsedJson = useMemo(() => (typeof value === 'string' ? parseJSONStringValue(value) : null), [value]);

  if (typeof value === 'string') {

    if (parsedJson && showParsed) {
      return (
        <div className={styles.requestEventsDetailJsonParsedString}>
          <div className={styles.requestEventsDetailJsonParsedHeader}>
            <span>{labels.jsonString}</span>
            <button type="button" onClick={() => setShowParsed(false)}>{labels.rawString}</button>
          </div>
          <div className={styles.requestEventsDetailJsonNestedExplorer}>
            <RequestDetailJsonNode name="" value={parsedJson.value} defaultExpanded />
          </div>
        </div>
      );
    }

    return (
      <div className={styles.requestEventsDetailJsonStringValue}>
        <button
          type="button"
          className={`${styles.requestEventsDetailJsonStringButton} ${isExpanded ? styles.requestEventsDetailJsonStringExpanded : ''}`}
          onClick={() => setLocalExpanded(!isExpanded)}
        >
          <span>"{displayValue}"</span>
          <span className={styles.requestEventsDetailJsonStringToggle}>{isExpanded ? '⌃' : '…'}</span>
        </button>
        {parsedJson && isExpanded && (
          <button
            type="button"
            className={styles.requestEventsDetailJsonParseButton}
            onClick={() => setShowParsed(true)}
          >
            {labels.parseString}
          </button>
        )}
      </div>
    );
  }

  if (typeof value === 'number') {
    return <span className={styles.requestEventsDetailJsonNumber}>{value}</span>;
  }
  if (typeof value === 'boolean') {
    return <span className={styles.requestEventsDetailJsonBoolean}>{String(value)}</span>;
  }
  if (value === null) {
    return <span className={styles.requestEventsDetailJsonNull}>null</span>;
  }
  return <span>{String(value)}</span>;
}

interface RequestDetailJsonBlockProps {
  labels: RequestDetailJsonLabels;
  title: string;
  rootName: string;
  value: JsonValue;
}

export function RequestDetailJsonBlock({ labels, title, rootName, value }: RequestDetailJsonBlockProps) {
  return (
    <div className={styles.requestEventsDetailJsonBlock}>
      <span>{title}</span>
      <div className={styles.requestEventsDetailJsonExplorer}>
        <RequestDetailJsonViewer labels={labels} rootName={rootName} value={value} />
      </div>
    </div>
  );
}
