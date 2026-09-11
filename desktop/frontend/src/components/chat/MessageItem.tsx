import { useState, useEffect } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import ToolCallCard from './ToolCallCard';
import ConfirmCard from './ConfirmCard';
import FileArtifactCard from './FileArtifactCard';
import type { ContentBlock } from '../../lib/store';

interface Props {
  message: {
    id: string;
    role: 'user' | 'assistant' | 'system';
    content: string;
    blocks?: ContentBlock[];
  };
  isStreaming: boolean;
  onPreviewOpen?: (url: string, title: string) => void;
  onFilePreview?: (workspaceId: string, filePath: string, fileName: string) => void;
}

/** 从 blocks 中找到 tool_call 对应的 tool_result */
function findResult(blocks: ContentBlock[], callId: string): ContentBlock | undefined {
  return blocks.find(
    (b) => b.type === 'tool_result' && b.call_id === callId
  ) as ContentBlock | undefined;
}

/** 复制到剪贴板 */
async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    // fallback
    const ta = document.createElement('textarea');
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
  }
}

export default function MessageItem({ message, isStreaming, onPreviewOpen, onFilePreview }: Props) {
  const isUser = message.role === 'user';
  const hasBlocks = message.blocks && message.blocks.length > 0;
  const isEmpty = !message.content && !hasBlocks;

  // Agent 消息折叠状态
  const [headerOpen, setHeaderOpen] = useState(true);
  const [thinkingOpen, setThinkingOpen] = useState(true);
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    copyToClipboard(message.content);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  // 思考内容：从 blocks 中提取真实 thinking 内容
  const hasToolCalls = hasBlocks && message.blocks!.some((b) => b.type === 'tool_call');

  // 从 blocks 提取模型推理/思考文本（来自 thinking 事件流）
  const thinkingBlocks = hasBlocks
    ? message.blocks!.filter((b) => b.type === 'thinking')
    : [];
  const realThinkingText = thinkingBlocks
    .map((b) => (b as { thinking: string }).thinking)
    .join('');

  // 从 blocks 提取文件产物卡片（统一放到回答末尾展示）
  const fileArtifactBlocks = hasBlocks
    ? message.blocks!.filter((b) => b.type === 'file_artifact')
    : [];

  // 检查所有工具调用是否都已完成（都有对应的 tool_result）
  const allToolsDone = hasToolCalls && message.blocks!.every((b) => {
    if (b.type !== 'tool_call') return true;
    return message.blocks!.some((r) => r.type === 'tool_result' && r.call_id === b.call_id);
  });

  // 思考完成后自动折叠
  useEffect(() => {
    if (allToolsDone) {
      setThinkingOpen(false);
    }
  }, [allToolsDone]);

  // 思考标签文字：优先展示模型真实推理状态
  const hasRealThinking = realThinkingText.length > 0;
  const thinkingLabel = allToolsDone
    ? '思考完成'
    : (hasRealThinking ? '正在思考...' : (hasToolCalls ? '正在分析你的请求，检索相关信息...' : ''));
  const showThinking = hasRealThinking || hasToolCalls;

  // ===== User Message =====
  if (isUser) {
    return (
      <div className="user-msg">
        <div>
          <div
            className="user-msg__bubble"
            style={{
              background: 'var(--color-user-bubble)',
              color: 'var(--color-user-bubble-text)',
            }}
          >
            {message.content}
          </div>
          <div className="user-msg__actions">
            <button
              className="user-msg__action-btn"
              onClick={handleCopy}
              title="复制"
            >
              {copied ? '✓' : (
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                  <rect x="3" y="3" width="8" height="8" rx="1.5" stroke="currentColor" strokeWidth="1.2"/>
                  <path d="M1 9V2a1 1 0 011-1h6" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round"/>
                </svg>
              )}
            </button>
            <button
              className="user-msg__action-btn"
              onClick={() => {
                window.dispatchEvent(
                  new CustomEvent('suggestion-click', { detail: message.content })
                );
              }}
              title="编辑"
            >
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                <path d="M8 1.5l2.5 2.5-7 7L1 11.5l.5-2.5 7-7z" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
            </button>
          </div>
        </div>
      </div>
    );
  }

  // ===== Assistant / Agent Message =====
  // 加载中空内容
  if (isEmpty && isStreaming) {
    return (
      <div className="agent-msg">
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '4px 0', color: 'var(--color-text-muted)', fontSize: 13 }}>
          <span style={{
            width: 6, height: 6, borderRadius: '50%', background: 'var(--color-accent)',
            animation: 'pulse 1s ease-in-out infinite',
          }} />
          正在思考...
        </div>
      </div>
    );
  }

  if (isEmpty) return null;

  return (
    <div className="agent-msg">
      {/* 标题栏：已工作 X 秒 / 可折叠 */}
      <div
        className="agent-msg__header"
        onClick={() => setHeaderOpen((v) => !v)}
        role="button"
        tabIndex={0}
        aria-expanded={headerOpen}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            setHeaderOpen((v) => !v);
          }
        }}
      >
        <span className={`agent-msg__header-arrow${headerOpen ? ' agent-msg__header-arrow--open' : ''}`}>
          <svg width="8" height="12" viewBox="0 0 8 12" fill="none">
            <path d="M1.5 1L6.5 6L1.5 11" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        </span>
        <span className="agent-msg__header-text">
          已工作 · {message.content ? `${Math.max(1, Math.round(message.content.length / 15))} 秒` : '计算中...'}
        </span>
      </div>

      {headerOpen && (
        <>
          {/* 思考块 */}
          {showThinking && (
            <div className="agent-msg__thinking">
              <div
                className="agent-msg__thinking-toggle"
                onClick={() => setThinkingOpen((v) => !v)}
                role="button"
                tabIndex={0}
                aria-expanded={thinkingOpen}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    setThinkingOpen((v) => !v);
                  }
                }}
              >
                <svg
                  width="8" height="12" viewBox="0 0 8 12" fill="none"
                  style={{
                    transform: thinkingOpen ? 'rotate(90deg)' : 'none',
                    transition: 'transform 0.2s ease',
                    flexShrink: 0,
                  }}
                >
                  <path d="M1.5 1L6.5 6L1.5 11" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
                <span style={{
                  color: allToolsDone ? 'var(--color-success)' : 'var(--color-text-secondary)',
                }}>
                  {allToolsDone ? '✓ ' : ''}{thinkingLabel}
                </span>
              </div>
              {thinkingOpen && (
                <div className="agent-msg__thinking-content">
                  {realThinkingText ? (
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>
                      {realThinkingText}
                    </ReactMarkdown>
                  ) : (
                    <p>{thinkingLabel}</p>
                  )}
                </div>
              )}
            </div>
          )}

          {/* 工具调用卡片 — 可折叠（file_artifact 排到末尾） */}
          {hasBlocks && message.blocks!.map((block, i) => {
            switch (block.type) {
              case 'text':
                return (
                  <div key={i} className="markdown-body">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>
                      {block.text}
                    </ReactMarkdown>
                  </div>
                );
              case 'tool_call': {
                const resultBlock = findResult(message.blocks!, block.call_id);
                return (
                  <ToolCallCard
                    key={i}
                    callId={block.call_id}
                    name={block.name}
                    arguments={block.arguments}
                    result={resultBlock ? (resultBlock as { result: string; error?: string }).result : undefined}
                    error={resultBlock ? (resultBlock as { result: string; error?: string }).error : undefined}
                    isExecuting={!resultBlock}
                  />
                );
              }
              case 'confirm_request':
                return (
                  <ConfirmCard
                    key={i}
                    callId={block.call_id}
                    question={block.question}
                    options={block.options}
                    context={block.context}
                    response={(block as { response?: string }).response}
                  />
                );
              case 'file_artifact':
                // 文件产物卡片放到回答末尾统一展示
                return null;
              case 'tool_result':
                return null;
              default:
                return null;
            }
          })}
        </>
      )}

      {/* 最终回答正文 — 始终可见 */}
      {message.content && (
        <div className="agent-msg__answer">
          <div className={`markdown-body${isStreaming && !hasBlocks ? ' streaming-cursor' : ''}`}>
            <ReactMarkdown remarkPlugins={[remarkGfm]}>
              {message.content}
            </ReactMarkdown>
          </div>
        </div>
      )}

      {/* 文件产物卡片 — 统一排在回答末尾 */}
      {fileArtifactBlocks.length > 0 && (
        <div className="agent-msg__files">
          {fileArtifactBlocks.map((block, i) => (
            <FileArtifactCard
              key={`file-${i}`}
              filePath={(block as { file_path: string }).file_path}
              fileName={(block as { file_name: string }).file_name}
              byteCount={(block as { byte_count: number }).byte_count}
              workspaceId={(block as { workspace_id: string }).workspace_id}
              onPreview={onFilePreview}
            />
          ))}
        </div>
      )}

      {/* 底部操作栏 — 始终可见 */}
      <div className="agent-msg__actions">
            <button className="agent-msg__action-btn" onClick={handleCopy} title="复制">
              {copied ? (
                <span style={{ color: 'var(--color-success)', fontSize: 12 }}>✓</span>
              ) : (
                <svg width="13" height="13" viewBox="0 0 13 13" fill="none">
                  <rect x="3" y="3" width="8" height="8" rx="1.5" stroke="currentColor" strokeWidth="1.1"/>
                  <path d="M1 9V2a1 1 0 011-1h6" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round"/>
                </svg>
              )}
            </button>
            <button className="agent-msg__action-btn" title="点赞">
              <svg width="13" height="13" viewBox="0 0 13 13" fill="none">
                <path d="M2.5 5.5h2v6h-2a1 1 0 01-1-1V6.5a1 1 0 011-1z" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" strokeLinejoin="round"/>
                <path d="M4.5 5.5L6 2.5a1.5 1.5 0 011.5 1.5v1.5h2.5a1 1 0 011 1.2l-1 4.5a1 1 0 01-1 .8H4.5" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
            </button>
            <button className="agent-msg__action-btn" title="点踩">
              <svg width="13" height="13" viewBox="0 0 13 13" fill="none">
                <path d="M10.5 7.5h-2v-6h2a1 1 0 011 1v4a1 1 0 01-1 1z" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" strokeLinejoin="round"/>
                <path d="M8.5 7.5L7 10.5a1.5 1.5 0 01-1.5-1.5V7.5H3a1 1 0 01-1-1.2l1-4.5a1 1 0 011-.8h4" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
            </button>
            <button className="agent-msg__action-btn" title="分享">
              <svg width="13" height="13" viewBox="0 0 13 13" fill="none">
                <circle cx="3" cy="6.5" r="1.5" stroke="currentColor" strokeWidth="1.1"/>
                <circle cx="10" cy="3.5" r="1.5" stroke="currentColor" strokeWidth="1.1"/>
                <circle cx="10" cy="9.5" r="1.5" stroke="currentColor" strokeWidth="1.1"/>
                <path d="M4.3 5.8l4.4-1.6M4.3 7.2l4.4 1.6" stroke="currentColor" strokeWidth="1.1"/>
              </svg>
            </button>
            <span className="agent-msg__timestamp">
              {new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}
            </span>
          </div>
    </div>
  );
}