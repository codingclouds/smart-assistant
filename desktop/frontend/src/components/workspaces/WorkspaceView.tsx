import { useState, useEffect } from 'react';
import { fetchWorkspaces, createWorkspace } from '../../lib/api';
import { useWorkspaceStore } from '../../lib/store';

export default function WorkspaceView() {
  const { workspaces, activeWorkspaceId, setWorkspaces, setActiveWorkspace, addWorkspace } =
    useWorkspaceStore();
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [creating, setCreating] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  useEffect(() => {
    loadWorkspaces();
  }, []);

  const loadWorkspaces = async () => {
    try {
      const data = await fetchWorkspaces();
      setWorkspaces(data);
    } catch (err) {
      console.error('Failed to load workspaces:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    const name = newName.trim();
    if (!name) return;

    setCreating(true);
    setMessage(null);
    try {
      const result = await createWorkspace(name);
      addWorkspace(result);
      setNewName('');
      setShowCreate(false);
      setMessage({ type: 'success', text: `工作空间 "${name}" 创建成功` });
    } catch (err) {
      setMessage({ type: 'error', text: `创建失败: ${err}` });
    } finally {
      setCreating(false);
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
              工作空间
            </h1>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
              管理项目空间，隔离对话、记忆与配置
            </p>
          </div>
          <button
            onClick={() => {
              setShowCreate(true);
              setMessage(null);
            }}
            className="text-sm px-4 py-2 rounded-xl font-medium transition-all hover:brightness-110 active:scale-95"
            style={{
              background: 'var(--color-accent)',
              color: '#fff',
              boxShadow: '0 2px 8px rgba(124,138,255,0.25)',
            }}
          >
            + 新建
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

        {/* 创建表单 */}
        {showCreate && (
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
              新建工作空间
            </h3>
            <div className="flex gap-2.5">
              <input
                type="text"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleCreate();
                  if (e.key === 'Escape') setShowCreate(false);
                }}
                placeholder="输入工作空间名称"
                className="flex-1 px-3.5 py-2.5 rounded-xl text-[15px] border outline-none transition-all focus:border-[var(--color-accent)]"
                style={{
                  background: 'var(--color-bg-tertiary)',
                  color: 'var(--color-text-primary)',
                  borderColor: 'var(--color-border)',
                }}
                autoFocus
              />
              <button
                onClick={handleCreate}
                disabled={creating || !newName.trim()}
                className="text-sm px-5 py-2.5 rounded-xl font-medium transition-all disabled:opacity-40 hover:brightness-110 active:scale-95"
                style={{
                  background: 'var(--color-accent)',
                  color: '#fff',
                }}
              >
                {creating ? '创建中...' : '创建'}
              </button>
              <button
                onClick={() => setShowCreate(false)}
                className="text-sm px-4 py-2.5 rounded-xl transition-all hover:bg-white/5"
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

        {!loading && workspaces.length === 0 && (
          <div className="text-center py-20">
            <div
              className="w-16 h-16 rounded-2xl flex items-center justify-center mx-auto mb-5"
              style={{ background: 'var(--color-bg-secondary)' }}
            >
              <span className="text-3xl">📁</span>
            </div>
            <p className="text-[15px] mb-1.5" style={{ color: 'var(--color-text-secondary)' }}>
              暂无工作空间
            </p>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
              创建项目空间以隔离对话、记忆与模型配置
            </p>
          </div>
        )}

        {/* 工作空间卡片列表 */}
        {!loading && workspaces.length > 0 && (
          <div className="space-y-2.5">
            {workspaces.map((ws) => {
              const isActive = ws.id === activeWorkspaceId;
              return (
                <div
                  key={ws.id}
                  onClick={() => setActiveWorkspace(ws.id)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      setActiveWorkspace(ws.id);
                    }
                  }}
                  role="button"
                  tabIndex={0}
                  className="flex items-center justify-between px-5 py-4 rounded-2xl cursor-pointer transition-all hover:border-[var(--color-border-light)]"
                  style={{
                    background:
                      isActive ? 'var(--color-accent-soft)' : 'var(--color-bg-secondary)',
                    border: `1px solid ${
                      isActive ? 'rgba(124,138,255,0.35)' : 'var(--color-border)'
                    }`,
                  }}
                >
                  <div className="flex items-center gap-3.5 min-w-0">
                    <span className="text-xl">{isActive ? '📂' : '📁'}</span>
                    <div className="min-w-0">
                      <p
                        className="text-[15px] font-medium truncate"
                        style={{
                          color: isActive
                            ? 'var(--color-accent)'
                            : 'var(--color-text-primary)',
                        }}
                      >
                        {ws.name}
                      </p>
                      <p className="text-xs mt-0.5" style={{ color: 'var(--color-text-muted)' }}>
                        {isActive ? '当前空间' : `ID: ${ws.id.slice(0, 8)}`}
                      </p>
                    </div>
                  </div>
                  {isActive && (
                    <span
                      className="text-xs px-2.5 py-1 rounded-full font-medium"
                      style={{
                        background: 'var(--color-accent-muted)',
                        color: 'var(--color-accent)',
                      }}
                    >
                      当前
                    </span>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}