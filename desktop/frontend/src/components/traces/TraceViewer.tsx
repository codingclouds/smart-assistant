import { useState, useEffect } from 'react';
import { fetchTraces, fetchSessions, type TraceEvent } from '../../lib/api';

interface SessionOption {
  id: string;
  title: string;
}

const EVENT_STYLE: Record<string, { label: string; color: string; bg: string }> = {
  tool_call: { label: '工具调用', color: '#3b82f6', bg: 'rgba(59,130,246,0.1)' },
  thought: { label: '思考', color: '#a855f7', bg: 'rgba(168,85,247,0.1)' },
  response: { label: '响应', color: '#22c55e', bg: 'rgba(34,197,94,0.1)' },
  error: { label: '错误', color: '#ef4444', bg: 'rgba(239,68,68,0.1)' },
};

function parsePayload(payload: string): Record<string, unknown> | null {
  try {
    return JSON.parse(payload);
  } catch {
    return null;
  }
}

export default function TraceViewer() {
  const [sessions, setSessions] = useState<SessionOption[]>([]);
  const [selectedId, setSelectedId] = useState<string>('');
  const [events, setEvents] = useState<TraceEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  useEffect(() => {
    fetchSessions()
      .then(setSessions)
      .catch(console.error);
  }, []);

  const loadTraces = async (sessionId: string) => {
    setSelectedId(sessionId);
    setLoading(true);
    try {
      const data = await fetchTraces(sessionId);
      setEvents(data);
    } catch (err) {
      console.error('Failed to load traces:', err);
      setEvents([]);
    } finally {
      setLoading(false);
    }
  };

  const toggleExpand = (id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <div className="flex-1 flex flex-col items-center overflow-y-auto p-8">
      <div className="w-full max-w-3xl">
        <h1 className="text-xl font-semibold mb-6" style={{ color: 'var(--color-text-primary)' }}>
          执行链路追踪
        </h1>

        {/* 会话选择器 */}
        <div className="mb-6">
          <label className="block text-xs mb-2" style={{ color: 'var(--color-text-muted)' }}>
            选择会话
          </label>
          <select
            value={selectedId}
            onChange={(e) => loadTraces(e.target.value)}
            className="w-full px-3 py-2 rounded-md text-sm border outline-none transition-colors appearance-none cursor-pointer"
            style={{
              background: 'var(--color-bg-secondary)',
              color: 'var(--color-text-primary)',
              borderColor: 'var(--color-border)',
            }}
          >
            <option value="" disabled>
              {sessions.length === 0 ? '暂无会话' : '请选择会话查看 Trace'}
            </option>
            {sessions.map((s) => (
              <option key={s.id} value={s.id}>
                {s.title || '新对话'}
              </option>
            ))}
          </select>
        </div>

        {loading && (
          <p className="text-sm text-center py-12" style={{ color: 'var(--color-text-muted)' }}>
            加载中...
          </p>
        )}

        {!loading && selectedId && events.length === 0 && (
          <div className="text-center py-12">
            <p className="text-4xl mb-3">🔍</p>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
              该会话暂无 Trace 记录
            </p>
            <p className="text-xs mt-1" style={{ color: 'var(--color-text-muted)' }}>
              Agent 执行后会自动记录
            </p>
          </div>
        )}

        {!loading && events.length > 0 && (
          <div className="space-y-1">
            {events.map((evt, idx) => {
              const style = EVENT_STYLE[evt.event_type] || EVENT_STYLE.thought;
              const payload = parsePayload(evt.payload);
              const isExpanded = expanded.has(evt.id);

              return (
                <div key={evt.id} className="relative">
                  {/* 时间线连接线 */}
                  {idx < events.length - 1 && (
                    <div
                      className="absolute left-[11px] top-8 w-0.5"
                      style={{
                        height: 'calc(100% + 4px)',
                        background: 'var(--color-border)',
                      }}
                    />
                  )}

                  <div className="flex gap-3 py-1.5">
                    {/* 节点圆点 */}
                    <div
                      className="w-[22px] h-[22px] rounded-full flex items-center justify-center flex-shrink-0 mt-0.5 z-10"
                      style={{ background: style.bg, border: `2px solid ${style.color}` }}
                    >
                      <div className="w-2 h-2 rounded-full" style={{ background: style.color }} />
                    </div>

                    {/* 事件内容 */}
                    <div
                      className="flex-1 rounded-lg p-2.5 cursor-pointer transition-colors min-w-0"
                      style={{
                        background: 'var(--color-bg-secondary)',
                        border: `1px solid ${isExpanded ? style.color : 'var(--color-border)'}`,
                      }}
                      onClick={() => toggleExpand(evt.id)}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <div className="flex items-center gap-2 min-w-0">
                          <span
                            className="text-xs px-1.5 py-0.5 rounded font-medium flex-shrink-0"
                            style={{ background: style.bg, color: style.color }}
                          >
                            {style.label}
                          </span>
                          <span className="text-xs truncate" style={{ color: 'var(--color-text-secondary)' }}>
                            {evt.agent_id}
                          </span>
                        </div>
                        <div className="flex items-center gap-3 flex-shrink-0">
                          {evt.duration_ms > 0 && (
                            <span className="text-xs" style={{ color: 'var(--color-text-muted)' }}>
                              {evt.duration_ms >= 1000
                                ? `${(evt.duration_ms / 1000).toFixed(1)}s`
                                : `${evt.duration_ms}ms`}
                            </span>
                          )}
                          <span className="text-xs" style={{ color: 'var(--color-text-muted)' }}>
                            {new Date(evt.created_at).toLocaleTimeString()}
                          </span>
                        </div>
                      </div>

                      {/* 展开的 Payload 详情 */}
                      {isExpanded && payload && (
                        <div
                          className="mt-2 p-2 rounded text-xs font-mono overflow-x-auto"
                          style={{
                            background: 'var(--color-bg-tertiary)',
                            color: 'var(--color-text-secondary)',
                            maxHeight: 200,
                            overflowY: 'auto',
                          }}
                        >
                          <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                            {JSON.stringify(payload, null, 2)}
                          </pre>
                        </div>
                      )}

                      {isExpanded && !payload && (
                        <div
                          className="mt-2 p-2 rounded text-xs overflow-x-auto"
                          style={{
                            background: 'var(--color-bg-tertiary)',
                            color: 'var(--color-text-muted)',
                          }}
                        >
                          {evt.payload || '(空)'}
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}