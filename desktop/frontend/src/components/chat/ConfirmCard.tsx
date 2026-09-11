import { useState, useEffect, useCallback } from 'react';
import { sendConfirmation } from '../../lib/api';
import { useMessageStore } from '../../lib/store';

interface ConfirmCardProps {
  callId: string;
  question: string;
  options?: string[];
  context?: string;
  /** 存在 = 已确认（历史模式），不存在 = 待确认（交互模式） */
  response?: string;
}

/** 提取命令文本（去掉可能的 markdown 代码块包裹） */
function extractCommand(context: string): string {
  const trimmed = context.trim();
  const codeBlockMatch = trimmed.match(/```[\w]*\n?([\s\S]*?)```/);
  if (codeBlockMatch) return codeBlockMatch[1].trim();
  return trimmed;
}

/** 判断是否为 shell 命令 */
function isShellCommand(text: string): boolean {
  const cmd = extractCommand(text);
  return /^\$?\s*(curl|wget|npm|yarn|pnpm|pip|python|node|go|bash|sh|git|docker|kubectl|helm|cat|ls|rm|mv|cp|mkdir|chmod|chown|export|source|echo|printf|find|grep|sed|awk|make|cargo|rustc|java|javac|mvn|gradle|brew|apt|yum|dnf|ssh|scp|rsync)\b/.test(
    cmd,
  );
}

/**
 * 确认卡片 — 消息流内嵌组件，统一处理命令权限审批和文件覆盖确认。
 *
 * 两种模式：
 * - 交互模式（response 为 undefined）：待确认状态，用户可通过键盘/鼠标选择并提交
 * - 历史模式（response 有值）：已确认状态，只读回放
 */
export default function ConfirmCard({
  callId,
  question,
  options,
  context,
  response: initialResponse,
}: ConfirmCardProps) {
  // ---- 状态 ----
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // 本地 resolved 状态：提交成功后本地置位，无需等 store 更新回流
  const [resolvedResponse, setResolvedResponse] = useState<string | undefined>(initialResponse);

  const isResolved = !!resolvedResponse;

  // 选项列表
  const items: string[] = options && options.length > 0 ? options : [];

  // 场景检测
  const showAsShell = context ? isShellCommand(context) : false;
  const cmd = context ? extractCommand(context) : '';
  const isFileScenario = !!context && !showAsShell;

  // ---- 提交处理 ----
  const handleConfirm = useCallback(async () => {
    if (sending || isResolved) return;
    if (items.length === 0) return;

    setSending(true);
    setError(null);

    const chosenOption = items[selectedIndex];

    try {
      await sendConfirmation(callId, chosenOption);
      // 更新 store 中的 block（标记为已确认，持久化）
      useMessageStore.getState().resolveConfirmBlock(callId, chosenOption);
      // 本地立即置为已确认
      setResolvedResponse(chosenOption);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '提交失败';
      setError(msg);
      setSending(false);
    }
  }, [sending, isResolved, items, selectedIndex, callId]);

  // ---- 键盘导航 ----
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (sending || isResolved || items.length === 0) return;

      switch (e.key) {
        case 'ArrowUp':
        case 'ArrowLeft':
          e.preventDefault();
          setSelectedIndex((prev) => (prev > 0 ? prev - 1 : items.length - 1));
          break;
        case 'ArrowDown':
        case 'ArrowRight':
          e.preventDefault();
          setSelectedIndex((prev) => (prev < items.length - 1 ? prev + 1 : 0));
          break;
        case 'Tab':
          e.preventDefault();
          if (e.shiftKey) {
            setSelectedIndex((prev) => (prev > 0 ? prev - 1 : items.length - 1));
          } else {
            setSelectedIndex((prev) => (prev < items.length - 1 ? prev + 1 : 0));
          }
          break;
        case 'Enter':
          e.preventDefault();
          handleConfirm();
          break;
        case 'Escape':
          // 不允许通过 Escape 关闭
          e.preventDefault();
          break;
      }
    },
    [sending, isResolved, items.length, handleConfirm],
  );

  useEffect(() => {
    if (isResolved) return;
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown, isResolved]);

  // Sync initialResponse changes (in case store update triggers re-render)
  useEffect(() => {
    if (initialResponse) {
      setResolvedResponse(initialResponse);
    }
  }, [initialResponse]);

  // ===== 渲染 =====
  return (
    <div
      style={{
        marginTop: 8,
        marginBottom: 8,
        border: '1px solid var(--color-border-light)',
        borderRadius: 'var(--radius-md)',
        background: 'var(--color-bg-secondary)',
        overflow: 'hidden',
        opacity: isResolved ? 0.85 : 1,
        transition: 'opacity 0.3s ease',
      }}
    >
      {/* ===== 头部 ===== */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          padding: '10px 14px',
          borderBottom: '1px solid var(--color-border-light)',
        }}
      >
        <span style={{ fontSize: 15, flexShrink: 0 }}>
          {isResolved ? '✅' : '⏳'}
        </span>
        <span
          style={{
            fontWeight: 600,
            fontSize: 13,
            color: 'var(--color-text-secondary)',
          }}
        >
          {isResolved ? '已确认' : '等待确认'}
        </span>
        {isResolved && resolvedResponse && (
          <span
            style={{
              fontSize: 11,
              color: 'var(--color-success)',
              marginLeft: 'auto',
              fontWeight: 500,
            }}
          >
            ✓ {resolvedResponse}
          </span>
        )}
      </div>

      {/* ===== 信息展示区 ===== */}
      <div style={{ padding: '12px 14px' }}>
        {/* 问题描述 */}
        <p
          style={{
            fontSize: 14,
            fontWeight: 500,
            color: 'var(--color-text-primary)',
            lineHeight: 1.6,
            margin: '0 0 12px 0',
          }}
        >
          {question}
        </p>

        {/* 场景 1: 命令代码块 */}
        {showAsShell && cmd && (
          <div style={{ marginBottom: 12 }}>
            <div
              style={{
                borderRadius: 8,
                overflow: 'hidden',
                background: 'var(--color-bg-primary)',
                border: '1px solid var(--color-border-light)',
              }}
            >
              {/* 代码块头部 */}
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '6px 12px',
                  borderBottom: '1px solid var(--color-border-light)',
                }}
              >
                <span
                  style={{
                    fontSize: 11,
                    fontWeight: 500,
                    color: 'var(--color-text-muted)',
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                  }}
                >
                  Shell 命令
                </span>
                <span style={{ fontSize: 11, color: 'var(--color-text-muted)', opacity: 0.6 }}>
                  bash
                </span>
              </div>
              {/* 代码内容 */}
              <pre
                style={{
                  margin: 0,
                  padding: '12px 14px',
                  fontFamily: '"SF Mono", "Fira Code", "Fira Mono", Menlo, Consolas, monospace',
                  fontSize: 13,
                  lineHeight: 1.6,
                  color: 'var(--color-text-primary)',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                  overflowX: 'auto',
                }}
              >
                {cmd.startsWith('$ ') ? cmd : `$ ${cmd}`}
              </pre>
            </div>
            {/* 预览输出占位 */}
            <div
              style={{
                marginTop: 6,
                padding: '8px 14px',
                borderRadius: 6,
                background: 'color-mix(in srgb, var(--color-bg-primary) 60%, transparent)',
                fontSize: 12,
                color: 'var(--color-text-muted)',
                fontStyle: 'italic',
              }}
            >
              没有输出。
            </div>
          </div>
        )}

        {/* 场景 2: 文件操作信息 */}
        {isFileScenario && context && (
          <div
            style={{
              marginBottom: 12,
              padding: '10px 14px',
              borderRadius: 8,
              background: 'var(--color-bg-primary)',
              border: '1px solid var(--color-border-light)',
              fontSize: 13,
              lineHeight: 1.7,
              color: 'var(--color-text-secondary)',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}
          >
            {context}
          </div>
        )}

        {/* ===== 选项列表 — 整行可点击，无 radio 圆点 ===== */}
        {items.length > 0 && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2, marginBottom: 4 }}>
            {items.map((item, idx) => {
              const isSelected = idx === selectedIndex && !isResolved;
              const disabled = isResolved || sending;
              return (
                <button
                  key={item}
                  type="button"
                  disabled={disabled}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 10,
                    width: '100%',
                    textAlign: 'left',
                    padding: '10px 14px',
                    borderRadius: 8,
                    border: isSelected
                      ? '1px solid var(--color-accent)'
                      : '1px solid transparent',
                    background: isSelected
                      ? 'var(--color-accent-muted)'
                      : 'transparent',
                    cursor: disabled ? 'default' : 'pointer',
                    opacity: isResolved ? 0.5 : 1,
                    transition: 'all var(--transition-fast)',
                    fontFamily: 'inherit',
                    fontSize: 14,
                    color: isSelected
                      ? 'var(--color-text-primary)'
                      : 'var(--color-text-secondary)',
                  }}
                  onClick={() => {
                    if (!disabled) setSelectedIndex(idx);
                  }}
                  onMouseEnter={(e) => {
                    if (!disabled && !isSelected) {
                      e.currentTarget.style.background = 'var(--color-bg-hover)';
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (!disabled && !isSelected) {
                      e.currentTarget.style.background = 'transparent';
                    }
                  }}
                >
                  {/* 序号 */}
                  <span
                    style={{
                      flexShrink: 0,
                      width: 22,
                      height: 22,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      borderRadius: 6,
                      fontSize: 12,
                      fontWeight: 600,
                      fontFamily: '"SF Mono", "Fira Code", Menlo, Consolas, monospace',
                      color: isSelected ? 'var(--color-accent)' : 'var(--color-text-muted)',
                      background: isSelected ? 'color-mix(in srgb, var(--color-accent) 20%, transparent)' : 'transparent',
                    }}
                  >
                    {idx + 1}
                  </span>
                  {/* 选项文字 */}
                  <span style={{ fontWeight: isSelected ? 500 : 400 }}>{item}</span>
                </button>
              );
            })}
          </div>
        )}

        {/* 错误提示 */}
        {error && (
          <div
            style={{
              fontSize: 13,
              padding: '8px 12px',
              borderRadius: 8,
              marginTop: 8,
              background: 'var(--color-error-muted)',
              color: 'var(--color-error)',
            }}
          >
            {error}
          </div>
        )}
      </div>

      {/* ===== 底部操作栏（仅交互模式） ===== */}
      {!isResolved && items.length > 0 && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '10px 14px',
            borderTop: '1px solid var(--color-border-light)',
          }}
        >
          <span
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 5,
              fontSize: 11,
              color: 'var(--color-text-muted)',
            }}
          >
            <span style={{ fontSize: 12, opacity: 0.7 }}>💡</span>
            使用 Tab / 上下键选择，回车确认
          </span>
          <button
            type="button"
            disabled={sending}
            style={{
              padding: '7px 20px',
              borderRadius: 10,
              fontSize: 14,
              fontWeight: 500,
              border: 'none',
              background: sending
                ? 'rgba(255, 255, 255, 0.08)'
                : 'linear-gradient(135deg, var(--color-accent-hover), var(--color-accent))',
              color: sending ? 'rgba(255, 255, 255, 0.3)' : '#fff',
              cursor: sending ? 'not-allowed' : 'pointer',
              transition: 'all var(--transition-fast)',
            }}
            onClick={handleConfirm}
            onMouseEnter={(e) => {
              if (!sending) {
                e.currentTarget.style.filter = 'brightness(1.1)';
                e.currentTarget.style.transform = 'translateY(-1px)';
              }
            }}
            onMouseLeave={(e) => {
              if (!sending) {
                e.currentTarget.style.filter = 'brightness(1)';
                e.currentTarget.style.transform = 'translateY(0)';
              }
            }}
          >
            {sending ? '提交中...' : '确认'}
          </button>
        </div>
      )}
    </div>
  );
}