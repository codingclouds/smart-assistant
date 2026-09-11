import { useEffect, useState, useRef } from 'react';
import { fetchExperts, type ExpertInfo } from '../../lib/api';

interface ExpertAgentCardProps {
  visible: boolean;
  onClose: () => void;
  onSelect: (expert: ExpertInfo) => void;
}

export default function ExpertAgentCard({ visible, onClose, onSelect }: ExpertAgentCardProps) {
  const [experts, setExperts] = useState<ExpertInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!visible) return;
    setLoading(true);
    setError(null);
    fetchExperts()
      .then(setExperts)
      .catch((err) => setError(err.message || '加载专家列表失败'))
      .finally(() => setLoading(false));
  }, [visible]);

  // 点击外部关闭
  useEffect(() => {
    if (!visible) return;
    const handler = (e: MouseEvent) => {
      if (panelRef.current && !panelRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    const timer = setTimeout(() => {
      document.addEventListener('mousedown', handler);
    }, 100);
    return () => {
      clearTimeout(timer);
      document.removeEventListener('mousedown', handler);
    };
  }, [visible, onClose]);

  if (!visible) return null;

  return (
    <div
      ref={panelRef}
      className="expert-card-panel"
      style={{
        position: 'absolute',
        bottom: '100%',
        left: 0,
        marginBottom: 8,
        width: 320,
        maxHeight: 360,
        background: 'var(--color-bg-secondary)',
        border: '1px solid var(--color-border)',
        borderRadius: 12,
        boxShadow: '0 8px 32px rgba(0,0,0,0.24)',
        zIndex: 100,
        overflow: 'hidden',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      {/* 标题栏 */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '10px 14px',
          borderBottom: '1px solid var(--color-border)',
          flexShrink: 0,
        }}
      >
        <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-primary)' }}>
          选择专家 Agent
        </span>
        <button
          onClick={onClose}
          style={{
            width: 22,
            height: 22,
            borderRadius: 5,
            border: 'none',
            background: 'transparent',
            color: 'var(--color-text-muted)',
            cursor: 'pointer',
            fontSize: 14,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          ✕
        </button>
      </div>

      {/* 内容区 */}
      <div style={{ flex: 1, overflowY: 'auto', padding: 6 }}>
        {loading && (
          <div style={{ padding: 24, textAlign: 'center', color: 'var(--color-text-muted)', fontSize: 13 }}>
            加载中...
          </div>
        )}

        {error && (
          <div
            style={{
              margin: 8,
              padding: '8px 12px',
              background: 'var(--color-error-muted)',
              borderRadius: 8,
              fontSize: 12,
              color: 'var(--color-error)',
            }}
          >
            {error}
          </div>
        )}

        {!loading && !error && experts.length === 0 && (
          <div style={{ padding: 24, textAlign: 'center', color: 'var(--color-text-muted)', fontSize: 13 }}>
            暂无可用的专家 Agent
          </div>
        )}

        {!loading &&
          experts.map((expert) => (
            <button
              key={expert.id}
              onClick={() => {
                onSelect(expert);
                onClose();
              }}
              style={{
                display: 'flex',
                alignItems: 'flex-start',
                gap: 10,
                width: '100%',
                padding: '10px 12px',
                borderRadius: 8,
                border: 'none',
                background: 'transparent',
                cursor: 'pointer',
                textAlign: 'left',
                transition: 'background 0.12s',
              }}
              className="expert-card-item"
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLElement).style.background = 'var(--color-bg-tertiary)';
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLElement).style.background = 'transparent';
              }}
            >
              {/* Emoji 头像 */}
              <span
                style={{
                  width: 36,
                  height: 36,
                  borderRadius: 10,
                  background: 'var(--color-accent-muted)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: 18,
                  flexShrink: 0,
                }}
              >
                {expert.emoji || '🤖'}
              </span>

              {/* 信息 */}
              <div style={{ flex: 1, minWidth: 0 }}>
                <div
                  style={{
                    fontSize: 13,
                    fontWeight: 600,
                    color: 'var(--color-text-primary)',
                    marginBottom: 2,
                  }}
                >
                  {expert.name}
                </div>
                <div
                  style={{
                    fontSize: 11,
                    color: 'var(--color-text-secondary)',
                    lineHeight: 1.4,
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                  }}
                >
                  {expert.description}
                </div>

                {/* 技能标签 */}
                {expert.skills && expert.skills.length > 0 && (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginTop: 6 }}>
                    {expert.skills.slice(0, 4).map((skill) => (
                      <span
                        key={skill}
                        style={{
                          fontSize: 10,
                          padding: '1px 6px',
                          borderRadius: 4,
                          background: 'var(--color-bg-tertiary)',
                          color: 'var(--color-text-muted)',
                        }}
                      >
                        {skill}
                      </span>
                    ))}
                    {expert.skills.length > 4 && (
                      <span
                        style={{
                          fontSize: 10,
                          color: 'var(--color-text-muted)',
                        }}
                      >
                        +{expert.skills.length - 4}
                      </span>
                    )}
                  </div>
                )}
              </div>

              {/* 选中箭头 */}
              <svg
                width="14"
                height="14"
                viewBox="0 0 14 14"
                fill="none"
                style={{ flexShrink: 0, marginTop: 10, color: 'var(--color-text-muted)' }}
              >
                <path
                  d="M5 3L9 7L5 11"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </button>
          ))}
      </div>

      {/* 底部提示 */}
      <div
        style={{
          padding: '6px 14px',
          borderTop: '1px solid var(--color-border)',
          flexShrink: 0,
          fontSize: 10,
          color: 'var(--color-text-muted)',
          textAlign: 'center',
        }}
      >
        选择一个专家 Agent 发起对话
      </div>
    </div>
  );
}