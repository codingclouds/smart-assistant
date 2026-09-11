import { useEffect, useRef, useState } from 'react';
import { useSessionStore, useMessageStore, useWorkspaceStore, useChatUIStore, useWSStore, type ContentBlock } from '../../lib/store';
import { fetchMessages, fetchTokenStats, fetchModels, streamChat, cancelChat, type ModelConfig } from '../../lib/api';
import HomeView from './HomeView';
import MessageList from './MessageList';
import ChatInput from './ChatInput';
import ChatHeader from './ChatHeader';
import { IconSettings } from '../icons';
import { navigateTo } from '../layout/Sidebar';

// 底部调试状态栏
function DebugBar() {
  const { error, isStreaming } = useChatUIStore();
  const wsState = useWSStore((s) => s.state);
  return (
    <div className="debug-bar">
      <span>WS: <b style={{color: wsState==='connected'?'#4ade80':'#f87171'}}>{wsState}</b></span>
      <span>streaming: <b>{String(isStreaming)}</b></span>
      <span>error: <b style={{color: error ? '#f87171' : '#888'}}>{error ? JSON.stringify(error) : '(null)'}</b></span>
    </div>
  );
}

interface TokenStat {
  model_name: string;
  total_input: number;
  total_output: number;
  total_tokens: number;
}

interface ChatViewProps {
  onPreviewOpen?: (url: string, title: string) => void;
  onFilePreview?: (workspaceId: string, filePath: string, fileName: string) => void;
}

export default function ChatView({ onPreviewOpen, onFilePreview }: ChatViewProps) {
  const activeSessionId = useSessionStore((s) => s.activeSessionId);
  const activeWorkspaceId = useWorkspaceStore((s) => s.activeWorkspaceId);
  const {
    messages,
    isLoading,
    runId,
    setMessages,
    addMessage,
    updateLastAssistant,
    appendBlock,
    appendThinking,
    setLoading,
    setRunId,
  } = useMessageStore();
  const { setIsStreaming, setError, error: streamError } = useChatUIStore();
  const cancelRef = useRef<(() => void) | null>(null);
  const streamAccRef = useRef('');
  const sessionTokenRef = useRef(0);
  // 标记刚完成流式输出（成功或失败），防止 useEffect 重取消息覆盖本地内容
  const streamFinishedRef = useRef(false);
  // 延迟展示的文件产物卡片 — 收集后，等文字输出完毕再统一追加
  const deferredFileBlocksRef = useRef<ContentBlock[]>([]);

  const [tokenStats, setTokenStats] = useState<TokenStat[]>([]);
  const [sessionTokens, setSessionTokens] = useState(0);
  const [msgLoadError, setMsgLoadError] = useState<string | null>(null);
  const [models, setModels] = useState<ModelConfig[]>([]);
  const [selectedModel, setSelectedModel] = useState<string>('');
  const enabledModels = models.filter((m) => m.enabled);

  // 加载模型列表
  const loadModels = () => {
    fetchModels()
      .then((data) => {
        const list = Array.isArray(data) ? data : [];
        setModels(list);
        // 优先选用标记为默认的已启用模型，其次选第一个已启用模型
        const defaultModel = list.find((m) => m.enabled && m.is_default);
        const firstEnabled = list.find((m) => m.enabled);
        if (!selectedModel) {
          setSelectedModel(defaultModel?.name || firstEnabled?.name || '');
        }
      })
      .catch(() => {});
  };

  // 初始加载模型
  useEffect(() => {
    loadModels();
  }, []);

  // 加载 Token 统计
  useEffect(() => {
    fetchTokenStats().then(setTokenStats).catch(console.error);
  }, [activeWorkspaceId, messages.length]);

  // 切换会话时加载消息
  useEffect(() => {
    if (activeSessionId) {
      // 正在流式生成中时跳过 — 消息由 onToken 回调本地构建
      if (isLoading) return;

      // 流式刚结束（done/error），消息已在本地构建完成，跳过服务端重取
      if (streamFinishedRef.current) {
        streamFinishedRef.current = false;
        return;
      }

      setMsgLoadError(null);
      fetchMessages(activeSessionId)
        .then((msgs) => {
          // 将历史消息中的 blocks JSON 解析为 ContentBlock[]，并将 tool 消息的 blocks
          // 合并到前一条 assistant 消息中（与实时流式展示的单一 assistant + blocks 结构一致）。
          const merged: Array<{
            id: string;
            role: 'user' | 'assistant' | 'system';
            content: string;
            blocks?: Array<any>;
            created_at?: string;
          }> = [];
          for (const m of msgs) {
            let parsedBlocks: any[] = [];
            if (m.blocks) {
              try {
                parsedBlocks = JSON.parse(m.blocks);
              } catch {
                parsedBlocks = [];
              }
            }

            if (m.role === 'user') {
              merged.push({ id: m.id, role: 'user', content: m.content, created_at: m.created_at });
            } else if (m.role === 'assistant') {
              merged.push({ id: m.id, role: 'assistant', content: m.content, blocks: [], created_at: m.created_at });
            } else if (m.role === 'tool' && parsedBlocks.length > 0) {
              // tool 消息的 blocks 合并到前一条 assistant 消息
              // 向上查找最近的 assistant 消息
              for (let j = merged.length - 1; j >= 0; j--) {
                if (merged[j].role === 'assistant') {
                  merged[j].blocks = [...(merged[j].blocks || []), ...parsedBlocks];
                  break;
                }
              }
            }
          }
          setMessages(merged);
          const t = msgs.reduce((sum, m) => sum + (m.token_count || 0), 0);
          sessionTokenRef.current = t;
          setSessionTokens(t);
        })
        .catch((err) => {
          setMsgLoadError(err.message || '加载消息失败');
          console.error(err);
        });
    } else {
      // 新建对话时清空消息，但流式生成中（isLoading=true）不清理 —
      // handleSend 中 setLoading(true) 触发此 effect 时 activeSessionId 可能仍为 null
      // （onMeta 还没回来），此时 messages 是刚加入的用户+助手消息，不应被清空
      if (!isLoading) {
        setMessages([]);
        setMsgLoadError(null);
        sessionTokenRef.current = 0;
        setSessionTokens(0);
      }
    }
  }, [activeSessionId, isLoading]);

  // 取消
  const handleCancel = async () => {
    if (cancelRef.current) {
      cancelRef.current();
      cancelRef.current = null;
    }
    if (runId) {
      try {
        await cancelChat(runId);
      } catch {}
    }
    setIsStreaming(false);
    setLoading(false);
    setRunId(null);
  };

  // 插队：取消当前任务，清空输入准备新消息
  const handleSkipQueue = async () => {
    await handleCancel();
  };

  // 发送消息
  const handleSend = async (prompt: string) => {
    console.log('[ChatView] handleSend START prompt="%s" isLoading=%s activeSessionId=%s activeWorkspaceId=%s', prompt, isLoading, activeSessionId, activeWorkspaceId);
    if (!prompt.trim() || isLoading) return;

    // 检查是否有可用的模型
    const modelName = selectedModel || enabledModels[0]?.name;
    if (!modelName) {
      setError('请先在「模型配置」中添加并启用至少一个模型');
      return;
    }

    setLoading(true);
    setIsStreaming(true);
    setError(null);
    streamAccRef.current = '';
    deferredFileBlocksRef.current = [];

    const userMsg = {
      id: `user-${Date.now()}`,
      role: 'user' as const,
      content: prompt,
    };
    addMessage(userMsg);

    addMessage({
      id: `assistant-${Date.now()}`,
      role: 'assistant',
      content: '',
    });

    const cancel = streamChat(
      prompt,
      activeSessionId || '',
      activeWorkspaceId,
      modelName,
      {
        onToken: (token) => {
          streamAccRef.current += token as string;
          updateLastAssistant(streamAccRef.current);
        },
        onMeta: (meta) => {
          setRunId(meta.run_id);
          if (!activeSessionId) {
            useSessionStore.getState().addSession({
              id: meta.session_id,
              title: prompt.slice(0, 30),
              workspace_id: activeWorkspaceId,
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            });
          }
        },
        onDone: (data) => {
          // 文字输出完成 → 追加所有延迟的文件产物卡片
          for (const block of deferredFileBlocksRef.current) {
            appendBlock(block);
          }
          deferredFileBlocksRef.current = [];
          streamFinishedRef.current = true;
          setLoading(false);
          setIsStreaming(false);
          setRunId(null);
          cancelRef.current = null;
          if (data.token_count) {
            sessionTokenRef.current += data.token_count;
            setSessionTokens(sessionTokenRef.current);
          }
          fetchTokenStats().then(setTokenStats).catch(console.error);
        },
        onError: (err) => {
          console.log('[ChatView] handleSend onError:', err);
          // 在 assistant 消息末尾附加友好的错误提示，保留已生成内容
          updateLastAssistant(
            streamAccRef.current +
              '\n\n---\n⚠️ **生成中断**：回答过程中遇到了问题，请稍后重试。\n' +
              (err ? `\n> 错误详情：${err}` : '')
          );
          streamFinishedRef.current = true;
          setError('生成中断，请稍后重试');
          setLoading(false);
          setIsStreaming(false);
          setRunId(null);
          cancelRef.current = null;
        },
        onEvent: (type, data) => {
          if (type === 'thinking') {
            appendThinking(data as string);
          } else if (type === 'tool_call') {
            const d = data as { call_id: string; name: string; arguments: string };
            appendBlock({
              type: 'tool_call',
              call_id: d.call_id,
              name: d.name,
              arguments: d.arguments,
            });
          } else if (type === 'tool_result') {
            const d = data as {
              call_id: string;
              name: string;
              result: string;
              error?: string;
              file_info?: { file_path: string; file_name: string; byte_count: number; workspace_id: string };
              file_infos?: Array<{ file_path: string; file_name: string; byte_count: number; workspace_id: string }>;
            };
            // 追加 tool_result block
            appendBlock({
              type: 'tool_result',
              call_id: d.call_id,
              name: d.name,
              result: d.result,
              error: d.error,
            });
            // 收集文件产物到延迟队列，等文字输出完成后统一展示
            if (d.file_infos) {
              for (const fi of d.file_infos) {
                deferredFileBlocksRef.current.push({
                  type: 'file_artifact',
                  file_path: fi.file_path,
                  file_name: fi.file_name,
                  byte_count: fi.byte_count,
                  workspace_id: fi.workspace_id,
                  call_id: d.call_id,
                });
              }
            } else if (d.file_info) {
              deferredFileBlocksRef.current.push({
                type: 'file_artifact',
                file_path: d.file_info.file_path,
                file_name: d.file_info.file_name,
                byte_count: d.file_info.byte_count,
                workspace_id: d.file_info.workspace_id,
                call_id: d.call_id,
              });
            }
          } else if (type === 'confirm_request') {
            const d = data as { call_id: string; question: string; options?: string[]; context?: string };
            // 直接写入消息 blocks，由 ConfirmCard 内嵌渲染（交互模式：待确认）
            appendBlock({
              type: 'confirm_request',
              call_id: d.call_id,
              question: d.question,
              options: d.options,
              context: d.context,
            });
          }
        },
      }
    );

    cancelRef.current = cancel;
  };

  const showHome = !activeSessionId && messages.length === 0;
  const noModels = !activeSessionId && enabledModels.length === 0;

  return (
    <>
    <div className="h-full flex">
      {/* 首页：无会话时显示 */}
      {showHome ? (
        <div className="flex-1 flex flex-col min-h-0" style={{ paddingBottom: 36 }}>
          {/* 无模型配置提示横幅 */}
          {noModels && (
            <div className="flex-shrink-0"
              style={{
                background: 'var(--color-warning-muted)',
                borderBottom: '1px solid rgba(245, 158, 11, 0.2)',
              }}
            >
              <div className="max-w-3xl mx-auto px-6 py-2.5 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="text-sm">⚠️</span>
                  <span className="text-[13px]" style={{ color: 'var(--color-warning)' }}>
                    尚未配置模型，请先前往「模型配置」添加 AI 模型后再开始对话
                  </span>
                </div>
                <button
                  onClick={() => navigateTo('settings')}
                  className="flex items-center gap-1.5 text-[13px] font-medium px-3 py-1.5 rounded-lg transition-all hover:brightness-110"
                  style={{
                    background: 'var(--color-accent)',
                    color: '#fff',
                  }}
                >
                  <IconSettings size={14} />
                  去配置
                </button>
              </div>
            </div>
          )}
          <HomeView onSend={handleSend} onCancel={handleCancel} disabled={isLoading || noModels} isStreaming={isLoading} models={enabledModels} selectedModel={selectedModel} onModelChange={setSelectedModel} />
        </div>
      ) : (
        <>
          {/* 聊天主区域 */}
          <div className="flex-1 flex flex-col min-w-0" style={{ paddingBottom: 36 }}>
            {/* 无模型配置提示横幅（有会话时也显示） */}
            {enabledModels.length === 0 && (
              <div className="flex-shrink-0"
                style={{
                  background: 'var(--color-warning-muted)',
                  borderBottom: '1px solid rgba(245, 158, 11, 0.2)',
                }}
              >
                <div className="px-5 py-2.5 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-sm">⚠️</span>
                    <span className="text-[13px]" style={{ color: 'var(--color-warning)' }}>
                      尚未配置模型，请先前往「模型配置」添加 AI 模型后再开始对话
                    </span>
                  </div>
                  <button
                    onClick={() => navigateTo('settings')}
                    className="flex items-center gap-1.5 text-[13px] font-medium px-3 py-1.5 rounded-lg transition-all hover:brightness-110"
                    style={{
                      background: 'var(--color-accent)',
                      color: '#fff',
                    }}
                  >
                    <IconSettings size={14} />
                    去配置
                  </button>
                </div>
              </div>
            )}

            {/* 顶栏：会话标题 + 停止/插队按钮 */}
            {activeSessionId && (
              <ChatHeader onStop={handleCancel} onSkipQueue={handleSkipQueue} />
            )}

            {/* 错误横幅 */}
            {streamError && (
              <div
                className="flex items-center justify-between px-4 py-2.5 text-sm"
                style={{
                  background: 'var(--color-error-muted)',
                  color: 'var(--color-error)',
                  borderBottom: '1px solid rgba(239, 68, 68, 0.15)',
                }}
              >
                <span className="flex items-center gap-2">
                  <span className="text-xs">⚠</span>
                  {streamError}
                </span>
                <button
                  onClick={() => setError(null)}
                  className="text-xs px-2 py-0.5 rounded-lg hover:bg-black/5 transition-all"
                  style={{ color: 'var(--color-error)' }}
                >
                  ✕
                </button>
              </div>
            )}

            {/* 消息加载错误 */}
            {msgLoadError && (
              <div
                className="flex items-center justify-between px-4 py-2.5 text-sm"
                style={{
                  background: 'var(--color-error-muted)',
                  color: 'var(--color-error)',
                  borderBottom: '1px solid rgba(239, 68, 68, 0.15)',
                }}
              >
                <span className="flex items-center gap-2">
                  <span className="text-xs">⚠</span>
                  {msgLoadError}
                </span>
                <button
                  onClick={() => setMsgLoadError(null)}
                  className="text-xs px-2 py-0.5 rounded-lg hover:bg-black/5 transition-all"
                  style={{ color: 'var(--color-error)' }}
                >
                  ✕
                </button>
              </div>
            )}

            <MessageList
              messages={messages}
              isLoading={isLoading}
              onPreviewOpen={onPreviewOpen}
              onFilePreview={onFilePreview}
            />
            <ChatInput
              onSend={handleSend}
              onCancel={handleCancel}
              disabled={isLoading || enabledModels.length === 0}
              isStreaming={isLoading}
              sessionTokens={sessionTokens}
              stats={tokenStats}
              models={enabledModels}
              selectedModel={selectedModel}
              onModelChange={setSelectedModel}
            />
          </div>
        </>
      )}
    </div>
    <DebugBar />
    </>
  );
}