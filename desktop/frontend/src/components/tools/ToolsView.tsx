import { useEffect, useState } from 'react';
import { fetchTools, type ToolDef } from '../../lib/api';

// 工具名 → 图标映射
const TOOL_ICONS: Record<string, string> = {
  web_search: '🔍',
  get_current_time: '🕐',
  ask_user: '🤔',
  read_file: '📖',
  write_file: '✏️',
  execute_command: '💻',
  bash: '💻',
  web_fetch: '🌐',
  default: '🔧',
};

// 工具名 → 显示标签
const TOOL_LABELS: Record<string, string> = {
  web_search: '网页搜索',
  get_current_time: '实时时间',
  ask_user: '交互确认',
  read_file: '读取文件',
  write_file: '写入文件',
  execute_command: '执行命令',
  bash: '命令执行',
  web_fetch: '网页抓取',
  default: '工具',
};

export default function ToolsView() {
  const [tools, setTools] = useState<ToolDef[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchTools()
      .then((data) => {
        setTools(Array.isArray(data) ? data : []);
        setLoading(false);
      })
      .catch((err) => {
        setError(err.message || '加载失败');
        setLoading(false);
      });
  }, []);

  return (
    <div className="h-full flex flex-col" style={{ background: 'var(--color-bg-primary)' }}>
      {/* 顶栏 */}
      <div
        className="flex items-center justify-between px-6 py-4 border-b"
        style={{ borderColor: 'var(--color-border)' }}
      >
        <div>
          <h1 className="text-lg font-semibold" style={{ color: 'var(--color-text-primary)' }}>
            工具能力
          </h1>
          <p className="text-sm mt-1" style={{ color: 'var(--color-text-muted)' }}>
            Agent 当前可用的所有工具及其能力描述
          </p>
        </div>
        {!loading && (
          <span
            className="text-xs px-3 py-1.5 rounded-full"
            style={{
              background: 'var(--color-accent-muted)',
              color: 'var(--color-accent)',
            }}
          >
            {tools.length} 个工具
          </span>
        )}
      </div>

      {/* 内容区 */}
      <div className="flex-1 overflow-y-auto px-6 py-4">
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <div className="flex items-center gap-2" style={{ color: 'var(--color-text-muted)' }}>
              <div
                style={{
                  width: 16,
                  height: 16,
                  border: '2px solid var(--color-border)',
                  borderTopColor: 'var(--color-accent)',
                  borderRadius: '50%',
                  animation: 'spin 0.8s linear infinite',
                }}
              />
              <span className="text-sm">加载中...</span>
            </div>
          </div>
        ) : error ? (
          <div
            className="flex flex-col items-center justify-center py-20 gap-3"
            style={{ color: 'var(--color-error)' }}
          >
            <span className="text-3xl">⚠️</span>
            <span className="text-sm">{error}</span>
          </div>
        ) : tools.length === 0 ? (
          <div
            className="flex flex-col items-center justify-center py-20 gap-3"
            style={{ color: 'var(--color-text-muted)' }}
          >
            <span className="text-3xl">🔧</span>
            <span className="text-sm">暂无可用的工具</span>
          </div>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: 12 }}>
            {tools.map((tool) => {
              const icon = TOOL_ICONS[tool.name] || TOOL_ICONS.default;
              const label = TOOL_LABELS[tool.name] || tool.name;

              // 提取参数名称列表
              let paramNames: string[] = [];
              try {
                const params = tool.parameters as Record<string, unknown> | undefined;
                if (params && params.properties) {
                  paramNames = Object.keys(params.properties as Record<string, unknown>);
                }
              } catch {
                // ignore
              }

              return (
                <div
                  key={tool.name}
                  style={{
                    padding: '16px 18px',
                    borderRadius: 12,
                    border: '1px solid var(--color-border)',
                    background: 'var(--color-bg-secondary)',
                    transition: 'border-color 0.15s, box-shadow 0.15s',
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.borderColor = 'var(--color-accent)';
                    e.currentTarget.style.boxShadow = '0 2px 12px rgba(79, 110, 246, 0.1)';
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.borderColor = 'var(--color-border)';
                    e.currentTarget.style.boxShadow = 'none';
                  }}
                >
                  {/* 工具头部 */}
                  <div className="flex items-center gap-2 mb-2">
                    <span style={{ fontSize: 20 }}>{icon}</span>
                    <span
                      style={{
                        fontSize: 14,
                        fontWeight: 600,
                        color: 'var(--color-text-primary)',
                      }}
                    >
                      {label}
                    </span>
                    <code
                      style={{
                        fontSize: 11,
                        fontFamily: 'monospace',
                        color: 'var(--color-text-muted)',
                        marginLeft: 'auto',
                        padding: '2px 6px',
                        borderRadius: 4,
                        background: 'color-mix(in srgb, var(--color-bg-secondary) 60%, transparent)',
                      }}
                    >
                      {tool.name}
                    </code>
                  </div>

                  {/* 描述 */}
                  <p
                    style={{
                      fontSize: 13,
                      lineHeight: 1.6,
                      color: 'var(--color-text-secondary)',
                      marginBottom: paramNames.length > 0 ? 10 : 0,
                    }}
                  >
                    {tool.description}
                  </p>

                  {/* 参数列表 */}
                  {paramNames.length > 0 && (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                      <span
                        style={{
                          fontSize: 10,
                          fontWeight: 600,
                          textTransform: 'uppercase',
                          letterSpacing: '0.05em',
                          color: 'var(--color-text-muted)',
                          alignSelf: 'center',
                        }}
                      >
                        参数:
                      </span>
                      {paramNames.map((name) => (
                        <code
                          key={name}
                          style={{
                            fontSize: 11,
                            fontFamily: 'monospace',
                            color: 'var(--color-accent)',
                            padding: '2px 8px',
                            borderRadius: 4,
                            background: 'color-mix(in srgb, var(--color-accent) 8%, transparent)',
                          }}
                        >
                          {name}
                        </code>
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}