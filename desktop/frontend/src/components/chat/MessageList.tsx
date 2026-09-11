import { useRef, useEffect } from 'react';
import MessageItem from './MessageItem';

interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
}

interface Props {
  messages: Message[];
  isLoading: boolean;
  onPreviewOpen?: (url: string, title: string) => void;
  onFilePreview?: (workspaceId: string, filePath: string, fileName: string) => void;
}

const SUGGESTIONS = [
  { icon: '📝', label: '帮我写一份项目计划书' },
  { icon: '🔍', label: '解释这个代码片段的作用' },
  { icon: '💡', label: '推荐适合初学者的编程语言' },
  { icon: '📊', label: '分析这份数据的趋势和模式' },
  { icon: '🌐', label: '解释 RESTful API 设计原则' },
  { icon: '🐛', label: '帮我调试一个 JavaScript 错误' },
];

export default function MessageList({ messages, isLoading, onPreviewOpen, onFilePreview }: Props) {
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // ===== 空态 =====
  if (messages.length === 0) {
    return (
      <div className="msg-list">
        <div className="msg-list-empty">
          <h1>Hi, 我是小智</h1>
          <p>你的 AI 智能伙伴，随时为你提供帮助</p>
          <div className="flex flex-wrap justify-center gap-2" style={{ maxWidth: 520 }}>
            {SUGGESTIONS.map((item) => (
              <button
                key={item.label}
                className="home-suggestion-chip"
                onClick={() => {
                  window.dispatchEvent(
                    new CustomEvent('suggestion-click', { detail: item.label })
                  );
                }}
              >
                {item.icon} {item.label}
              </button>
            ))}
          </div>
        </div>
      </div>
    );
  }

  // ===== 消息列表 =====
  /** 连续同角色用户消息合并为一组 */
  const groups = (() => {
    const result: Array<Message | Message[]> = [];
    let i = 0;
    while (i < messages.length) {
      if (messages[i].role === 'user') {
        const group: Message[] = [messages[i]];
        let j = i + 1;
        while (j < messages.length && messages[j].role === 'user') {
          group.push(messages[j]);
          j++;
        }
        result.push(group.length > 1 ? group : messages[i]);
        i = j;
      } else {
        result.push(messages[i]);
        i++;
      }
    }
    return result;
  })();

  return (
    <div className="msg-list">
      <div className="msg-list__inner">
        {groups.map((item) => {
          if (Array.isArray(item)) {
            // 连续用户消息合并为一个气泡组
            return (
              <div className="user-msg-group" key={item.map((m) => m.id).join('-')}>
                {item.map((msg) => (
                  <MessageItem
                    key={msg.id}
                    message={msg}
                    isStreaming={false}
                    onPreviewOpen={onPreviewOpen}
                    onFilePreview={onFilePreview}
                  />
                ))}
              </div>
            );
          }
          // 单条消息（用户或 Agent）
          return (
            <MessageItem
              key={item.id}
              message={item}
              isStreaming={
                isLoading &&
                item.role === 'assistant' &&
                messages[messages.length - 1].id === item.id
              }
              onPreviewOpen={onPreviewOpen}
              onFilePreview={onFilePreview}
            />
          );
        })}

        <div ref={bottomRef} />
      </div>
    </div>
  );
}