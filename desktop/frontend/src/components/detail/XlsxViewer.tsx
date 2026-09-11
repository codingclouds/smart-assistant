import { useEffect, useMemo, useState } from 'react';
import * as XLSX from 'xlsx';

interface XlsxViewerProps {
  /** base64 编码的 .xlsx 文件内容 */
  data: string;
  fileName: string;
}

export default function XlsxViewer({ data, fileName }: XlsxViewerProps) {
  const [activeSheet, setActiveSheet] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [workbook, setWorkbook] = useState<XLSX.WorkBook | null>(null);

  useEffect(() => {
    let cancelled = false;
    try {
      const binary = atob(data);
      const bytes = new Uint8Array(binary.length);
      for (let i = 0; i < binary.length; i++) {
        bytes[i] = binary.charCodeAt(i);
      }
      console.log('[XlsxViewer] parsing xlsx, buffer size:', bytes.byteLength, 'bytes');
      const wb = XLSX.read(bytes.buffer, { type: 'array' });
      if (!cancelled) {
        setWorkbook(wb);
        setError(null);
      }
    } catch (err) {
      console.error('[XlsxViewer] parse failed:', err);
      if (!cancelled) {
        setWorkbook(null);
        setError((err as Error).message || '解析表格文件失败');
      }
    }
    return () => { cancelled = true; };
  }, [data]);

  const sheets = workbook ? workbook.SheetNames : [];
  const currentSheetName = sheets[activeSheet] || '';

  const sheetData = useMemo(() => {
    if (!workbook || !currentSheetName) return null;
    try {
      const ws = workbook.Sheets[currentSheetName];
      return XLSX.utils.sheet_to_json<string[]>(ws, { header: 1, defval: '' });
    } catch {
      return null;
    }
  }, [workbook, currentSheetName]);

  const MAX_ROWS = 500;

  if (error) {
    return (
      <div style={{ padding: 32, textAlign: 'center', color: 'var(--color-text-muted)' }}>
        <p>⚠️ 表格渲染失败</p>
        <p style={{ fontSize: 12, marginTop: 8 }}>{error}</p>
      </div>
    );
  }

  if (workbook && (!sheetData || sheetData.length === 0)) {
    return (
      <div style={{ padding: 32, textAlign: 'center', color: 'var(--color-text-muted)' }}>
        <p>表格中没有数据</p>
      </div>
    );
  }

  if (!workbook) {
    return (
      <div style={{ padding: 32, textAlign: 'center', color: 'var(--color-text-muted)' }}>
        <p>加载表格中...</p>
      </div>
    );
  }

  const headerRow = sheetData![0] as string[];
  const dataRows = sheetData!.slice(1);
  const displayRows = dataRows.slice(0, MAX_ROWS);
  const truncated = dataRows.length > MAX_ROWS;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Sheet 标签栏 */}
      {sheets.length > 1 && (
        <div style={{
          display: 'flex', gap: 2, padding: '6px 8px', overflowX: 'auto',
          borderBottom: '1px solid var(--color-border)', flexShrink: 0,
        }}>
          {sheets.map((name, idx) => (
            <button
              key={name}
              onClick={() => setActiveSheet(idx)}
              style={{
                padding: '4px 14px', borderRadius: 6, border: 'none', cursor: 'pointer',
                fontSize: 12, fontWeight: idx === activeSheet ? 600 : 400,
                whiteSpace: 'nowrap',
                background: idx === activeSheet ? 'var(--color-accent)' : 'transparent',
                color: idx === activeSheet ? '#fff' : 'var(--color-text-secondary)',
                transition: 'all 0.15s',
              }}
            >
              {name}
            </button>
          ))}
        </div>
      )}

      {/* 表格容器 */}
      <div style={{ flex: 1, overflow: 'auto' }}>
        <table style={{
          width: '100%', borderCollapse: 'collapse', fontSize: 13,
          fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
        }}>
          <thead>
            <tr>
              {headerRow.map((cell, ci) => (
                <th key={ci} style={{
                  background: 'rgba(255,255,255,0.09)', color: '#ebecee', fontWeight: 600,
                  padding: '9px 14px', textAlign: 'left', borderBottom: '2px solid rgba(255,255,255,0.14)',
                  fontSize: 12, letterSpacing: '.02em', whiteSpace: 'nowrap',
                  position: 'sticky', top: 0, zIndex: 1,
                }}>
                  {String(cell) || ' '}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {displayRows.map((row, ri) => (
              <tr key={ri} style={{
                background: ri % 2 === 0 ? 'rgba(255,255,255,0.005)' : 'rgba(255,255,255,0.018)',
              }}>
                {headerRow.map((_, ci) => (
                  <td key={ci} style={{
                    padding: '7px 14px', borderBottom: '1px solid rgba(255,255,255,0.05)',
                    color: '#cfd1d6', whiteSpace: 'nowrap', verticalAlign: 'middle',
                  }}>
                    {formatCell(row[ci])}
                  </td>
                ))}
              </tr>
            ))}
            {truncated && (
              <tr>
                <td colSpan={headerRow.length} style={{
                  padding: 16, textAlign: 'center', color: 'var(--color-text-muted)', fontSize: 12,
                }}>
                  … 表格过大，仅显示前 {MAX_ROWS} 行（共 {dataRows.length} 行）
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/** 格式化单元格值 */
function formatCell(val: unknown): string {
  if (val === null || val === undefined || val === '') return ' ';
  const s = String(val);
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}