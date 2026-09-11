import { create } from 'zustand';

// ===== WebSocket 连接状态 =====
type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting';

interface WSState {
  state: ConnectionState;
  setState: (s: ConnectionState) => void;
}

export const useWSStore = create<WSState>((set) => ({
  state: 'disconnected',
  setState: (s) => set({ state: s }),
}));

// ===== 认证（连接级，token 仅作本地持久化标记） =====
interface AuthState {
  userId: string | null;
  username: string | null;
  isLoggedIn: boolean;
  login: (userId: string, username: string) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  userId: localStorage.getItem('userId'),
  username: localStorage.getItem('username'),
  isLoggedIn: !!localStorage.getItem('userId'),
  login: (userId, username) => {
    localStorage.setItem('userId', userId);
    localStorage.setItem('username', username);
    set({ userId, username, isLoggedIn: true });
  },
  logout: () => {
    localStorage.removeItem('userId');
    localStorage.removeItem('username');
    set({ userId: null, username: null, isLoggedIn: false });
  },
}));

// ===== 会话 =====
interface Session {
  id: string;
  title: string;
  workspace_id: string;
  created_at: string;
  updated_at: string;
}

interface SessionState {
  sessions: Session[];
  activeSessionId: string | null;
  setSessions: (sessions: Session[]) => void;
  setActiveSession: (id: string | null) => void;
  addSession: (session: Session) => void;
  removeSession: (id: string) => void;
}

export const useSessionStore = create<SessionState>((set) => ({
  sessions: [],
  activeSessionId: null,
  setSessions: (sessions) => set({ sessions }),
  setActiveSession: (id) => set({ activeSessionId: id }),
  addSession: (session) =>
    set((s) => ({ sessions: [session, ...s.sessions], activeSessionId: session.id })),
  removeSession: (id) =>
    set((s) => ({
      sessions: s.sessions.filter((x) => x.id !== id),
      activeSessionId: s.activeSessionId === id ? null : s.activeSessionId,
    })),
}));

// ===== 工作空间 =====
interface Workspace {
  id: string;
  name: string;
}

interface WorkspaceState {
  workspaces: Workspace[];
  activeWorkspaceId: string;
  setWorkspaces: (workspaces: Workspace[]) => void;
  setActiveWorkspace: (id: string) => void;
  addWorkspace: (ws: Workspace) => void;
  removeWorkspace: (id: string) => void;
}

export const useWorkspaceStore = create<WorkspaceState>((set) => ({
  workspaces: [],
  activeWorkspaceId: localStorage.getItem('activeWorkspaceId') || 'default',
  setWorkspaces: (workspaces) => set({ workspaces }),
  setActiveWorkspace: (id) => {
    localStorage.setItem('activeWorkspaceId', id);
    set({ activeWorkspaceId: id });
  },
  addWorkspace: (ws) => set((s) => ({ workspaces: [...s.workspaces, ws] })),
  removeWorkspace: (id) =>
    set((s) => ({
      workspaces: s.workspaces.filter((w) => w.id !== id),
      activeWorkspaceId: s.activeWorkspaceId === id ? 'default' : s.activeWorkspaceId,
    })),
}));

// ===== 对话 UI =====
interface ChatUIState {
  isStreaming: boolean;
  streamingText: string;
  error: string | null;
  setIsStreaming: (v: boolean) => void;
  appendStreamText: (text: string) => void;
  resetStreamText: () => void;
  setError: (err: string | null) => void;
}

export const useChatUIStore = create<ChatUIState>((set) => ({
  isStreaming: false,
  streamingText: '',
  error: null,
  setIsStreaming: (v) => set({ isStreaming: v }),
  appendStreamText: (text) => set((s) => ({ streamingText: s.streamingText + text })),
  resetStreamText: () => set({ streamingText: '', error: null }),
  setError: (err) => set({ error: err, isStreaming: false }),
}));

// ===== 内容块类型（用于工具调用 UI） =====
export type ContentBlock =
  | { type: 'text'; text: string }
  | { type: 'thinking'; thinking: string }
  | { type: 'tool_call'; call_id: string; name: string; arguments: string }
  | { type: 'tool_result'; call_id: string; name: string; result: string; error?: string }
  | { type: 'file_artifact'; file_path: string; file_name: string; byte_count: number; workspace_id: string; call_id: string }
  | { type: 'confirm_request'; call_id: string; question: string; options?: string[]; context?: string; response?: string };

// ===== 消息 =====
interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  blocks?: ContentBlock[];
  created_at?: string;
}

interface MessageState {
  messages: Message[];
  isLoading: boolean;
  runId: string | null;
  setMessages: (messages: Message[]) => void;
  addMessage: (message: Message) => void;
  updateLastAssistant: (content: string) => void;
  appendBlock: (block: ContentBlock) => void;
  appendThinking: (thinkingText: string) => void;
  setLoading: (v: boolean) => void;
  setRunId: (id: string | null) => void;
  resolveConfirmBlock: (callId: string, response: string) => void;
}

export const useMessageStore = create<MessageState>((set) => ({
  messages: [],
  isLoading: false,
  runId: null,
  setMessages: (messages) => set({ messages }),
  addMessage: (message) => set((s) => ({ messages: [...s.messages, message] })),
  updateLastAssistant: (content) =>
    set((s) => {
      const msgs = [...s.messages];
      const last = msgs[msgs.length - 1];
      if (last && last.role === 'assistant') {
        msgs[msgs.length - 1] = { ...last, content };
      }
      return { messages: msgs };
    }),
  appendBlock: (block) =>
    set((s) => {
      const msgs = [...s.messages];
      const last = msgs[msgs.length - 1];
      if (last && last.role === 'assistant') {
        const blocks = [...(last.blocks || [])];
        blocks.push(block);
        msgs[msgs.length - 1] = { ...last, blocks };
      }
      return { messages: msgs };
    }),
  appendThinking: (thinkingText) =>
    set((s) => {
      const msgs = [...s.messages];
      const last = msgs[msgs.length - 1];
      if (last && last.role === 'assistant') {
        const blocks = [...(last.blocks || [])];
        // streaming: append to last thinking block if it exists
        const lastBlock = blocks[blocks.length - 1];
        if (lastBlock && lastBlock.type === 'thinking') {
          blocks[blocks.length - 1] = { ...lastBlock, thinking: lastBlock.thinking + thinkingText };
        } else {
          blocks.push({ type: 'thinking', thinking: thinkingText });
        }
        msgs[msgs.length - 1] = { ...last, blocks };
      }
      return { messages: msgs };
    }),
  setLoading: (v) => set({ isLoading: v }),
  setRunId: (id) => set({ runId: id }),
  resolveConfirmBlock: (callId, response) =>
    set((s) => {
      const msgs = [...s.messages];
      // 从最后一条消息开始向前查找（confirm_request block 总是在最近的消息中）
      for (let i = msgs.length - 1; i >= 0; i--) {
        const blocks = msgs[i].blocks;
        if (!blocks) continue;
        const idx = blocks.findIndex(
          (b) => b.type === 'confirm_request' && 'call_id' in b && b.call_id === callId
        );
        if (idx !== -1) {
          const updated = [...blocks];
          updated[idx] = { ...updated[idx], response } as ContentBlock;
          msgs[i] = { ...msgs[i], blocks: updated };
          break;
        }
      }
      return { messages: msgs };
    }),
}));

// ===== 三栏布局拖拽宽度 =====
interface LayoutState {
  sidebarWidth: number;
  detailWidth: number;
  detailCollapsed: boolean;
  detailActiveTab: DetailTab;
  previewFile: PreviewFileEntry | null;
  setSidebarWidth: (w: number) => void;
  setDetailWidth: (w: number) => void;
  setDetailCollapsed: (v: boolean) => void;
  toggleDetailCollapsed: () => void;
  setDetailActiveTab: (tab: DetailTab) => void;
  setPreviewFile: (entry: PreviewFileEntry | null) => void;
}

export type DetailTab = 'trace' | 'files' | 'memory' | 'token';

export interface PreviewFileEntry {
  workspaceId: string;
  filePath: string;
  fileName: string;
}

export const useLayoutStore = create<LayoutState>((set) => ({
  sidebarWidth: 260,
  detailWidth: 380,
  detailCollapsed: true,
  detailActiveTab: 'trace',
  previewFile: null,
  setSidebarWidth: (w) => set({ sidebarWidth: Math.max(200, Math.min(380, w)) }),
  setDetailWidth: (w) => set({ detailWidth: Math.max(300, Math.min(600, w)) }),
  setDetailCollapsed: (v) => set({ detailCollapsed: v }),
  toggleDetailCollapsed: () => set((s) => ({ detailCollapsed: !s.detailCollapsed })),
  setDetailActiveTab: (tab) => set({ detailActiveTab: tab, detailCollapsed: false }),
  setPreviewFile: (entry) => set({
    previewFile: entry,
    detailActiveTab: entry ? 'files' : undefined,
    detailCollapsed: entry ? false : undefined,
  }),
}));