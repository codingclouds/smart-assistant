import { useState, useEffect } from 'react';
import { fetchMemories, toggleMemory, createMemory, type MemoryFragment } from '../../lib/api';
import { useWorkspaceStore } from '../../lib/store';

export default function MemoryView() {
  const activeWorkspaceId = useWorkspaceStore((s) => s.activeWorkspaceId);
  const [memories, setMemories] = useState<MemoryFragment[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAdd, setShowAdd] = useState(false);
  const [newContent, setNewContent] = useState('');
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const [togglingId, setTogglingId] = useState<string | null>(null);

  useEffect(() => {
    loadMemories();
  }, [activeWorkspaceId]);

  const loadMemories = async () => {
    setLoading(true);
    try {
      const data = await fetchMemories(activeWorkspaceId);
      setMemories(data);
    } catch (err) {
      console.error('Failed to load memories:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleToggle = async (m: MemoryFragment) => {
    setTogglingId(m.id);
    try {
      const updated = await toggleMemory(m.id, !m.enabled);
      setMemories((prev) => prev.map((item) => (item.id === m.id ? updated : item)));
    } catch (err) {
      setMessage({ type: 'error', text: `操作失败: ${err}` });
    } finally {
      setTogglingId(null);
    }
  };

  const handleCreate = async () => {
    const content = newContent.trim();
    if (!content) return;

    setSaving(true);
    setMessage(null);
    try {
      const result = await createMemory(activeWorkspaceId, content);
      setMemories((prev) => [result, ...prev]);
      setNewContent('');
      setShowAdd(false);
      setMessage({ type: 'success', text: '记忆已创建' });
    } catch (err) {
      setMessage({ type: 'error', text: `创建失败: ${err}` });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex-1 flex flex-col items-center overflow-y-auto p-8">
      <div className="w-full max-w-2xl">
        {/* 标题栏 */}
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1
              className="text-2xl font-bold mb-1 tracking-tight"
              style={{ color: 'var(--color-text-primary)' }}
            >
              长期记忆
            </h1>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
              Agent 从对话中自动提取关键信息，也可手动管理
            </p>
          </div>
          <button
            onClick={() => {
              setShowAdd(true);
              setMessage(null);
            }}
            className="text-sm px-4 py-2 rounded-xl font-medium transition-all hover:brightness-110 active:scale-95"
            style={{
              background: 'var(--color-accent)',
              color: '#fff',
              boxShadow: '0 2px 8px rgba(124,138,255,0.25)',
            }}
          >
            + 新增
          </button>
        </div>

        {message && (
          <div
            className="text-sm px-4 py-3 rounded-xl mb-6"
            style={{
              background:
                message.type === 'success' ? 'var(--color-success-muted)' : 'var(--color-error-muted)',
              color: message.type === 'success' ? 'var(--color-success)' : 'var(--color-error)',
              border: `1px solid ${
                message.type === 'success'
                  ? 'rgba(74,222,128,0.2)'
                  : 'rgba(248,113,113,0.2)'
              }`,
            }}
          >
            {message.text}
          </div>
        )}

        {/* 新增表单 */}
        {showAdd && (
          <div
            className="rounded-2xl p-5 mb-6"
            style={{
              background: 'var(--color-bg-secondary)',
              border: '1px solid var(--color-border)',
            }}
          >
            <h3
              className="text-sm font-medium mb-3"
              style={{ color: 'var(--color-text-primary)' }}
            >
              添加记忆
            </h3>
            <textarea
              value={newContent}
              onChange={(e) => setNewContent(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Escape') setShowAdd(false);
              }}
              placeholder="输入要记住的内容..."
              rows={3}
              className="w-full px-3.5 py-2.5 rounded-xl text-[15px] border outline-none resize-none transition-all focus:border-[var(--color-accent)] mb-3"
              style={{
                background: 'var(--color-bg-tertiary)',
                color: 'var(--color-text-primary)',
                borderColor: 'var(--color-border)',
              }}
              autoFocus
            />
            <div className="flex gap-2.5">
              <button
                onClick={handleCreate}
                disabled={saving || !newContent.trim()}
                className="text-sm px-4 py-2 rounded-xl font-medium transition-all disabled:opacity-40 hover:brightness-110 active:scale-95"
                style={{ background: 'var(--color-accent)', color: '#fff' }}
              >
                {saving ? '保存中...' : '保存'}
              </button>
              <button
                onClick={() => setShowAdd(false)}
                className="text-sm px-4 py-2 rounded-xl transition-all hover:bg-white/5"
                style={{ color: 'var(--color-text-muted)' }}
              >
                取消
              </button>
            </div>
          </div>
        )}

        {loading && (
          <p
            className="text-sm text-center py-16"
            style={{ color: 'var(--color-text-muted)' }}
          >
            加载中...
          </p>
        )}

        {!loading && memories.length === 0 && (
          <div className="text-center py-20">
            <div
              className="w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-5"
              style={{ background: 'var(--color-bg-secondary)' }}
            >
              <span className="text-3xl">🧠</span>
            </div>
            <p className="text-[15px] mb-1.5" style={{ color: 'var(--color-text-secondary)' }}>
              暂无记忆
            </p>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
              Agent 将从对话中自动提取关键信息，或手动添加
            </p>
          </div>
        )}

        {/* 记忆列表 */}
        {!loading && memories.length > 0 && (
          <div className="space-y-2.5">
            {memories.map((m) => (
              <div
                key={m.id}
                className="flex items-start gap-3.5 px-5 py-4 rounded-2xl transition-all"
                style={{
                  background: 'var(--color-bg-secondary)',
                  border: `1px solid ${
                    m.enabled ? 'rgba(167,139,250,0.25)' : 'var(--color-border)'
                  }`,
                  opacity: m.enabled ? 1 : 0.55,
                }}
              >
                <span className="text-lg flex-shrink-0 mt-0.5">
                  {m.enabled ? '💜' : '🤍'}
                </span>
                <div className="flex-1 min-w-0">
                  <p
                    className="text-[15px] leading-relaxed"
                    style={{
                      color: m.enabled
                        ? 'var(--color-text-primary)'
                        : 'var(--color-text-muted)',
                    }}
                  >
                    {m.content}
                  </p>
                  {m.source_session_id && (
                    <p className="text-xs mt-1.5" style={{ color: 'var(--color-text-muted)' }}>
                      来源: {m.source_session_id.slice(0, 8)}
                    </p>
                  )}
                </div>
                <button
                  onClick={() => handleToggle(m)}
                  disabled={togglingId === m.id}
                  className="text-xs px-3 py-1.5 rounded-lg font-medium transition-all flex-shrink-0 disabled:opacity-50 hover:brightness-110"
                  style={{
                    background: m.enabled
                      ? 'var(--color-error-muted)'
                      : 'var(--color-success-muted)',
                    color: m.enabled ? 'var(--color-error)' : 'var(--color-success)',
                  }}
                >
                  {togglingId === m.id ? '...' : m.enabled ? '禁用' : '启用'}
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}