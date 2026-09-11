import { useState } from 'react';

interface Props {
  url: string;
  title: string;
  activeTab: 'web' | 'code';
  onTabChange: (tab: 'web' | 'code') => void;
  onClose: () => void;
}

export default function PreviewPanel({ url, title, activeTab, onTabChange, onClose }: Props) {
  const [codeContent, setCodeContent] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // 切换到 Code 标签时获取页面源码
  const handleTabChange = (tab: 'web' | 'code') => {
    onTabChange(tab);
    if (tab === 'code' && codeContent === null) {
      setLoading(true);
      fetch(url)
        .then((r) => r.text())
        .then((text) => setCodeContent(text))
        .catch(() => setCodeContent('// 无法获取源码'))
        .finally(() => setLoading(false));
    }
  };

  return (
    <div
      className="h-full flex flex-col border-l"
      style={{
        width: '42%',
        minWidth: 400,
        background: '#fff',
        borderColor: 'var(--color-border)',
      }}
    >
      {/* 头部：标题 + 关闭 */}
      <div
        className="flex items-center justify-between px-4 py-2.5 border-b"
        style={{ borderColor: 'var(--color-border)' }}
      >
        <div className="flex items-center gap-3 min-w-0">
          <span className="text-sm font-medium truncate" style={{ color: 'var(--color-text-primary)' }}>
            {title}
          </span>
        </div>
        <button
          onClick={onClose}
          className="w-6 h-6 rounded-md flex items-center justify-center text-sm transition-all hover:bg-black/5"
          style={{ color: 'var(--color-text-muted)' }}
          title="关闭预览"
        >
          ✕
        </button>
      </div>

      {/* 标签栏：Web / Code */}
      <div
        className="flex items-center px-4 border-b"
        style={{ borderColor: 'var(--color-border)' }}
      >
        {(['web', 'code'] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => handleTabChange(tab)}
            className="relative px-4 py-2.5 text-[13px] font-medium transition-all"
            style={{
              color: activeTab === tab ? 'var(--color-text-primary)' : 'var(--color-text-muted)',
            }}
          >
            {tab === 'web' ? '🌐 Web' : '💻 Code'}
            {activeTab === tab && (
              <span
                className="absolute bottom-0 left-1/2 -translate-x-1/2 w-8 h-0.5 rounded-full"
                style={{ background: 'var(--color-accent)' }}
              />
            )}
          </button>
        ))}
      </div>

      {/* 内容区 */}
      <div className="flex-1 flex flex-col min-h-0">
        {activeTab === 'web' ? (
          <>
            {/* 地址栏 */}
            <div
              className="flex items-center gap-2 px-3 py-2 border-b"
              style={{ borderColor: 'var(--color-border)', background: 'var(--color-bg-tertiary)' }}
            >
              <span className="text-[11px]" style={{ color: 'var(--color-text-muted)' }}>🔗</span>
              <input
                type="text"
                value={url}
                readOnly
                className="flex-1 bg-transparent border-none outline-none text-[12px] truncate"
                style={{ color: 'var(--color-text-secondary)' }}
              />
            </div>
            {/* Web 预览 */}
            <iframe
              src={url}
              className="flex-1 w-full border-none"
              title="预览"
              sandbox="allow-scripts allow-same-origin"
            />
          </>
        ) : (
          <>
            {/* Code 视图 */}
            <div className="flex-1 overflow-auto p-4">
              {loading ? (
                <p className="text-[13px] text-center py-12" style={{ color: 'var(--color-text-muted)' }}>
                  加载源码中...
                </p>
              ) : codeContent ? (
                <pre
                  className="text-[12px] leading-relaxed font-mono whitespace-pre-wrap break-all"
                  style={{ color: 'var(--color-text-primary)' }}
                >
                  {codeContent}
                </pre>
              ) : (
                <p className="text-[13px] text-center py-12" style={{ color: 'var(--color-text-muted)' }}>
                  点击 Code 标签查看页面源码
                </p>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}