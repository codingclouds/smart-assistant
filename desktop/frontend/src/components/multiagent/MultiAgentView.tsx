import { useState, useEffect, useRef } from 'react';
import { fetchExperts, dispatchRun, type ExpertInfo } from '../../lib/api';
import { useSessionStore, useWorkspaceStore } from '../../lib/store';

interface Expert extends ExpertInfo {
  icon: string;
  status: 'idle' | 'running' | 'done' | 'error';
  output: string;
  error: string;
}

const STATUS_STYLE: Record<string, { label: string; color: string }> = {
  idle: { label: '待命', color: '#71717a' },
  running: { label: '执行中', color: '#7c8aff' },
  done: { label: '已完成', color: '#4ade80' },
  error: { label: '异常', color: '#f87171' },
};

const WORKFLOW_TEMPLATES = [
  {
    id: 'code-review',
    name: '代码审查',
    description: '代码专家 → 安全审计 → 测试验证',
    expertIds: ['code-expert', 'security-auditor', 'test-engineer'],
  },
  {
    id: 'doc-gen',
    name: '文档生成',
    description: '架构分析 → 文档撰写',
    expertIds: ['architect', 'doc-writer'],
  },
  {
    id: 'data-pipeline',
    name: '数据处理',
    description: '数据清洗 → 统计分析',
    expertIds: ['data-analyst', 'doc-writer'],
  },
  {
    id: 'full-audit',
    name: '全面审计',
    description: '架构评估 + 安全审计 + 代码审查 → 汇总',
    expertIds: ['architect', 'security-auditor', 'code-expert', 'test-engineer'],
  },
];

export default function MultiAgentView() {
  const [experts, setExperts] = useState<Expert[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [activeWorkflow, setActiveWorkflow] = useState<string | null>(null);
  const [isRunning, setIsRunning] = useState(false);
  const [prompt, setPrompt] = useState('');
  const [combinedOutput, setCombinedOutput] = useState('');
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const cancelRef = useRef<(() => void) | null>(null);

  const activeSessionId = useSessionStore((s) => s.activeSessionId);
  const activeWorkspaceId = useWorkspaceStore((s) => s.activeWorkspaceId);

  useEffect(() => {
    fetchExperts()
      .then((list) => {
        setExperts(
          list.map((e) => ({
            ...e,
            icon: e.emoji,
            status: 'idle' as const,
            output: '',
            error: '',
          }))
        );
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  const toggleExpert = (id: string) => {
    if (isRunning) return;
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const selectWorkflow = (wfId: string) => {
    setActiveWorkflow(wfId);
    const template = WORKFLOW_TEMPLATES.find((w) => w.id === wfId);
    if (template) {
      setSelectedIds(new Set(template.expertIds));
    }
  };

  const runDispatch = () => {
    if (isRunning || selectedIds.size === 0 || !prompt.trim()) return;
    setIsRunning(true);
    setCombinedOutput('');
    setErrorMsg(null);

    setExperts((prev) =>
      prev.map((e) => ({
        ...e,
        status: 'idle' as const,
        output: '',
        error: '',
      }))
    );

    const cancel = dispatchRun(prompt, activeSessionId || '', activeWorkspaceId, [...selectedIds], {
      onMeta: (data) => {
        if (!activeSessionId && data.session_id) {
          useSessionStore.getState().addSession({
            id: data.session_id,
            title: prompt.slice(0, 30),
            workspace_id: activeWorkspaceId,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          });
        }
      },
      onToken: (data) => {
        const { expert_id, content } = data as { expert_id: string; content: string };
        setExperts((prev) =>
          prev.map((e) =>
            e.id === expert_id
              ? { ...e, output: e.output + content, status: 'running' }
              : e
          )
        );
      },
      onEvent: (type, data) => {
        const d = data as Record<string, unknown>;
        if (type === 'expert_start') {
          setExperts((prev) =>
            prev.map((e) =>
              e.id === d.expert_id ? { ...e, status: 'running', output: '' } : e
            )
          );
        } else if (type === 'expert_done') {
          setExperts((prev) =>
            prev.map((e) =>
              e.id === d.expert_id
                ? { ...e, status: 'done', output: (d.output as string) || e.output }
                : e
            )
          );
        }
      },
      onDone: (data) => {
        setIsRunning(false);
        cancelRef.current = null;
        setCombinedOutput(data.output || '');
        setExperts((prev) =>
          prev.map((e) =>
            selectedIds.has(e.id) && e.status !== 'done' && e.status !== 'error'
              ? { ...e, status: 'done' }
              : e
          )
        );
      },
      onError: (err) => {
        setIsRunning(false);
        cancelRef.current = null;
        setErrorMsg(err || '调度执行失败');
      },
    });
    cancelRef.current = cancel;
  };

  const resetAll = () => {
    if (isRunning) return;
    setSelectedIds(new Set());
    setActiveWorkflow(null);
    setPrompt('');
    setCombinedOutput('');
    setErrorMsg(null);
    setExperts((prev) =>
      prev.map((e) => ({ ...e, status: 'idle' as const, output: '', error: '' }))
    );
  };

  const selectedCount = selectedIds.size;
  const statusCounts = experts.reduce(
    (acc, e) => {
      acc[e.status] = (acc[e.status] || 0) + 1;
      return acc;
    },
    {} as Record<string, number>
  );

  if (loading) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <p style={{ color: 'var(--color-text-muted)' }}>加载专家池...</p>
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col items-center overflow-y-auto p-8">
      <div className="w-full max-w-3xl">
        {/* 标题栏 */}
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1
              className="text-2xl font-bold mb-1 tracking-tight"
              style={{ color: 'var(--color-text-primary)' }}
            >
              多 Agent 调度
            </h1>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
              选择专家 Agent 协同执行复杂任务
            </p>
          </div>
          <div className="flex gap-2.5">
            {!isRunning && (
              <button
                onClick={resetAll}
                className="text-sm px-4 py-2 rounded-xl transition-all hover:bg-white/5"
                style={{
                  background: 'var(--color-bg-secondary)',
                  color: 'var(--color-text-secondary)',
                  border: '1px solid var(--color-border)',
                }}
              >
                重置
              </button>
            )}
            {isRunning ? (
              <button
                onClick={() => {
                  if (cancelRef.current) {
                    cancelRef.current();
                    cancelRef.current = null;
                  }
                  setIsRunning(false);
                  setExperts((prev) =>
                    prev.map((e) =>
                      e.status === 'running'
                        ? { ...e, status: 'idle' as const, output: '', error: '' }
                        : e
                    )
                  );
                }}
                className="text-sm px-5 py-2 rounded-xl font-medium transition-all active:scale-95"
                style={{
                  background: 'var(--color-error)',
                  color: '#fff',
                  boxShadow: '0 2px 12px rgba(248,113,113,0.3)',
                }}
              >
                ⏹ 停止
              </button>
            ) : (
              <button
                onClick={runDispatch}
                disabled={selectedCount === 0 || !prompt.trim()}
                className="text-sm px-5 py-2 rounded-xl font-medium transition-all disabled:opacity-40 hover:brightness-110 active:scale-95"
                style={{
                  background: 'var(--color-accent)',
                  color: '#fff',
                  boxShadow: '0 2px 8px rgba(124,138,255,0.25)',
                }}
              >
                ▶ 启动调度
              </button>
            )}
          </div>
        </div>

        {/* 错误横幅 */}
        {errorMsg && (
          <div
            className="flex items-center justify-between px-4 py-3 rounded-2xl mb-6 text-sm"
            style={{
              background: 'var(--color-error-muted)',
              color: 'var(--color-error)',
              border: '1px solid rgba(248,113,113,0.25)',
            }}
          >
            <span className="flex items-center gap-2">
              <span className="text-xs">⚠</span>
              {errorMsg}
            </span>
            <button
              onClick={() => setErrorMsg(null)}
              className="text-xs px-2 py-1 rounded-lg hover:bg-white/5 transition-all"
              style={{ color: 'var(--color-error)' }}
            >
              ✕
            </button>
          </div>
        )}

        {/* 输入区 */}
        <div
          className="rounded-2xl p-4 mb-6 transition-all"
          style={{
            background: 'var(--color-bg-secondary)',
            border: '1px solid var(--color-border)',
          }}
        >
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder="输入任务描述，选中的专家将依次处理..."
            disabled={isRunning}
            rows={3}
            className="w-full bg-transparent border-none outline-none resize-none text-[15px] leading-relaxed placeholder:text-[var(--color-text-muted)]"
            style={{ color: 'var(--color-text-primary)' }}
          />
        </div>

        {/* 状态概览 */}
        <div className="grid grid-cols-4 gap-3 mb-8">
          {(['idle', 'running', 'done', 'error'] as const).map((s) => (
            <div
              key={s}
              className="rounded-2xl px-4 py-3 text-center"
              style={{
                background: 'var(--color-bg-secondary)',
                border: '1px solid var(--color-border)',
              }}
            >
              <p className="text-2xl font-bold mb-0.5" style={{ color: STATUS_STYLE[s].color }}>
                {statusCounts[s] || 0}
              </p>
              <p className="text-xs" style={{ color: 'var(--color-text-muted)' }}>
                {STATUS_STYLE[s].label}
              </p>
            </div>
          ))}
        </div>

        {/* 工作流模板 */}
        <div className="mb-8">
          <h2
            className="text-sm font-semibold mb-3 px-1"
            style={{ color: 'var(--color-text-secondary)' }}
          >
            预设工作流
          </h2>
          <div className="grid grid-cols-2 gap-2.5">
            {WORKFLOW_TEMPLATES.map((wf) => (
              <button
                key={wf.id}
                onClick={() => selectWorkflow(wf.id)}
                disabled={isRunning}
                className="text-left p-4 rounded-2xl transition-all disabled:opacity-50 hover:border-[var(--color-border-light)]"
                style={{
                  background:
                    activeWorkflow === wf.id
                      ? 'var(--color-accent-soft)'
                      : 'var(--color-bg-secondary)',
                  border: `1px solid ${
                    activeWorkflow === wf.id
                      ? 'rgba(124,138,255,0.35)'
                      : 'var(--color-border)'
                  }`,
                }}
              >
                <p
                  className="text-sm font-medium mb-0.5"
                  style={{ color: 'var(--color-text-primary)' }}
                >
                  {wf.name}
                </p>
                <p className="text-xs mb-2" style={{ color: 'var(--color-text-secondary)' }}>
                  {wf.description}
                </p>
                <span
                  className="text-[11px] px-2 py-0.5 rounded-md font-medium"
                  style={{
                    background: 'var(--color-bg-tertiary)',
                    color: 'var(--color-text-muted)',
                  }}
                >
                  {wf.expertIds.length} 位专家
                </span>
              </button>
            ))}
          </div>
        </div>

        {/* 执行进度 */}
        {isRunning && (
          <div className="mb-8">
            <h2
              className="text-sm font-semibold mb-3 px-1"
              style={{ color: 'var(--color-text-secondary)' }}
            >
              执行进度
            </h2>
            <div
              className="rounded-2xl p-5"
              style={{
                background: 'var(--color-bg-secondary)',
                border: '1px solid var(--color-border)',
              }}
            >
              <div className="flex items-center gap-4 flex-wrap">
                {experts
                  .filter((e) => selectedIds.has(e.id))
                  .map((e, idx) => (
                    <div key={e.id} className="flex items-center gap-2">
                      {idx > 0 && (
                        <span
                          className="text-xs mx-1"
                          style={{ color: 'var(--color-text-muted)' }}
                        >
                          →
                        </span>
                      )}
                      <div className="flex flex-col items-center gap-1">
                        <div
                          className="w-10 h-10 rounded-full flex items-center justify-center text-lg"
                          style={{
                            background: `${STATUS_STYLE[e.status].color}15`,
                            border: `2px solid ${STATUS_STYLE[e.status].color}`,
                          }}
                        >
                          {e.icon || e.emoji}
                        </div>
                        <span
                          className="text-[11px] font-medium"
                          style={{ color: STATUS_STYLE[e.status].color }}
                        >
                          {STATUS_STYLE[e.status].label}
                        </span>
                      </div>
                    </div>
                  ))}
              </div>
            </div>
          </div>
        )}

        {/* 专家 Agent 池 */}
        <div className="mb-8">
          <div className="flex items-center justify-between mb-3 px-1">
            <h2
              className="text-sm font-semibold"
              style={{ color: 'var(--color-text-secondary)' }}
            >
              专家 Agent 池 ({experts.length})
            </h2>
            {selectedCount > 0 && (
              <span
                className="text-xs px-2.5 py-1 rounded-full font-medium"
                style={{
                  background: 'var(--color-accent-muted)',
                  color: 'var(--color-accent)',
                }}
              >
                已选 {selectedCount} 位
              </span>
            )}
          </div>

          <div className="space-y-2.5">
            {experts.map((expert) => {
              const isSelected = selectedIds.has(expert.id);
              const statusStyle = STATUS_STYLE[expert.status];

              return (
                <div
                  key={expert.id}
                  onClick={() => toggleExpert(expert.id)}
                  onKeyDown={(e) => {
                    if ((e.key === 'Enter' || e.key === ' ') && !isRunning) {
                      e.preventDefault();
                      toggleExpert(expert.id);
                    }
                  }}
                  role="checkbox"
                  aria-checked={isSelected}
                  tabIndex={isRunning ? -1 : 0}
                  className="flex items-start gap-3.5 p-4 rounded-2xl transition-all hover:border-[var(--color-border-light)]"
                  style={{
                    background: isSelected
                      ? 'var(--color-accent-soft)'
                      : 'var(--color-bg-secondary)',
                    border: `1px solid ${
                      isSelected ? 'rgba(124,138,255,0.35)' : 'var(--color-border)'
                    }`,
                    cursor: isRunning ? 'default' : 'pointer',
                    opacity: isRunning ? 0.9 : 1,
                  }}
                >
                  <span className="text-2xl flex-shrink-0">{expert.icon || expert.emoji}</span>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span
                        className="text-sm font-semibold"
                        style={{ color: 'var(--color-text-primary)' }}
                      >
                        {expert.name}
                      </span>
                      <span
                        className="flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full font-medium"
                        style={{
                          background: `${statusStyle.color}15`,
                          color: statusStyle.color,
                        }}
                      >
                        <span
                          className="w-1.5 h-1.5 rounded-full inline-block"
                          style={{
                            background: statusStyle.color,
                            animation:
                              expert.status === 'running'
                                ? 'pulse 1s infinite'
                                : undefined,
                          }}
                        />
                        {statusStyle.label}
                      </span>
                      {isSelected && (
                        <span
                          className="text-[11px] px-2 py-0.5 rounded-full font-medium"
                          style={{
                            background: 'var(--color-accent-muted)',
                            color: 'var(--color-accent)',
                          }}
                        >
                          已选
                        </span>
                      )}
                    </div>
                    <p
                      className="text-xs mb-2 leading-relaxed"
                      style={{ color: 'var(--color-text-secondary)' }}
                    >
                      {expert.description}
                    </p>
                    <div className="flex items-center gap-2">
                      <span
                        className="text-[11px] px-2 py-0.5 rounded-md"
                        style={{
                          background: 'var(--color-bg-tertiary)',
                          color: 'var(--color-text-muted)',
                        }}
                      >
                        {expert.model_name}
                      </span>
                      <div className="flex flex-wrap gap-1">
                        {expert.skills.slice(0, 3).map((sk) => (
                          <span
                            key={sk}
                            className="text-[11px] px-1.5 py-0.5 rounded-md"
                            style={{
                              background: 'var(--color-bg-tertiary)',
                              color: 'var(--color-text-muted)',
                            }}
                          >
                            {sk}
                          </span>
                        ))}
                        {expert.skills.length > 3 && (
                          <span
                            className="text-[11px]"
                            style={{ color: 'var(--color-text-muted)' }}
                          >
                            +{expert.skills.length - 3}
                          </span>
                        )}
                      </div>
                    </div>
                    {/* 专家执行输出预览 */}
                    {expert.output &&
                      (expert.status === 'running' || expert.status === 'done') && (
                        <div
                          className="mt-3 p-3 rounded-xl text-xs font-mono max-h-32 overflow-y-auto"
                          style={{
                            background: 'var(--color-bg-tertiary)',
                            color: 'var(--color-text-secondary)',
                            whiteSpace: 'pre-wrap',
                            wordBreak: 'break-word',
                          }}
                        >
                          {expert.output.slice(-500)}
                        </div>
                      )}
                  </div>
                  {!isRunning && (
                    <div
                      className="w-5 h-5 rounded-md border-2 flex items-center justify-center flex-shrink-0 mt-1 transition-all"
                      style={{
                        borderColor: isSelected
                          ? 'var(--color-accent)'
                          : 'var(--color-border-light)',
                        background: isSelected ? 'var(--color-accent)' : 'transparent',
                      }}
                    >
                      {isSelected && (
                        <span className="text-[11px] text-white font-bold">✓</span>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>

        {/* 合并输出结果 */}
        {combinedOutput && (
          <div className="mb-8">
            <h2
              className="text-sm font-semibold mb-3 px-1"
              style={{ color: 'var(--color-text-secondary)' }}
            >
              调度结果
            </h2>
            <div
              className="rounded-2xl p-5 text-[15px] leading-relaxed"
              style={{
                background: 'var(--color-bg-secondary)',
                border: '1px solid var(--color-border)',
                color: 'var(--color-text-primary)',
                whiteSpace: 'pre-wrap',
              }}
            >
              {combinedOutput}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}