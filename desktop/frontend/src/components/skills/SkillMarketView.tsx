import { useState, useEffect } from 'react';
import { fetchSkills, installSkill, type SkillInfo } from '../../lib/api';

const MARKET_SKILLS = [
  {
    id: 'code-reviewer',
    name: 'code-reviewer',
    description: '自动化代码审查，检测常见代码质量问题、安全漏洞和性能瓶颈',
    version: '1.0.0',
    author: 'Smart Assistant Team',
    icon: '🔍',
  },
  {
    id: 'doc-generator',
    name: 'doc-generator',
    description: '自动生成 API 文档、README 和代码注释',
    version: '0.9.0',
    author: 'Smart Assistant Team',
    icon: '📝',
  },
  {
    id: 'data-analyzer',
    name: 'data-analyzer',
    description: '数据分析与可视化，支持 CSV/JSON/Excel 多种格式',
    version: '1.2.0',
    author: 'Smart Assistant Team',
    icon: '📊',
  },
  {
    id: 'git-assistant',
    name: 'git-assistant',
    description: '智能 Git 操作助手，自动生成 commit message、管理分支和 PR',
    version: '1.1.0',
    author: 'Smart Assistant Team',
    icon: '🔀',
  },
  {
    id: 'translator',
    name: 'translator',
    description: '多语言翻译引擎，支持 100+ 语言互译，保持技术文档格式',
    version: '2.0.0',
    author: 'Smart Assistant Team',
    icon: '🌐',
  },
  {
    id: 'test-writer',
    name: 'test-writer',
    description: '自动生成单元测试和集成测试用例，支持多种测试框架',
    version: '0.8.0',
    author: 'Smart Assistant Team',
    icon: '🧪',
  },
];

export default function SkillMarketView() {
  const [installed, setInstalled] = useState<SkillInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [installing, setInstalling] = useState<string | null>(null);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  useEffect(() => {
    loadInstalled();
  }, []);

  const loadInstalled = async () => {
    try {
      const data = await fetchSkills();
      setInstalled(data);
    } catch (err) {
      console.error('Failed to load skills:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleInstall = async (skill: (typeof MARKET_SKILLS)[0]) => {
    setInstalling(skill.name);
    setMessage(null);
    try {
      const result = await installSkill(skill.name, skill.version);
      setInstalled((prev) => [...prev, result]);
      setMessage({ type: 'success', text: `"${skill.name}" 安装成功` });
    } catch (err) {
      setMessage({ type: 'error', text: `安装失败: ${err}` });
    } finally {
      setInstalling(null);
    }
  };

  const installedNames = new Set(installed.map((s) => s.name));

  return (
    <div className="flex-1 flex flex-col items-center overflow-y-auto p-8">
      <div className="w-full max-w-3xl">
        <h1
          className="text-2xl font-bold mb-1 tracking-tight"
          style={{ color: 'var(--color-text-primary)' }}
        >
          Skill 市场
        </h1>
        <p className="text-sm mb-8" style={{ color: 'var(--color-text-muted)' }}>
          浏览和安装社区 Skill，扩展 Agent 能力
        </p>

        {message && (
          <div
            className="text-sm px-4 py-3 rounded-xl mb-6"
            style={{
              background:
                message.type === 'success' ? 'var(--color-success-muted)' : 'var(--color-error-muted)',
              color: message.type === 'success' ? 'var(--color-success)' : 'var(--color-error)',
              border: `1px solid ${
                message.type === 'success'
                  ? 'rgba(74,222,128,0.2)'
                  : 'rgba(248,113,113,0.2)'
              }`,
            }}
          >
            {message.text}
          </div>
        )}

        {/* 已安装 Skills */}
        {!loading && installed.length > 0 && (
          <div className="mb-8">
            <h2
              className="text-sm font-semibold mb-3 px-1"
              style={{ color: 'var(--color-text-secondary)' }}
            >
              已安装 ({installed.length})
            </h2>
            <div className="flex flex-wrap gap-2">
              {installed.map((s) => (
                <span
                  key={s.id}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium"
                  style={{
                    background: s.enabled
                      ? 'var(--color-success-muted)'
                      : 'rgba(255,255,255,0.04)',
                    color: s.enabled ? 'var(--color-success)' : 'var(--color-text-muted)',
                    border: `1px solid ${
                      s.enabled ? 'rgba(74,222,128,0.3)' : 'var(--color-border)'
                    }`,
                  }}
                >
                  {s.name}
                  <span style={{ color: 'var(--color-text-muted)', opacity: 0.7 }}>
                    v{s.version}
                  </span>
                </span>
              ))}
            </div>
          </div>
        )}

        {/* 市场列表 */}
        <h2
          className="text-sm font-semibold mb-3 px-1"
          style={{ color: 'var(--color-text-secondary)' }}
        >
          可用 Skill ({MARKET_SKILLS.length})
        </h2>

        {loading && (
          <p
            className="text-sm text-center py-16"
            style={{ color: 'var(--color-text-muted)' }}
          >
            加载中...
          </p>
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {MARKET_SKILLS.map((skill) => {
            const isInstalled = installedNames.has(skill.name);
            const isProcessing = installing === skill.name;

            return (
              <div
                key={skill.id}
                className="rounded-2xl p-5 transition-all hover:border-[var(--color-border-light)]"
                style={{
                  background: 'var(--color-bg-secondary)',
                  border: `1px solid ${
                    isInstalled ? 'rgba(74,222,128,0.3)' : 'var(--color-border)'
                  }`,
                }}
              >
                <div className="flex items-start gap-3.5">
                  <span className="text-2xl flex-shrink-0">{skill.icon}</span>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <h3
                        className="text-sm font-semibold"
                        style={{ color: 'var(--color-text-primary)' }}
                      >
                        {skill.name}
                      </h3>
                      <span
                        className="text-[11px] px-1.5 py-0.5 rounded-md"
                        style={{
                          background: 'var(--color-bg-tertiary)',
                          color: 'var(--color-text-muted)',
                        }}
                      >
                        v{skill.version}
                      </span>
                    </div>
                    <p
                      className="text-xs mb-3 leading-relaxed"
                      style={{ color: 'var(--color-text-secondary)' }}
                    >
                      {skill.description}
                    </p>
                    <div className="flex items-center justify-between">
                      <span className="text-[11px]" style={{ color: 'var(--color-text-muted)' }}>
                        {skill.author}
                      </span>
                      <button
                        onClick={() => handleInstall(skill)}
                        disabled={isInstalled || isProcessing}
                        className="text-xs px-3.5 py-1.5 rounded-lg font-medium transition-all disabled:opacity-50 hover:brightness-110 active:scale-95"
                        style={{
                          background: isInstalled
                            ? 'var(--color-success-muted)'
                            : 'var(--color-accent)',
                          color: isInstalled ? 'var(--color-success)' : '#fff',
                        }}
                      >
                        {isProcessing
                          ? '安装中...'
                          : isInstalled
                          ? '已安装'
                          : '安装'}
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}