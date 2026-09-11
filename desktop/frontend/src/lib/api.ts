import { wsClient } from './ws';

// ===== Auth API =====
export async function register(username: string, password: string) {
  return wsClient.call<{ token: string; user_id: string; username: string }>('auth.register', { username, password });
}

export async function login(username: string, password: string) {
  return wsClient.call<{ token: string; user_id: string; username: string }>('auth.login', { username, password });
}

// ===== Sessions API =====
export async function fetchSessions() {
  return wsClient.call<Array<{ id: string; title: string; workspace_id: string; created_at: string; updated_at: string }>>('sessions.list');
}

export async function fetchMessages(sessionId: string) {
  return wsClient.call<Array<{ id: string; session_id: string; role: string; content: string; token_count: number; blocks: string; created_at: string }>>('sessions.get', { id: sessionId });
}

export async function deleteSession(sessionId: string) {
  return wsClient.call<{ status: string }>('sessions.delete', { id: sessionId });
}

// ===== Workspaces API =====
export async function fetchWorkspaces() {
  return wsClient.call<Array<{ id: string; name: string }>>('workspaces.list');
}

export async function createWorkspace(name: string) {
  return wsClient.call<{ id: string; name: string }>('workspaces.create', { name });
}

// ===== Chat API (WebSocket streaming) =====
export function streamChat(
  prompt: string,
  sessionId: string,
  workspaceId: string,
  modelName: string,
  callbacks: {
    onToken: (token: string) => void;
    onMeta?: (meta: { run_id: string; session_id: string }) => void;
    onDone: (data: { session_id: string; elapsed_ms: number; token_count: number }) => void;
    onError: (error: string) => void;
    onEvent?: (type: string, data: unknown) => void;
  }
): () => void {
  console.log('[api] streamChat prompt=%s session_id=%s workspace_id=%s model=%s', prompt, sessionId, workspaceId, modelName);
  return wsClient.stream('chat.send', {
    prompt,
    session_id: sessionId || '',
    workspace_id: workspaceId,
    model_name: modelName,
    title: prompt.slice(0, 50),
  }, {
    onToken: callbacks.onToken as (data: unknown) => void,
    onMeta: callbacks.onMeta,
    onDone: callbacks.onDone,
    onError: (err) => {
      console.log('[api] streamChat onError received:', err);
      callbacks.onError(err);
    },
    onEvent: callbacks.onEvent,
  });
}

// ===== Cancel API =====
export async function cancelChat(runId: string) {
  return wsClient.call<{ status: string }>('chat.cancel', { run_id: runId });
}

// ===== Tools API =====
export interface ToolDef {
  name: string;
  description: string;
  parameters?: Record<string, unknown>;
}

export async function fetchTools() {
  return wsClient.call<ToolDef[]>('tools.list');
}
export async function sendConfirmation(callId: string, response: string) {
  return wsClient.call<{ status: string }>('chat.confirm', { call_id: callId, response });
}

// ===== Token Stats =====
export async function fetchTokenStats() {
  return wsClient.call<Array<{ model_name: string; total_input: number; total_output: number; total_tokens: number }>>('stats.tokens');
}

// ===== Models API =====
export interface ModelConfig {
  id: string;
  name: string;
  base_url: string;
  api_format?: string;
  enabled: boolean;
  is_default?: boolean;
}

export async function fetchModels() {
  return wsClient.call<ModelConfig[]>('models.list');
}

export async function createModel(data: { name: string; base_url: string; api_format?: string; api_key?: string }) {
  return wsClient.call<ModelConfig>('models.create', {
    name: data.name,
    base_url: data.base_url,
    api_format: data.api_format || 'openai',
    api_key: data.api_key || '',
  });
}

export async function updateModel(id: string, data: { name?: string; base_url?: string; api_format?: string; enabled?: boolean; api_key?: string }) {
  return wsClient.call<{ status: string }>('models.update', { id, ...data });
}

export async function setDefaultModel(id: string) {
  return wsClient.call<{ status: string }>('models.set_default', { id });
}

// ===== Skills API =====
export interface SkillInfo {
  id: string;
  workspace_id: string;
  name: string;
  version: string;
  enabled: boolean;
  installed_at: string;
}

export async function fetchSkills() {
  return wsClient.call<SkillInfo[]>('skills.list');
}

export async function installSkill(name: string, version: string) {
  return wsClient.call<SkillInfo>('skills.install', { name, version });
}

// ===== Trace API =====
export interface TraceEvent {
  id: string;
  session_id: string;
  agent_id: string;
  event_type: 'tool_call' | 'thought' | 'response' | 'error' | 'agent_start' | 'agent_end';
  payload: string;
  duration_ms: number;
  created_at: string;
}

export async function fetchTraces(sessionId: string) {
  return wsClient.call<TraceEvent[]>('traces.get', { session_id: sessionId });
}

// ===== Memory API =====
export interface MemoryFragment {
  id: string;
  workspace_id: string;
  title?: string;
  category?: string;
  content: string;
  source_session_id: string;
  enabled: boolean;
  created_at: string;
}

export async function fetchMemories(workspaceId: string) {
  return wsClient.call<MemoryFragment[]>('memory.list', { workspace_id: workspaceId });
}

// ===== Token Stats =====
export interface TokenStats {
  model_name?: string;
  model?: string;
  prompt_tokens?: number;
  completion_tokens?: number;
  total_input?: number;
  total_output?: number;
  total_tokens: number;
}

// ===== Workspace Files =====
export interface WorkspaceFile {
  path: string;
  size?: number;
  modified_at?: string;
}

export async function getFiles(workspaceId: string) {
  return wsClient.call<WorkspaceFile[]>('workspaces.files', { workspace_id: workspaceId });
}

/** 获取文件内容用于预览（通过 WebSocket RPC，无 CORS 问题）。
 *  文本文件返回原始文本，二进制文件（图片等）返回 base64 编码后的内容。 */
export interface FileContent {
  file_name: string;
  file_path: string;
  content_type: string;
  content: string;
  size: number;
  workspace_id: string;
}

export async function getFileContent(workspaceId: string, filePath: string): Promise<FileContent> {
  return wsClient.call<FileContent>('workspaces.file_content', {
    workspace_id: workspaceId,
    file_path: filePath,
  });
}

/** 获取 Office 文档的 HTML 预览（通过 WebSocket RPC，无 CORS 问题）。 */
export interface FilePreview {
  file_name: string;
  file_path: string;
  content_type: string;
  size: number;
  workspace_id: string;
  preview_available: boolean;
  preview_html: string;
  preview_error: string;
}

export async function getFilePreview(workspaceId: string, filePath: string): Promise<FilePreview> {
  return wsClient.call<FilePreview>('workspaces.file_preview', {
    workspace_id: workspaceId,
    file_path: filePath,
  });
}

/** 通过 WebSocket 下载文件内容并在浏览器中触发下载。
 *  走 WebSocket RPC 获取 base64 编码的文件内容，转换为 Blob 后触发下载，
 *  完全不需要 HTTP fetch，无 CORS 问题。 */
export async function downloadFile(workspaceId: string, filePath: string, fileName: string): Promise<void> {
  const content = await getFileContent(workspaceId, filePath);
  // 将 base64 内容解码为二进制
  const byteChars = atob(content.content);
  const byteNums = new Array(byteChars.length);
  for (let i = 0; i < byteChars.length; i++) {
    byteNums[i] = byteChars.charCodeAt(i);
  }
  const byteArr = new Uint8Array(byteNums);
  const blob = new Blob([byteArr], { type: content.content_type });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

export async function toggleMemory(memoryId: string, enabled: boolean) {
  return wsClient.call<MemoryFragment>('memory.toggle', { memory_id: memoryId, enabled });
}

export async function createMemory(workspaceId: string, content: string) {
  return wsClient.call<MemoryFragment>('memory.create', { workspace_id: workspaceId, content });
}

// ===== Multi-Agent Dispatch API =====
export interface ExpertInfo {
  id: string;
  name: string;
  description: string;
  emoji: string;
  skills: string[];
  model_name: string;
}

export async function fetchExperts() {
  return wsClient.call<ExpertInfo[]>('experts.list');
}

export interface DispatchCallbacks {
  onMeta?: (data: { session_id: string; experts: Array<{ id: string; name: string; emoji: string }> }) => void;
  onToken?: (data: { expert_id: string; content: string }) => void;
  onEvent?: (type: string, data: unknown) => void;
  onDone?: (data: { session_id: string; output: string; sub_results: Array<{ expert_id: string; expert_name: string; output: string; error?: string; elapsed_ms: number }>; total_time_ms: number }) => void;
  onError?: (error: string) => void;
}

export function dispatchRun(
  prompt: string,
  sessionId: string,
  workspaceId: string,
  expertIds: string[],
  callbacks: DispatchCallbacks
): () => void {
  return wsClient.stream('dispatch.run', {
    prompt,
    session_id: sessionId || '',
    workspace_id: workspaceId,
    expert_ids: expertIds,
  }, {
    onMeta: callbacks.onMeta as (data: Record<string, unknown>) => void,
    onToken: callbacks.onToken as (data: unknown) => void,
    onEvent: callbacks.onEvent,
    onDone: callbacks.onDone as (data: Record<string, unknown>) => void,
    onError: callbacks.onError,
  });
}