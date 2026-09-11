import { useState, useRef, useEffect, KeyboardEvent } from 'react';
import type { ModelConfig, ExpertInfo } from '../../lib/api';
import ExpertAgentCard from './ExpertAgentCard';

interface TokenStats {
  model_name: string;
  total_input: number;
  total_output: number;
  total_tokens: number;
}

interface Props {
  onSend: (prompt: string) => void;
  onCancel?: () => void;
  disabled: boolean;
  isStreaming?: boolean;
  sessionTokens?: number;
  stats?: TokenStats[];
  wide?: boolean;
  hideTokens?: boolean;
  models?: ModelConfig[];
  selectedModel?: string;
  onModelChange?: (name: string) => void;
  onMentionExpert?: () => void;
}

function formatTokens(n: number): string {
  if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
  return String(n);
}

export default function ChatInput({ onSend, onCancel, disabled, isStreaming, sessionTokens, stats, wide, hideTokens, models, selectedModel, onModelChange, onMentionExpert }: Props) {
  const [input, setInput] = useState('');
  const [files, setFiles] = useState<File[]>([]);
  const [modelOpen, setModelOpen] = useState(false);
  const [showMention, setShowMention] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const modelMenuRef = useRef<HTMLDivElement>(null);

  const enabledModels = (models || []).filter((m) => m.enabled);

  // 关闭模型下拉（点击外部）
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (modelMenuRef.current && !modelMenuRef.current.contains(e.target as Node)) {
        setModelOpen(false);
      }
    };
    if (modelOpen) {
      document.addEventListener('mousedown', handler);
    }
    return () => document.removeEventListener('mousedown', handler);
  }, [modelOpen]);

  // 监听建议提示词点击
  useEffect(() => {
    const handler = (e: Event) => {
      const detail = (e as CustomEvent).detail as string;
      if (detail) {
        setInput(detail);
        setTimeout(() => {
          textareaRef.current?.focus();
        }, 100);
      }
    };
    window.addEventListener('suggestion-click', handler);
    return () => window.removeEventListener('suggestion-click', handler);
  }, []);

  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = Math.min(el.scrollHeight, 160) + 'px';
  }, [input]);

  const handleSend = () => {
    const text = input.trim();
    if (!text || disabled) return;
    onSend(text);
    setInput('');
    setFiles([]);
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    } else if (e.key === 'Escape') {
      if (input.trim()) {
        setInput('');
      }
      textareaRef.current?.blur();
    }
  };

  const handleFileClick = () => {
    fileInputRef.current?.click();
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      setFiles((prev) => [...prev, ...Array.from(e.target.files!)]);
    }
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  const removeFile = (index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index));
  };

  const handleSelectExpert = (expert: ExpertInfo) => {
    setInput((prev) => prev + ` @${expert.name} `);
    setTimeout(() => textareaRef.current?.focus(), 50);
  };

  const totalTokens = stats?.reduce((sum, s) => sum + s.total_tokens, 0) ?? 0;

  return (
    <div className="chat-input-wrapper">
      <div className="chat-input-container" style={{ position: 'relative' }}>
        {/* 专家 Agent 选择面板 */}
        <ExpertAgentCard
          visible={showMention}
          onClose={() => setShowMention(false)}
          onSelect={handleSelectExpert}
        />

        {/* 附件列表 */}
        {files.length > 0 && (
          <div className="chat-attachments">
            {files.map((f, i) => (
              <span key={i} className="chat-attachment-tag">
                <span style={{ maxWidth: 120, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {f.name}
                </span>
                <span
                  className="chat-attachment-remove"
                  onClick={() => removeFile(i)}
                  title="移除"
                >
                  ×
                </span>
              </span>
            ))}
          </div>
        )}

        {/* 输入区域 — 大圆角 + 内嵌工具栏 */}
        <div className="chat-input-box">
          {/* 附件上传按钮 */}
          <input
            ref={fileInputRef}
            type="file"
            multiple
            onChange={handleFileChange}
            className="hidden"
            aria-hidden
          />
          <button
            onClick={handleFileClick}
            className="chat-input-tool-btn"
            title="上传文件"
            style={{ fontSize: 18, fontWeight: 300 }}
          >
            +
          </button>

          {/* @专家 提及按钮 */}
          {onMentionExpert && (
            <button
              onClick={() => {
                setShowMention((v) => !v);
                onMentionExpert();
              }}
              className="chat-input-tool-btn"
              title="@提及专家 Agent"
              style={{ fontWeight: 700 }}
            >
              @
            </button>
          )}

          {/* 文本输入 */}
          <textarea
            ref={textareaRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="输入消息，Enter 发送，Shift+Enter 换行"
            disabled={disabled}
            rows={1}
            aria-label="输入消息"
            className="chat-input-textarea"
          />

          {/* 内嵌工具栏 */}
          <div className="chat-input-toolbar">
            {/* 模型切换下拉按钮 */}
            {enabledModels.length > 0 && (
              <div style={{ position: 'relative' }} ref={modelMenuRef}>
                <button
                  onClick={() => setModelOpen((v) => !v)}
                  className="chat-input-model-btn"
                  title="切换模型"
                >
                  <span style={{
                    width: 6,
                    height: 6,
                    borderRadius: '50%',
                    background: '#22c55e',
                    flexShrink: 0,
                  }} />
                  <span style={{ maxWidth: 60, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {selectedModel || enabledModels[0]?.name || '模型'}
                  </span>
                  <svg width="8" height="8" viewBox="0 0 8 8" fill="currentColor" style={{ opacity: 0.5 }}>
                    <path d="M2 3l2 2 2-2" stroke="currentColor" strokeWidth="1.2" fill="none" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                </button>
                {modelOpen && (
                  <div className="chat-input-model-menu">
                    {enabledModels.map((m) => (
                      <button
                        key={m.id}
                        onClick={() => {
                          onModelChange?.(m.name);
                          setModelOpen(false);
                        }}
                        className={`chat-input-model-option${m.name === selectedModel ? ' chat-input-model-option--selected' : ''}`}
                      >
                        <span style={{
                          width: 6,
                          height: 6,
                          borderRadius: '50%',
                          background: m.name === selectedModel ? '#6366f1' : '#22c55e',
                          flexShrink: 0,
                        }} />
                        <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {m.name}
                        </span>
                        {m.name === selectedModel && (
                          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" style={{ flexShrink: 0 }}>
                            <path d="M2.5 6L5 8.5L9.5 3.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                          </svg>
                        )}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* Token用量按钮 */}
            {!hideTokens && (
              <button className="chat-input-model-btn" title="Token 用量" style={{ gap: 3 }}>
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                  <circle cx="6" cy="6" r="4.5" stroke="currentColor" strokeWidth="1.1"/>
                  <path d="M6 3.5v2.5l2 1" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round"/>
                </svg>
                <span>
                  {sessionTokens !== undefined ? formatTokens(sessionTokens) : (totalTokens > 0 ? formatTokens(totalTokens) : '0')}
                </span>
              </button>
            )}

            {/* 发送 / 停止按钮 */}
            {isStreaming ? (
              <button
                onClick={onCancel}
                className="chat-input-send-btn"
                style={{ background: '#ef4444' }}
                title="停止"
              >
                <svg width="12" height="12" viewBox="0 0 12 12" fill="#fff">
                  <rect x="1.5" y="1.5" width="9" height="9" rx="1.5" />
                </svg>
              </button>
            ) : (
              <button
                onClick={handleSend}
                disabled={disabled || !input.trim()}
                className="chat-input-send-btn"
                style={{
                  background: (disabled || !input.trim()) ? 'var(--color-bg-tertiary)' : 'var(--color-accent)',
                }}
                title="发送"
              >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                  <path
                    d="M14.5 2L7 9.5M14.5 2L10 14.5L7 9.5M14.5 2L2 6.5L7 9.5"
                    stroke={disabled || !input.trim() ? 'var(--color-text-muted)' : '#fff'}
                    strokeWidth="1.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              </button>
            )}
          </div>
        </div>

        {/* 底部提示 */}
        {!hideTokens && (
          <div className="chat-input-bottom-bar">
            {sessionTokens === undefined && totalTokens === 0 && (
              <span>Enter 发送 · Shift+Enter 换行</span>
            )}
          </div>
        )}
      </div>
    </div>
  );
}