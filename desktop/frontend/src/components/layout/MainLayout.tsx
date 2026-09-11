import React, { useState, useEffect, useCallback, useRef } from 'react';
import Sidebar, { type ViewType, navigateTo, getCurrentView, onNavigate } from './Sidebar';
import ChatView from '../chat/ChatView';
import SettingsView from '../settings/SettingsView';
import SkillMarketView from '../skills/SkillMarketView';
import WorkspaceView from '../workspaces/WorkspaceView';
import MemoryView from '../memory/MemoryView';
import MultiAgentView from '../multiagent/MultiAgentView';
import ToolsView from '../tools/ToolsView';
import DetailPanel from '../detail/DetailPanel';
import { useLayoutStore, type DetailTab } from '../../lib/store';

/** React Error Boundary */
class ViewErrorBoundary extends React.Component<
  { children: React.ReactNode; view: ViewType },
  { error: Error | null }
> {
  constructor(props: { children: React.ReactNode; view: ViewType }) {
    super(props);
    this.state = { error: null };
  }

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidUpdate(prevProps: { view: ViewType }) {
    if (prevProps.view !== this.props.view) {
      this.setState({ error: null });
    }
  }

  render() {
    if (this.state.error) {
      return (
        <div style={{ padding: 40, textAlign: 'center' }}>
          <p style={{ color: '#ef4444', fontSize: 16, fontWeight: 600 }}>页面渲染错误</p>
          <p style={{ color: '#888', fontSize: 13, marginTop: 8 }}>{this.state.error.message}</p>
          <button
            onClick={() => {
              this.setState({ error: null });
              navigateTo('chat');
            }}
            style={{
              marginTop: 16,
              padding: '6px 16px',
              borderRadius: 8,
              border: 'none',
              background: 'var(--color-accent)',
              color: '#fff',
              cursor: 'pointer',
              fontSize: 13,
            }}
          >
            返回聊天
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}

/** 根据 view 类型渲染对应组件 */
function ViewContent({ view, onFilePreview }: { view: ViewType; onFilePreview?: (workspaceId: string, filePath: string, fileName: string) => void }) {
  switch (view) {
    case 'settings':
      return <SettingsView />;
    case 'skills':
      return <SkillMarketView />;
    case 'workspaces':
      return <WorkspaceView />;
    case 'memory':
      return <MemoryView />;
    case 'multiagent':
      return <MultiAgentView />;
    case 'tools':
      return <ToolsView />;
    default:
      return <ChatView onFilePreview={onFilePreview} />;
  }
}

/** 全局导航状态订阅 hook */
function useNavigationState(): ViewType {
  const [view, setView] = useState<ViewType>(getCurrentView);

  useEffect(() => {
    const unsub = onNavigate((v) => setView(v));
    const onHash = () => setView(getCurrentView());
    window.addEventListener('hashchange', onHash);
    return () => {
      unsub();
      window.removeEventListener('hashchange', onHash);
    };
  }, []);

  return view;
}

// ===== useResizable Hook =====
interface ResizableOptions {
  initialWidth: number;
  minWidth: number;
  maxWidth: number;
  direction: 'left' | 'right'; // 拖拽手柄在面板的哪一侧
  onResize: (width: number) => void;
}

function useResizable({ initialWidth, minWidth, maxWidth, direction, onResize }: ResizableOptions) {
  const [width, setWidth] = useState(initialWidth);
  const dragging = useRef(false);
  const startX = useRef(0);
  const startW = useRef(0);

  const onMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      dragging.current = true;
      startX.current = e.clientX;
      startW.current = width;
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';

      const onMouseMove = (ev: MouseEvent) => {
        if (!dragging.current) return;
        const delta = ev.clientX - startX.current;
        // direction 'right' means the handle is on the right edge of the panel (sidebar):
        // moving right = wider. direction 'left' means the handle is on the left edge
        // of the panel (detail panel): moving right = narrower.
        const newW = direction === 'right'
          ? startW.current + delta
          : startW.current - delta;
        const clamped = Math.max(minWidth, Math.min(maxWidth, newW));
        setWidth(clamped);
        onResize(clamped);
      };

      const onMouseUp = () => {
        dragging.current = false;
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
        document.removeEventListener('mousemove', onMouseMove);
        document.removeEventListener('mouseup', onMouseUp);
      };

      document.addEventListener('mousemove', onMouseMove);
      document.addEventListener('mouseup', onMouseUp);
    },
    [width, minWidth, maxWidth, direction, onResize]
  );

  // Sync external width changes
  useEffect(() => {
    setWidth(initialWidth);
  }, [initialWidth]);

  return { width, onMouseDown };
}

export default function MainLayout() {
  const currentView = useNavigationState();

  const {
    sidebarWidth,
    detailWidth,
    detailCollapsed,
    detailActiveTab,
    setSidebarWidth,
    setDetailWidth,
    setDetailCollapsed,
    toggleDetailCollapsed,
    setDetailActiveTab,
  } = useLayoutStore();

  // Sidebar resize
  const sidebarResize = useResizable({
    initialWidth: sidebarWidth,
    minWidth: 200,
    maxWidth: 380,
    direction: 'right',
    onResize: setSidebarWidth,
  });

  // Detail panel resize
  const detailResize = useResizable({
    initialWidth: detailWidth,
    minWidth: 300,
    maxWidth: 600,
    direction: 'left',
    onResize: setDetailWidth,
  });

  const handleOpenDetailTab = useCallback(
    (tab: DetailTab) => {
      setDetailActiveTab(tab);
    },
    [setDetailActiveTab]
  );

  const handleFilePreview = useCallback(
    (workspaceId: string, filePath: string, fileName: string) => {
      setDetailActiveTab('files');
      useLayoutStore.getState().setPreviewFile({
        workspaceId, filePath, fileName,
      });
    },
    [setDetailActiveTab]
  );

  const isChatView = currentView === 'chat';

  return (
    <div className="h-full flex" style={{ background: 'var(--color-bg-primary)' }}>
      {/* === 左侧导航栏（可拖拽宽度） === */}
      <aside
        style={{
          width: sidebarResize.width,
          minWidth: sidebarResize.width,
          maxWidth: sidebarResize.width,
          transition: 'none',
          flexShrink: 0,
        }}
      >
        <Sidebar
          currentView={currentView}
          onNavigate={navigateTo}
          onOpenDetailTab={isChatView ? handleOpenDetailTab : undefined}
        />
      </aside>

      {/* 侧边栏拖拽手柄 */}
      <div
        className="resize-handle resize-handle--sidebar"
        onMouseDown={sidebarResize.onMouseDown}
      />

      {/* === 中间主内容区 === */}
      <main
        className="flex-1 flex flex-col min-w-0"
        style={{ background: 'var(--color-bg-primary)' }}
      >
        <ViewErrorBoundary view={currentView}>
          <ViewContent view={currentView} onFilePreview={handleFilePreview} />
        </ViewErrorBoundary>
      </main>

      {/* === 右侧详情面板（可拖拽宽度 + 可收起）=== */}
      {isChatView && (
        <>
          {/* 详情面板拖拽手柄（仅在展开时显示） */}
          {!detailCollapsed && (
            <div
              className="resize-handle resize-handle--detail"
              onMouseDown={detailResize.onMouseDown}
            />
          )}

          {/* 收起状态：侧边浮动展开按钮 */}
          {detailCollapsed ? (
            <div
              className="detail-collapse-tab"
              onClick={toggleDetailCollapsed}
              title="展开详情面板"
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                <path
                  d="M6 4L10 8L6 12"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
              <span className="detail-collapse-tab__text">详情</span>
            </div>
          ) : (
            <DetailPanel
              width={detailResize.width}
              activeTab={detailActiveTab}
              onTabChange={setDetailActiveTab}
              onCollapse={() => setDetailCollapsed(true)}
            />
          )}
        </>
      )}

      {/* DEBUG 指示器 */}
      <div
        style={{
          position: 'fixed',
          top: 4,
          right: 4,
          background: '#000',
          color: '#0f0',
          padding: '2px 8px',
          borderRadius: 4,
          fontSize: 10,
          zIndex: 9999,
          fontFamily: 'monospace',
          opacity: 0.6,
        }}
      >
        view={currentView}
      </div>
    </div>
  );
}