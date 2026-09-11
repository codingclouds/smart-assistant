import { useEffect, useRef } from 'react';
import { useAuthStore, useSessionStore, useWorkspaceStore, useWSStore } from './lib/store';
import { fetchSessions, fetchWorkspaces } from './lib/api';
import { wsClient } from './lib/ws';
import LoginView from './components/auth/LoginView';
import MainLayout from './components/layout/MainLayout';

export default function App() {
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);
  const logout = useAuthStore((s) => s.logout);
  const setSessions = useSessionStore((s) => s.setSessions);
  const { setWorkspaces, setActiveWorkspace, activeWorkspaceId } = useWorkspaceStore();
  const wsState = useWSStore((s) => s.state);
  const setWSState = useWSStore((s) => s.setState);
  const dataLoadedRef = useRef(false);

  // 初始化 WebSocket 连接并监听状态
  useEffect(() => {
    const unsub = wsClient.onStateChange((state) => {
      setWSState(state);
      // 连接断开时重置数据加载标记，重连后需要重新验证 auth
      if (state === 'disconnected' || state === 'reconnecting') {
        console.log('[App] WS state -> %s, resetting dataLoaded', state);
        dataLoadedRef.current = false;
      }
    });
    wsClient.connect().catch(() => {
      // 连接失败由状态监听器处理
    });

    return () => {
      unsub();
    };
  }, [setWSState]);

  // 登录 + WS 连接就绪后加载数据
  useEffect(() => {
    if (!isLoggedIn) {
      dataLoadedRef.current = false;
      return;
    }

    if (wsState !== 'connected') return;
    if (dataLoadedRef.current) return;

    dataLoadedRef.current = true;

    Promise.all([
      fetchSessions().catch((err) => {
        // 认证失败 → 清除登录态，退回登录页
        if (err?.message?.includes('auth') || err?.message?.includes('not connected')) {
          logout();
        }
        return [];
      }),
      fetchWorkspaces().catch((err) => {
        if (err?.message?.includes('auth') || err?.message?.includes('not connected')) {
          logout();
        }
        return [];
      }),
    ]).then(([sessions, wsList]) => {
      // 防御性处理：服务端可能返回 null 而不是空数组
      setSessions(Array.isArray(sessions) ? sessions : []);
      setWorkspaces(Array.isArray(wsList) ? wsList : []);
      // 自动选中第一个工作空间；若 localStorage 存的 ID 不在列表中则清除
      const validIds = new Set(wsList.map((w: {id: string}) => w.id));
      if (activeWorkspaceId !== 'default' && !validIds.has(activeWorkspaceId)) {
        console.log('[App] localStorage workspace %s not found in server list, clearing', activeWorkspaceId);
        setActiveWorkspace(wsList.length > 0 ? wsList[0].id : 'default');
      } else if (wsList.length > 0 && (!activeWorkspaceId || activeWorkspaceId === 'default')) {
        setActiveWorkspace(wsList[0].id);
      }
    }).catch(() => {
      // 如果 Promise.all 整体失败（reject），后退登录
      logout();
    });
  }, [isLoggedIn, wsState]);

  if (!isLoggedIn) {
    return <LoginView />;
  }

  return <MainLayout />;
}