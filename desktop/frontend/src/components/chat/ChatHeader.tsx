import { useState, useRef, useEffect } from 'react';
import { useSessionStore, useChatUIStore, useMessageStore } from '../../lib/store';

interface ChatHeaderProps {
  onStop: () => void;
  onSkipQueue: () => void;
}

export default function ChatHeader({ onStop, onSkipQueue }: ChatHeaderProps) {
  const activeSessionId = useSessionStore((s) => s.activeSessionId);
  const sessions = useSessionStore((s) => s.sessions);
  const isStreaming = useChatUIStore((s) => s.isStreaming);
  const isLoading = useMessageStore((s) => s.isLoading);
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  const activeSession = sessions.find((s) => s.id === activeSessionId);

  useEffect(() => {
    if (activeSession) {
      setTitle(activeSession.title || '新对话');
    }
  }, [activeSession]);

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [editing]);

  const handleSave = () => {
    setEditing(false);
    const trimmed = title.trim();
    if (!trimmed) {
      setTitle(activeSession?.title || '新对话');
      return;
    }
    // TODO: 调用 API 更新会话标题
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSave();
    } else if (e.key === 'Escape') {
      setTitle(activeSession?.title || '新对话');
      setEditing(false);
    }
  };

  const isRunning = isStreaming || isLoading;

  return (
    <div
      className="chat-header flex items-center gap-3 px-4 py-2.5 border-b flex-shrink-0"
      style={{ borderColor: 'var(--color-border)', background: 'var(--color-bg-secondary)' }}
    >
      {/* 会话标题 */}
      <div className="chat-header__title flex-1 min-w-0">
        {editing ? (
          <input
            ref={inputRef}
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onBlur={handleSave}
            onKeyDown={handleKeyDown}
            className="chat-header__title-input"
            style={{
              fontSize: 14,
              fontWeight: 600,
              color: 'var(--color-text-primary)',
              background: 'var(--color-bg-tertiary)',
              border: '1px solid var(--color-accent)',
              borderRadius: 6,
              padding: '3px 8px',
              width: '100%',
              maxWidth: 320,
              outline: 'none',
            }}
          />
        ) : (
          <button
            onClick={() => setEditing(true)}
            className="chat-header__title-text"
            title="点击编辑会话标题"
          >
            {activeSession?.title || '新对话'}
            <svg
              width="12" height="12" viewBox="0 0 12 12" fill="none"
              className="chat-header__edit-icon"
            >
              <path
                d="M8.5 1.5a1.414 1.414 0 0 1 2 2l-6.5 6.5L1 11l1-3 6.5-6.5z"
                stroke="currentColor"
                strokeWidth="1.2"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </button>
        )}
      </div>

      {/* 操作按钮 */}
      <div className="chat-header__actions flex items-center gap-1">
        {/* 停止任务 */}
        <button
          onClick={onStop}
          disabled={!isRunning}
          className="chat-header__action-btn"
          title="停止当前任务"
          style={{
            opacity: isRunning ? 1 : 0.3,
            cursor: isRunning ? 'pointer' : 'not-allowed',
          }}
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <rect x="3" y="3" width="10" height="10" rx="1.5" fill="var(--color-error)" />
          </svg>
          <span className="chat-header__action-label">停止</span>
        </button>

        {/* 插队 */}
        <button
          onClick={onSkipQueue}
          disabled={!isRunning}
          className="chat-header__action-btn"
          title="跳过当前任务，优先处理新消息"
          style={{
            opacity: isRunning ? 1 : 0.3,
            cursor: isRunning ? 'pointer' : 'not-allowed',
          }}
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <path
              d="M13 2L9 6L13 10"
              stroke="var(--color-warning)"
              strokeWidth="1.8"
              strokeLinecap="round"
              strokeLinejoin="round"
              fill="none"
            />
            <path
              d="M9 8H4.5C3.12 8 2 6.88 2 5.5V5.5C2 4.12 3.12 3 4.5 3H5"
              stroke="var(--color-warning)"
              strokeWidth="1.4"
              strokeLinecap="round"
            />
          </svg>
          <span className="chat-header__action-label">插队</span>
        </button>
      </div>
    </div>
  );
}