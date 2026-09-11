import { useState, useEffect, useRef, type FormEvent } from 'react';
import { useAuthStore } from '../../lib/store';
import { login, register } from '../../lib/api';
import { wsClient } from '../../lib/ws';

type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting';

/** 从 unknown 错误中提取可读消息，不使用 any */
function getErrorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === 'object' && err !== null && 'message' in err) {
    return String((err as { message: unknown }).message);
  }
  return '操作失败';
}

export default function LoginView() {
  const authLogin = useAuthStore((s) => s.login);
  const [isRegister, setIsRegister] = useState(false);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [wsStatus, setWsStatus] = useState<ConnectionState>(wsClient.getState());
  const usernameRef = useRef<HTMLInputElement>(null);

  // 跟踪 WS 连接状态
  useEffect(() => {
    const unsub = wsClient.onStateChange((s: ConnectionState) => {
      setWsStatus(s);
      if (s === 'connected') setError('');
    });
    return unsub;
  }, []);

  useEffect(() => {
    usernameRef.current?.focus();
  }, []);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');

    // 连接状态检查
    const state = wsClient.getState();
    if (state !== 'connected') {
      setError(`WebSocket 未连接 (${state})，请稍等...`);
      return;
    }

    setLoading(true);

    try {
      if (isRegister) {
        // 注册模式：直接注册
        const res = await register(username, password);
        authLogin(res.user_id, res.username);
      } else {
        // 登录模式：先尝试登录，失败则自动注册
        try {
          const res = await login(username, password);
          authLogin(res.user_id, res.username);
        } catch (loginErr: unknown) {
          const msg = getErrorMessage(loginErr);
          if (msg === 'invalid credentials') {
            // 账号不存在，自动注册
            try {
              setError('账号不存在，正在自动注册...');
              const res = await register(username, password);
              authLogin(res.user_id, res.username);
            } catch (regErr: unknown) {
              const regMsg = getErrorMessage(regErr);
              setError(
                regMsg === 'username already exists'
                  ? '密码错误，请重试或点击"注册"创建新账号'
                  : regMsg,
              );
            }
          } else {
            setError(msg);
          }
        }
      }
    } catch (err: unknown) {
      setError(getErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  const wsLabel: Record<ConnectionState, string> = {
    connected: '已连接',
    connecting: '连接中...',
    reconnecting: '重连中...',
    disconnected: '未连接',
  };

  return (
    <div
      className="h-full flex items-center justify-center relative overflow-hidden select-none"
      style={{ background: 'var(--color-bg-primary)' }}
    >
      {/* ===== 科技感背景纹理：低透明度点阵 + 顶部渐变光晕 ===== */}
      <div
        className="absolute inset-0 pointer-events-none"
        style={{
          backgroundImage: `
            radial-gradient(circle at 50% 0%, rgba(99, 102, 241, 0.025) 0%, transparent 55%),
            radial-gradient(circle, rgba(0, 0, 0, 0.025) 0.5px, transparent 0.5px)
          `,
          backgroundSize: '100% 100%, 22px 22px',
          backgroundPosition: '0 0, 0 0',
        }}
      />

      {/* ===== 主内容 ===== */}
      <div className="w-full max-w-sm relative z-10 px-4">
        {/* --- Logo + 标题 --- */}
        <div className="text-center mb-8">
          <div
            className="w-[72px] h-[72px] rounded-2xl flex items-center justify-center mx-auto mb-5"
            style={{
              background: 'linear-gradient(135deg, #7c8aff, #a78bfa)',
              boxShadow:
                '0 8px 32px rgba(124, 138, 255, 0.18), 0 0 0 1px rgba(124, 138, 255, 0.06)',
              animation: 'login-logo-float 3s ease-in-out infinite',
            }}
          >
            <span className="text-[32px] leading-none select-none">🤖</span>
          </div>
          <h1
            className="text-[22px] font-bold mb-1 tracking-tight select-none"
            style={{ color: 'var(--color-text-primary)' }}
          >
            Smart Assistant
          </h1>
          <p
            className="text-[14px] select-none"
            style={{ color: 'var(--color-text-muted)' }}
          >
            智能助手客户端
          </p>
        </div>

        {/* --- 登录卡片 --- */}
        <form
          onSubmit={handleSubmit}
          className="p-6 space-y-4"
          style={{
            background: 'var(--color-surface)',
            borderRadius: 'var(--radius-xl)',
            border: '1px solid var(--color-border-light)',
            boxShadow: `
              0 8px 32px rgba(0, 0, 0, 0.05),
              0 0 0 1px rgba(0, 0, 0, 0.03),
              0 0 80px rgba(99, 102, 241, 0.04)
            `,
          }}
        >
          {/* 用户名 */}
          <div>
            <label
              className="block text-xs font-medium mb-1.5"
              style={{ color: 'var(--color-text-secondary)' }}
            >
              用户名
            </label>
            <input
              ref={usernameRef}
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full px-3.5 py-2.5 text-[15px] outline-none placeholder:text-[var(--color-text-muted)]"
              style={{
                background: 'var(--color-bg-secondary)',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius-md)',
                color: 'var(--color-text-primary)',
                transition: 'border-color var(--transition-base), box-shadow var(--transition-base)',
              }}
              placeholder="请输入用户名"
              required
              autoComplete="username"
              onFocus={(e) => {
                e.currentTarget.style.borderColor = 'var(--color-accent)';
                e.currentTarget.style.boxShadow = '0 0 0 3px rgba(99, 102, 241, 0.08)';
              }}
              onBlur={(e) => {
                e.currentTarget.style.borderColor = 'var(--color-border)';
                e.currentTarget.style.boxShadow = 'none';
              }}
            />
          </div>

          {/* 密码 */}
          <div>
            <label
              className="block text-xs font-medium mb-1.5"
              style={{ color: 'var(--color-text-secondary)' }}
            >
              密码
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-3.5 py-2.5 text-[15px] outline-none placeholder:text-[var(--color-text-muted)]"
              style={{
                background: 'var(--color-bg-secondary)',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius-md)',
                color: 'var(--color-text-primary)',
                transition: 'border-color var(--transition-base), box-shadow var(--transition-base)',
              }}
              placeholder="请输入密码"
              required
              autoComplete="current-password"
              onFocus={(e) => {
                e.currentTarget.style.borderColor = 'var(--color-accent)';
                e.currentTarget.style.boxShadow = '0 0 0 3px rgba(99, 102, 241, 0.08)';
              }}
              onBlur={(e) => {
                e.currentTarget.style.borderColor = 'var(--color-border)';
                e.currentTarget.style.boxShadow = 'none';
              }}
            />
          </div>

          {/* 错误提示 */}
          {error && (
            <div
              className="text-sm p-2.5"
              style={{
                background: 'var(--color-error-muted)',
                color: 'var(--color-error)',
                borderRadius: 'var(--radius-sm)',
              }}
            >
              {error}
            </div>
          )}

          {/* WS 连接状态指示 */}
          <div
            className="flex items-center gap-2 text-xs select-none"
            style={{ color: 'var(--color-text-muted)' }}
          >
            <span
              className="w-[7px] h-[7px] rounded-full inline-block flex-shrink-0"
              style={{
                background:
                  wsStatus === 'connected'
                    ? 'var(--color-success)'
                    : wsStatus === 'connecting' || wsStatus === 'reconnecting'
                      ? 'var(--color-warning)'
                      : 'var(--color-error)',
                boxShadow:
                  wsStatus === 'connected'
                    ? '0 0 5px rgba(34, 197, 94, 0.35)'
                    : 'none',
              }}
            />
            {wsLabel[wsStatus]}
          </div>

          {/* 登录按钮 — 柔和渐变 */}
          <button
            type="submit"
            disabled={loading || wsStatus !== 'connected'}
            className="w-full py-2.5 text-[15px] font-medium disabled:opacity-50 disabled:cursor-not-allowed select-none"
            style={{
              background:
                loading || wsStatus !== 'connected'
                  ? 'var(--color-bg-tertiary)'
                  : 'linear-gradient(135deg, #6366f1, #818cf8)',
              color:
                loading || wsStatus !== 'connected'
                  ? 'var(--color-text-muted)'
                  : '#fff',
              borderRadius: 'var(--radius-md)',
              transition: 'filter var(--transition-base), transform var(--transition-base), box-shadow var(--transition-base)',
            }}
            onMouseEnter={(e) => {
              if (!loading && wsStatus === 'connected') {
                e.currentTarget.style.filter = 'brightness(1.07)';
                e.currentTarget.style.transform = 'translateY(-1px)';
                e.currentTarget.style.boxShadow = '0 6px 20px rgba(99, 102, 241, 0.28)';
              }
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.filter = 'brightness(1)';
              e.currentTarget.style.transform = 'translateY(0)';
              e.currentTarget.style.boxShadow = 'none';
            }}
          >
            {loading ? '处理中...' : isRegister ? '注册' : '登录'}
          </button>

          {/* 注册切换链接 — 低调，hover 变色 */}
          <button
            type="button"
            onClick={() => {
              setIsRegister(!isRegister);
              setError('');
            }}
            className="w-full text-sm text-center py-1 select-none"
            style={{
              color: 'var(--color-text-muted)',
              transition: 'color var(--transition-base)',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.color = 'var(--color-accent)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.color = 'var(--color-text-muted)';
            }}
          >
            {isRegister ? '已有账号？点此登录' : '没有账号？点此注册'}
          </button>
        </form>
      </div>

      {/* ===== Logo 浮动动画 keyframes ===== */}
      <style>{`
        @keyframes login-logo-float {
          0%, 100% { transform: translateY(0); }
          50% { transform: translateY(-6px); }
        }
      `}</style>
    </div>
  );
}