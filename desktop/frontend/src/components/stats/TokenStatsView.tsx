import { useState, useEffect } from 'react';
import { fetchTokenStats } from '../../lib/api';

interface TokenStatRow {
  model_name: string;
  total_input: number;
  total_output: number;
  total_tokens: number;
}

export default function TokenStatsView() {
  const [stats, setStats] = useState<TokenStatRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadStats();
  }, []);

  const loadStats = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await fetchTokenStats();
      setStats(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  };

  const totalTokens = stats.reduce((sum, s) => sum + s.total_tokens, 0);
  const totalInput = stats.reduce((sum, s) => sum + s.total_input, 0);
  const totalOutput = stats.reduce((sum, s) => sum + s.total_output, 0);

  return (
    <div className="flex-1 flex flex-col items-center overflow-y-auto p-8">
      <div className="w-full max-w-2xl">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-xl font-semibold" style={{ color: 'var(--color-text-primary)' }}>
            Token 用量统计
          </h1>
          <button
            onClick={loadStats}
            className="text-xs px-3 py-1.5 rounded-md transition-colors"
            style={{
              background: 'var(--color-bg-secondary)',
              color: 'var(--color-text-secondary)',
              border: '1px solid var(--color-border)',
            }}
          >
            刷新
          </button>
        </div>

        {loading && (
          <p className="text-sm text-center py-12" style={{ color: 'var(--color-text-muted)' }}>
            加载中...
          </p>
        )}

        {error && (
          <div
            className="text-sm p-3 rounded-md mb-4"
            style={{ background: 'rgba(239,68,68,0.1)', color: 'var(--color-error)' }}
          >
            {error}
          </div>
        )}

        {!loading && !error && stats.length === 0 && (
          <div className="text-center py-12">
            <p className="text-4xl mb-3">📊</p>
            <p className="text-sm" style={{ color: 'var(--color-text-muted)' }}>
              暂无 Token 使用数据
            </p>
            <p className="text-xs mt-1" style={{ color: 'var(--color-text-muted)' }}>
              发送消息后将自动统计
            </p>
          </div>
        )}

        {!loading && !error && stats.length > 0 && (
          <>
            {/* 总计卡片 */}
            <div className="grid grid-cols-3 gap-4 mb-6">
              <div
                className="rounded-lg p-4 text-center"
                style={{ background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border)' }}
              >
                <p className="text-2xl font-bold" style={{ color: 'var(--color-accent)' }}>
                  {totalInput.toLocaleString()}
                </p>
                <p className="text-xs mt-1" style={{ color: 'var(--color-text-muted)' }}>
                  总输入 Token
                </p>
              </div>
              <div
                className="rounded-lg p-4 text-center"
                style={{ background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border)' }}
              >
                <p className="text-2xl font-bold" style={{ color: 'var(--color-accent)' }}>
                  {totalOutput.toLocaleString()}
                </p>
                <p className="text-xs mt-1" style={{ color: 'var(--color-text-muted)' }}>
                  总输出 Token
                </p>
              </div>
              <div
                className="rounded-lg p-4 text-center"
                style={{ background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border)' }}
              >
                <p className="text-2xl font-bold" style={{ color: 'var(--color-text-primary)' }}>
                  {totalTokens.toLocaleString()}
                </p>
                <p className="text-xs mt-1" style={{ color: 'var(--color-text-muted)' }}>
                  总 Token 消耗
                </p>
              </div>
            </div>

            {/* 按模型明细 */}
            <div
              className="rounded-lg overflow-hidden"
              style={{ background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border)' }}
            >
              <div className="px-4 py-3 border-b" style={{ borderColor: 'var(--color-border)' }}>
                <h2 className="text-sm font-medium" style={{ color: 'var(--color-text-secondary)' }}>
                  按模型统计
                </h2>
              </div>
              <table className="w-full text-sm">
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--color-border)' }}>
                    <th className="text-left px-4 py-2 text-xs font-medium" style={{ color: 'var(--color-text-muted)' }}>
                      模型
                    </th>
                    <th className="text-right px-4 py-2 text-xs font-medium" style={{ color: 'var(--color-text-muted)' }}>
                      输入
                    </th>
                    <th className="text-right px-4 py-2 text-xs font-medium" style={{ color: 'var(--color-text-muted)' }}>
                      输出
                    </th>
                    <th className="text-right px-4 py-2 text-xs font-medium" style={{ color: 'var(--color-text-muted)' }}>
                      合计
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {stats.map((s) => (
                    <tr
                      key={s.model_name}
                      style={{ borderBottom: '1px solid var(--color-border)' }}
                    >
                      <td className="px-4 py-2.5 font-medium" style={{ color: 'var(--color-text-primary)' }}>
                        {s.model_name}
                      </td>
                      <td className="px-4 py-2.5 text-right" style={{ color: 'var(--color-text-secondary)' }}>
                        {s.total_input.toLocaleString()}
                      </td>
                      <td className="px-4 py-2.5 text-right" style={{ color: 'var(--color-text-secondary)' }}>
                        {s.total_output.toLocaleString()}
                      </td>
                      <td className="px-4 py-2.5 text-right" style={{ color: 'var(--color-text-primary)', fontWeight: 500 }}>
                        {s.total_tokens.toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </div>
    </div>
  );
}