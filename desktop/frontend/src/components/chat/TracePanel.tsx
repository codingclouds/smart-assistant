import { useState, useEffect } from 'react';
import { fetchTraces, type TraceEvent } from '../../lib/api';

interface Props {
  sessionId: string;
  onClose: () => void;
}

const EVENT_STYLE: Record<string, { label: string; color: string; bg: string }> = {
  tool_call: { label: '工具调用', color: '#7c8aff', bg: 'rgba(124,138,255,0.1)' },
  thought: { label: '思考', color: '#a78bfa', bg: 'rgba(167,139,250,0.1)' },
  response: { label: '响应', color: '#4ade80', bg: 'rgba(74,222,128,0.1)' },
  error: { label: '错误', color: '#f87171', bg: 'rgba(248,113,113,0.1)' },
};

function parsePayload(payload: string): Record<string, unknown> | null {
  try {
    return JSON.parse(payload);
  } catch {
    return null;
  }
}

export default function TracePanel({ sessionId, onClose }: Props) {
  const [events, setEvents] = useState<TraceEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  const load = () => {
    if (!sessionId) return;
    setLoading(true);
    setError(null);
    fetchTraces(sessionId)
      .then(setEvents)
      .catch((err) => setError(err.message || '加载 Trace 失败'))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load();
  }, [sessionId]);

  const toggleExpand = (id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <div
      className="h-full flex flex-col border-l"
      style={{
        width: 360,
        minWidth: 360,
        background: 'var(--color-bg-secondary)',
        borderColor: 'var(--color-border)',
      }}
    >
      {/* 头部 */}
      <div
        className="flex items-center justify-between px-4 py-3 border-b"
        style={{ borderColor: 'var(--color-border)' }}
      >
        <div className="flex items-center gap-2.5">
          <span className="text-sm">🔍</span>
          <span className="text-sm font-medium" style={{ color: 'var(--color-text-primary)' }}>
            执行链路
          </span>
          {events.length > 0 && (
            <span
              className="text-[11px] px-1.5 py-0.5 rounded-full font-medium"
              style={{ background: 'var(--color-bg-tertiary)', color: 'var(--color-text-muted)' }}
            >
              {events.length}
            </span>
          )}
        </div>
        <button
          onClick={onClose}
          className="w-6 h-6 rounded-lg flex items-center justify-center text-sm transition-all hover:bg-white/5"
          style={{ color: 'var(--color-text-muted)' }}
        >
          ✕
        </button>
      </div>

      {/* 内容 */}
      <div className="flex-1 overflow-y-auto px-3 py-2">
        {loading && (
          <p className="text-xs text-center py-8" style={{ color: 'var(--color-text-muted)' }}>
            加载中...
          </p>
        )}

        {error && (
          <div className="text-center py-8 px-3">
            <p className="text-xs mb-2.5" style={{ color: 'var(--color-error)' }}>
              {error}
            </p>
            <button
              onClick={load}
              className="text-xs px-3 py-1.5 rounded-lg transition-all hover:brightness-110"
              style={{
                background: 'var(--color-accent-muted)',
                color: 'var(--color-accent)',
              }}
            >
              重试
            </button>
          </div>
        )}

        {!loading && !error && events.length === 0 && (
          <div className="text-center py-12">
            <p className="text-2xl mb-2 opacity-50">🔍</p>
            <p className="text-xs" style={{ color: 'var(--color-text-muted)' }}>
              暂无 Trace 记录
            </p>
          </div>
        )}

        {!loading &&
          !error &&
          events.map((evt, idx) => {
            const style = EVENT_STYLE[evt.event_type] || EVENT_STYLE.thought;
            const payload = parsePayload(evt.payload);
            const isExpanded = expanded.has(evt.id);

            return (
              <div key={evt.id} className="relative">
                {idx < events.length - 1 && (
                  <div
                    className="absolute left-[10px] top-7 w-0.5"
                    style={{
                      height: 'calc(100% + 2px)',
                      background: 'var(--color-border)',
                    }}
                  />
                )}
                <div className="flex gap-2.5 py-1">
                  <div
                    className="w-5 h-5 rounded-full flex items-center justify-center flex-shrink-0 mt-0.5 z-10"
                    style={{ background: style.bg, border: `2px solid ${style.color}` }}
                  >
                    <div className="w-1.5 h-1.5 rounded-full" style={{ background: style.color }} />
                  </div>
                  <div
                    className="flex-1 rounded-lg p-2.5 cursor-pointer transition-all min-w-0 hover:border-[var(--color-border-light)]"
                    style={{
                      background: 'var(--color-bg-tertiary)',
                      border: `1px solid ${isExpanded ? style.color : 'var(--color-border)'}`,
                    }}
                    onClick={() => toggleExpand(evt.id)}
                  >
                    <div className="flex items-center justify-between gap-1">
                      <span
                        className="text-[11px] px-1.5 py-0.5 rounded-md font-medium flex-shrink-0"
                        style={{ background: style.bg, color: style.color }}
                      >
                        {style.label}
                      </span>
                      <div className="flex items-center gap-2 flex-shrink-0">
                        {evt.duration_ms > 0 && (
                          <span className="text-[11px]" style={{ color: 'var(--color-text-muted)' }}>
                            {evt.duration_ms >= 1000
                              ? `${(evt.duration_ms / 1000).toFixed(1)}s`
                              : `${evt.duration_ms}ms`}
                          </span>
                        )}
                        <span className="text-[11px]" style={{ color: 'var(--color-text-muted)' }}>
                          {new Date(evt.created_at).toLocaleTimeString()}
                        </span>
                      </div>
                    </div>

                    {isExpanded && payload && (
                      <div
                        className="mt-2 p-2 rounded-lg text-xs font-mono overflow-x-auto"
                        style={{
                          background: 'var(--color-bg-primary)',
                          color: 'var(--color-text-secondary)',
                          maxHeight: 160,
                          overflowY: 'auto',
                        }}
                      >
                        <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                          {JSON.stringify(payload, null, 2)}
                        </pre>
                      </div>
                    )}

                    {isExpanded && !payload && evt.payload && (
                      <div
                        className="mt-2 p-2 rounded-lg text-xs overflow-x-auto"
                        style={{
                          background: 'var(--color-bg-primary)',
                          color: 'var(--color-text-muted)',
                        }}
                      >
                        {evt.payload}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
      </div>
    </div>
  );
}