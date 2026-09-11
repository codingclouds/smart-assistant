import { useEffect, useRef, useState } from 'react';
import { renderAsync } from 'docx-preview';

interface DocxViewerProps {
  /** base64 编码的 .docx 文件内容 */
  data: string;
}

export default function DocxViewer({ data }: DocxViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;

    async function render() {
      try {
        // base64 → Uint8Array → Blob
        const binary = atob(data);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
          bytes[i] = binary.charCodeAt(i);
        }
        const blob = new Blob([bytes.buffer], {
          type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        });

        console.log('[DocxViewer] rendering docx, blob size:', blob.size, 'bytes');

        const container = containerRef.current;
        if (!container || cancelled) return;

        // 清除旧内容
        container.innerHTML = '';

        // 10 秒超时防护
        const renderPromise = renderAsync(blob, container, undefined, {
          className: 'docx-viewer',
          inWrapper: true,
          ignoreWidth: false,
          ignoreHeight: false,
          breakPages: true,
          renderHeaders: true,
          renderFooters: true,
          renderFootnotes: true,
          renderEndnotes: true,
          renderComments: false,
          useBase64URL: false,
        });

        const timeoutPromise = new Promise<void>((_, reject) => {
          timer = setTimeout(() => reject(new Error('文档渲染超时')), 10000);
        });

        await Promise.race([renderPromise, timeoutPromise]);

        if (timer) clearTimeout(timer);

        if (!cancelled) setLoading(false);
      } catch (err) {
        console.error('[DocxViewer] render failed:', err);
        if (timer) clearTimeout(timer);
        if (!cancelled) {
          setError((err as Error).message || '渲染文档失败');
          setLoading(false);
        }
      }
    }

    render();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [data]);

  if (error) {
    return (
      <div style={{ padding: 32, textAlign: 'center', color: 'var(--color-text-muted)' }}>
        <p>⚠️ 文档渲染失败</p>
        <p style={{ fontSize: 12, marginTop: 8 }}>{error}</p>
      </div>
    );
  }

  return (
    <div style={{ position: 'relative', minHeight: 200 }}>
      {loading && (
        <div style={{
          position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center',
          background: 'var(--color-bg-primary)', zIndex: 1,
        }}>
          <span style={{ color: 'var(--color-text-muted)', fontSize: 13 }}>加载文档中...</span>
        </div>
      )}
      <div ref={containerRef} style={{ padding: '12px 8px' }} />
    </div>
  );
}