import { useState, useEffect } from 'react';

// 工具名 → 图标映射
const TOOL_ICONS: Record<string, string> = {
  web_search: '🔍',
  search: '🔍',
  read_file: '📖',
  write_file: '✏️',
  execute_command: '💻',
  bash: '💻',
  web_fetch: '🌐',
  default: '🔧',
};

// 工具名 → 显示标签
const TOOL_LABELS: Record<string, string> = {
  web_search: '网页搜索',
  search: '搜索',
  read_file: '读取文件',
  write_file: '写入文件',
  execute_command: '执行命令',
  bash: '命令执行',
  web_fetch: '网页抓取',
  default: '工具调用',
};

interface ToolCallCardProps {
  callId: string;
  name: string;
  arguments: string;
  result?: string;
  error?: string;
  isExecuting?: boolean;
}

export default function ToolCallCard({
  callId,
  name,
  arguments: args,
  result,
  error,
  isExecuting,
}: ToolCallCardProps) {
  const [expanded, setExpanded] = useState(false);

  // 结果到来时自动折叠
  useEffect(() => {
    if (result !== undefined || error) {
      setExpanded(false);
    }
  }, [result, error]);

  const icon = TOOL_ICONS[name] || TOOL_ICONS.default;
  const label = TOOL_LABELS[name] || name;

  // 解析参数摘要（最多 60 字符）
  let argSummary = '';
  try {
    const parsed = JSON.parse(args || '{}');
    const firstKey = Object.keys(parsed)[0];
    if (firstKey) {
      const val = String(parsed[firstKey]);
      argSummary = `${firstKey}: "${val.length > 40 ? val.slice(0, 40) + '...' : val}"`;
    }
  } catch {
    argSummary = args ? args.slice(0, 60) : '';
  }

  const hasError = !!error;

  return (
    <div
      className={`chat-tool-card ${hasError ? 'chat-tool-card--error' : ''}`}
      style={{
        marginTop: 6,
        marginBottom: 6,
      }}
    >
      {/* 折叠态：扁平行，无卡片 chrome */}
      <button
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        className="chat-tool-card__header"
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          width: '100%',
          padding: '4px 8px 4px 0',
          borderRadius: 6,
          border: 'none',
          background: 'transparent',
          cursor: 'pointer',
          fontSize: 13,
          lineHeight: 1.5,
          color: hasError ? 'var(--color-error)' : 'var(--color-text-secondary)',
          textAlign: 'left',
          transition: 'background 0.15s',
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.background = 'var(--color-bg-hover)';
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.background = 'transparent';
        }}
      >
        {/* 展开箭头 */}
        <span
          style={{
            display: 'inline-flex',
            transition: 'transform 0.15s',
            transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)',
            fontSize: 11,
            flexShrink: 0,
            width: 14,
          }}
        >
          ▸
        </span>

        {/* 工具图标 + 名称 */}
        <span style={{ flexShrink: 0 }}>{icon}</span>
        <span style={{ fontWeight: 500, flexShrink: 0 }}>{label}</span>

        {/* 执行中动画 */}
        {isExecuting && !result && !error && (
          <span
            className="chat-tool-card__spinner"
            style={{
              display: 'inline-block',
              width: 12,
              height: 12,
              border: '2px solid var(--color-border)',
              borderTopColor: 'var(--color-accent)',
              borderRadius: '50%',
              animation: 'spin 0.8s linear infinite',
              flexShrink: 0,
            }}
          />
        )}

        {/* 参数摘要 */}
        {argSummary && (
          <span
            style={{
              fontFamily: 'monospace',
              fontSize: 12,
              color: 'var(--color-text-muted)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              flex: 1,
              minWidth: 0,
            }}
          >
            {argSummary}
          </span>
        )}

        {/* 状态指示器 */}
        {result !== undefined && !hasError && (
          <span style={{ flexShrink: 0, marginLeft: 'auto', fontSize: 11, color: 'var(--color-accent)' }}>✓</span>
        )}
        {hasError && (
          <span style={{ flexShrink: 0, marginLeft: 'auto', fontSize: 11 }}>✕</span>
        )}
      </button>

      {/* 展开态：详情体 */}
      {expanded && (
        <div
          className="chat-tool-card__body"
          style={{
            padding: '0 8px 8px 28px',
            borderLeft: '2px solid var(--color-border)',
            marginLeft: 6,
          }}
        >
          {/* TOOL INPUT */}
          <div style={{ marginTop: 8 }}>
            <div
              style={{
                fontSize: 10,
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.05em',
                color: 'var(--color-text-muted)',
                marginBottom: 4,
              }}
            >
              TOOL INPUT
            </div>
            <pre
              style={{
                margin: 0,
                padding: '8px 10px',
                borderRadius: 6,
                fontSize: 12,
                fontFamily: 'monospace',
                lineHeight: 1.5,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
                background: 'color-mix(in srgb, var(--color-bg-secondary) 82%, transparent)',
                color: 'var(--color-text-primary)',
                maxHeight: '40vh',
                overflowY: 'auto',
              }}
            >
              {(() => {
                try {
                  return JSON.stringify(JSON.parse(args || '{}'), null, 2);
                } catch {
                  return args || '(no arguments)';
                }
              })()}
            </pre>
          </div>

          {/* RESULT */}
          {(result !== undefined || error) && (
            <div style={{ marginTop: 8 }}>
              <div
                style={{
                  fontSize: 10,
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  letterSpacing: '0.05em',
                  color: hasError ? 'var(--color-error)' : 'var(--color-text-muted)',
                  marginBottom: 4,
                }}
              >
                {hasError ? 'ERROR' : 'RESULT'}
              </div>
              <pre
                style={{
                  margin: 0,
                  padding: '8px 10px',
                  borderRadius: 6,
                  fontSize: 12,
                  fontFamily: 'monospace',
                  lineHeight: 1.5,
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                  background: hasError
                    ? 'color-mix(in srgb, var(--color-error) 8%, transparent)'
                    : 'color-mix(in srgb, var(--color-bg-secondary) 82%, transparent)',
                  color: hasError ? 'var(--color-error)' : 'var(--color-text-primary)',
                  maxHeight: '60vh',
                  overflowY: 'auto',
                }}
              >
                {hasError ? error : result}
              </pre>
            </div>
          )}

          {/* Executing placeholder */}
          {isExecuting && result === undefined && !error && (
            <div style={{ marginTop: 8 }}>
              <span
                style={{
                  fontSize: 12,
                  color: 'var(--color-text-muted)',
                  fontStyle: 'italic',
                }}
              >
                执行中...
              </span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}