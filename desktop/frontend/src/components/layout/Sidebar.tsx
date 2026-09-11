import { useEffect, useState } from 'react';
import { useAuthStore, useSessionStore, useWorkspaceStore, type DetailTab } from '../../lib/store';
import { deleteSession, fetchTokenStats } from '../../lib/api';
import {
  IconPlus,
  IconMessageSquare,
  IconFolder,
  IconBrain,
  IconBot,
  IconPuzzle,
  IconSettings,
  IconWrench,
} from '../icons';

export type ViewType = 'chat' | 'settings' | 'skills' | 'workspaces' | 'memory' | 'multiagent' | 'tools' | '_files';

// ===== 全局导航事件 =====

type NavListener = (view: ViewType) => void;
const listeners = new Set<NavListener>();

export function onNavigate(fn: NavListener): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export function navigateTo(view: ViewType) {
  window.location.hash = view === 'chat' ? '' : view;
  listeners.forEach((fn) => fn(view));
}

export function getCurrentView(): ViewType {
  const hash = window.location.hash.replace('#', '');
  const valid: ViewType[] = ['chat', 'settings', 'skills', 'workspaces', 'memory', 'multiagent', 'tools'];
  return valid.includes(hash as ViewType) ? (hash as ViewType) : 'chat';
}

interface SidebarProps {
  currentView: ViewType;
  onNavigate: (view: ViewType) => void;
  onOpenDetailTab?: (tab: DetailTab) => void;
}

interface NavItem {
  id: ViewType;
  label: string;
  Icon: React.FC<{ size?: number }>;
  detailTab?: DetailTab;
}

const NAV_ITEMS: NavItem[] = [
  { id: 'memory', label: '长期记忆', Icon: IconBrain, detailTab: 'memory' },
  { id: 'multiagent', label: '多 Agent 调度', Icon: IconBot, detailTab: 'trace' },
  { id: '_files', label: '项目文件', Icon: IconFolder, detailTab: 'files' },
  { id: 'skills', label: 'Skill 市场', Icon: IconPuzzle },
  { id: 'tools', label: '工具能力', Icon: IconWrench },
  { id: 'settings', label: '模型配置', Icon: IconSettings },
];

/** 将会话按时间段分组 */
function groupSessions(sessions: { id: string; title: string; created_at: string; updated_at: string }[]) {
  const now = new Date();
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterdayStart = new Date(todayStart.getTime() - 86400000);
  const weekAgo = new Date(todayStart.getTime() - 7 * 86400000);
  const monthAgo = new Date(todayStart.getTime() - 30 * 86400000);

  const groups: { label: string; items: typeof sessions }[] = [
    { label: '今天', items: [] },
    { label: '昨天', items: [] },
    { label: '前7天', items: [] },
    { label: '前30天', items: [] },
    { label: '更早', items: [] },
  ];

  for (const s of sessions) {
    const d = new Date(s.updated_at || s.created_at);
    if (d >= todayStart) groups[0].items.push(s);
    else if (d >= yesterdayStart) groups[1].items.push(s);
    else if (d >= weekAgo) groups[2].items.push(s);
    else if (d >= monthAgo) groups[3].items.push(s);
    else groups[4].items.push(s);
  }

  return groups.filter((g) => g.items.length > 0);
}

function formatTokens(n: number): string {
  if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
  return String(n);
}

export default function Sidebar({ currentView, onNavigate, onOpenDetailTab }: SidebarProps) {
  const username = useAuthStore((s) => s.username);
  const logout = useAuthStore((s) => s.logout);
  const { sessions, activeSessionId, setActiveSession, removeSession } = useSessionStore();
  const { workspaces, activeWorkspaceId, setActiveWorkspace } = useWorkspaceStore();
  const [totalTokens, setTotalTokens] = useState(0);
  const [wsOpen, setWsOpen] = useState(false);

  // 加载 Token 统计
  useEffect(() => {
    fetchTokenStats()
      .then((stats) => {
        const total = Array.isArray(stats)
          ? stats.reduce((sum, s) => sum + (s.total_tokens || 0), 0)
          : 0;
        setTotalTokens(total);
      })
      .catch(() => {});
  }, [activeWorkspaceId]);

  // 按工作空间过滤会话
  const filteredSessions = (sessions || []).filter(
    (s) => s.workspace_id === activeWorkspaceId || (!s.workspace_id && activeWorkspaceId === 'default')
  );

  const grouped = groupSessions(filteredSessions);
  const activeWs = workspaces.find((w) => w.id === activeWorkspaceId);

  const handleNewChat = () => {
    onNavigate('chat');
    setActiveSession(null);
  };

  const handleDeleteSession = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    removeSession(id);
    try {
      await deleteSession(id);
    } catch {
      // ignore
    }
  };

  const handleNavClick = (item: NavItem) => {
    if (item.id === '_files') {
      if (onOpenDetailTab) onOpenDetailTab('files');
      return;
    }
    if (item.detailTab && onOpenDetailTab && currentView === 'chat') {
      onOpenDetailTab(item.detailTab);
    } else {
      onNavigate(item.id);
    }
  };

  return (
    <aside
      className="flex flex-col h-full select-none sidebar"
      style={{
        width: '100%',
        minWidth: '100%',
        background: 'var(--color-sidebar-bg)',
        color: 'var(--color-sidebar-text)',
      }}
    >
      {/* 顶部：Logo + 新对话 */}
      <div className="sidebar__header">
        <div className="sidebar__brand">
          <IconMessageSquare size={16} />
          <span className="sidebar__brand-text">小智 AI</span>
        </div>
        <button onClick={handleNewChat} className="sidebar__new-chat-btn" title="新对话">
          <IconPlus size={14} />
          <span>新对话</span>
        </button>
      </div>

      {/* === 工作空间区域 === */}
      <div className="sidebar__workspace-section">
        <p className="sidebar__section-title">工作空间</p>
        <div className="sidebar__workspace-selector">
          <button
            className="sidebar__workspace-btn"
            onClick={() => setWsOpen((v) => !v)}
            onBlur={() => setTimeout(() => setWsOpen(false), 200)}
          >
            <IconFolder size={14} />
            <span className="sidebar__workspace-name">
              {activeWs?.name || '默认空间'}
            </span>
            <svg
              width="10" height="10" viewBox="0 0 10 10"
              fill="currentColor"
              style={{ opacity: 0.5, flexShrink: 0, transform: wsOpen ? 'rotate(180deg)' : undefined }}
            >
              <path d="M2.5 3.5L5 6L7.5 3.5" stroke="currentColor" strokeWidth="1.2" fill="none" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </button>
          {wsOpen && (
            <div className="sidebar__workspace-dropdown">
              {workspaces.map((ws) => (
                <button
                  key={ws.id}
                  className={`sidebar__workspace-option${ws.id === activeWorkspaceId ? ' sidebar__workspace-option--active' : ''}`}
                  onClick={() => {
                    setActiveWorkspace(ws.id);
                    setActiveSession(null);
                    setWsOpen(false);
                  }}
                >
                  <IconFolder size={12} />
                  <span>{ws.name}</span>
                  {ws.id === activeWorkspaceId && (
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none" style={{ marginLeft: 'auto' }}>
                      <path d="M2.5 6L5 8.5L9.5 3.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                    </svg>
                  )}
                </button>
              ))}
              <div className="sidebar__workspace-divider" />
              <button
                className="sidebar__workspace-option"
                onClick={() => {
                  onNavigate('workspaces');
                  setWsOpen(false);
                }}
              >
                <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                  <path d="M6 2v8M2 6h8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
                </svg>
                <span>管理工作空间</span>
              </button>
            </div>
          )}
        </div>
      </div>

      {/* 中间：会话列表 */}
      <div className="sidebar__sessions">
        <p className="sidebar__section-title">历史对话</p>
        {grouped.length === 0 ? (
          <p className="sidebar__empty">暂无历史对话</p>
        ) : (
          grouped.map((group) => (
            <div key={group.label} className="sidebar__group">
              <p className="sidebar__group-label">{group.label}</p>
              <div className="sidebar__group-items">
                {group.items.map((s) => {
                  const isActive = s.id === activeSessionId && currentView === 'chat';
                  return (
                    <div
                      key={s.id}
                      onClick={() => {
                        onNavigate('chat');
                        setActiveSession(s.id);
                      }}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          onNavigate('chat');
                          setActiveSession(s.id);
                        }
                      }}
                      role="button"
                      tabIndex={0}
                      className={`sidebar__session-item${isActive ? ' sidebar__session-item--active' : ''}`}
                    >
                      <IconMessageSquare size={14} className="sidebar__session-icon" />
                      <span className="sidebar__session-title">{s.title || '新对话'}</span>
                      <button
                        onClick={(e) => handleDeleteSession(s.id, e)}
                        className="sidebar__session-delete"
                        title="删除"
                      >
                        ×
                      </button>
                    </div>
                  );
                })}
              </div>
            </div>
          ))
        )}
      </div>

      {/* 导航按钮区 */}
      <nav className="sidebar__nav">
        {NAV_ITEMS.map((item) => {
          const active = currentView === item.id;
          const Icon = item.Icon;
          return (
            <button
              key={item.id}
              id={`nav-btn-${item.id}`}
              onClick={() => handleNavClick(item)}
              className={`nav-item${active ? ' nav-item--active' : ''}`}
            >
              <span className="nav-item__icon">
                <Icon size={16} />
              </span>
              <span className="nav-item__text">{item.label}</span>
            </button>
          );
        })}
      </nav>

      {/* 底部：Token 用量 + 用户信息 */}
      <div className="sidebar__footer">
        {/* Token 用量概览 */}
        {onOpenDetailTab && totalTokens > 0 && (
          <button
            className="sidebar__token-row"
            onClick={() => onOpenDetailTab('token')}
            title="查看 Token 用量详情"
          >
            <span className="sidebar__token-label">Token 用量</span>
            <span className="sidebar__token-value">{formatTokens(totalTokens)}</span>
          </button>
        )}

        <div className="sidebar__user">
          <div className="sidebar__user-avatar">
            {username?.[0]?.toUpperCase() || 'U'}
          </div>
          <span className="sidebar__user-name">{username}</span>
          <button onClick={logout} className="sidebar__logout-btn" title="退出登录">
            退出
          </button>
        </div>
      </div>
    </aside>
  );
}