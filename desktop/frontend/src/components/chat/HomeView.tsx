import { useAuthStore, useWorkspaceStore } from '../../lib/store';
import ChatInput from './ChatInput';
import { fetchTokenStats, type ModelConfig } from '../../lib/api';
import { useEffect, useState } from 'react';

interface TokenStat {
  model_name: string;
  total_input: number;
  total_output: number;
  total_tokens: number;
}

interface HomeViewProps {
  onSend: (prompt: string) => void;
  onCancel?: () => void;
  disabled: boolean;
  isStreaming?: boolean;
  models?: ModelConfig[];
  selectedModel?: string;
  onModelChange?: (name: string) => void;
}

/** 功能卡片数据 */
interface FeatureCard {
  id: string;
  icon: React.ReactNode;
  color: string;
  bgColor: string;
  title: string;
  desc: string;
}

const FEATURE_CARDS: FeatureCard[] = [
  {
    id: 'read',
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
        <path d="M4 6h16M4 12h10M4 18h8" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round"/>
        <circle cx="18" cy="17" r="3" stroke="currentColor" strokeWidth="1.5"/>
        <path d="M20 15l1.5 1.5L23 15" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
      </svg>
    ),
    color: '#6366f1',
    bgColor: 'rgba(99, 102, 241, 0.08)',
    title: 'AI 阅读',
    desc: '智能总结与分析文档',
  },
  {
    id: 'translate',
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
        <path d="M2 5h7m6 0h7M5 5c0 4 2 8 7 12C16 13 18 9 18 5M8 12l-3 7m14-7l3 7" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"/>
      </svg>
    ),
    color: '#8b5cf6',
    bgColor: 'rgba(139, 92, 246, 0.08)',
    title: 'AI 翻译',
    desc: '多语言实时翻译',
  },
  {
    id: 'draw',
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
        <rect x="3" y="3" width="18" height="18" rx="4" stroke="currentColor" strokeWidth="1.8"/>
        <circle cx="8.5" cy="10" r="1.5" fill="currentColor"/>
        <path d="M16 7l-7 10-3-2" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"/>
      </svg>
    ),
    color: '#ec4899',
    bgColor: 'rgba(236, 72, 153, 0.08)',
    title: 'AI 画图',
    desc: '根据描述生成图像',
  },
  {
    id: 'search',
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
        <circle cx="11" cy="11" r="7" stroke="currentColor" strokeWidth="1.8"/>
        <path d="M16.5 16.5L21 21" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round"/>
        <path d="M8 11h6M11 8v6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
      </svg>
    ),
    color: '#f59e0b',
    bgColor: 'rgba(245, 158, 11, 0.08)',
    title: 'AI 搜索',
    desc: '联网深度搜索信息',
  },
  {
    id: 'file',
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
        <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"/>
        <path d="M14 2v6h6" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"/>
        <path d="M8 13h8M8 17h5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
      </svg>
    ),
    color: '#06b6d4',
    bgColor: 'rgba(6, 182, 212, 0.08)',
    title: '文件处理',
    desc: '批量处理分析文件',
  },
  {
    id: 'data',
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
        <path d="M3 3v18h18" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"/>
        <path d="M7 16l4-5 3 2 4-6" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"/>
      </svg>
    ),
    color: '#22c55e',
    bgColor: 'rgba(34, 197, 94, 0.08)',
    title: '数据处理',
    desc: '分析与可视化数据',
  },
];

/** 建议提示词 */
const SUGGESTIONS = [
  '帮我总结这份文档的核心要点',
  '写一份项目周报',
  '这段代码有什么问题？',
  '帮我规划一个旅行攻略',
];

export default function HomeView({ onSend, onCancel, disabled, isStreaming, models, selectedModel, onModelChange }: HomeViewProps) {
  const username = useAuthStore((s) => s.username);
  const activeWorkspaceId = useWorkspaceStore((s) => s.activeWorkspaceId);
  const [stats, setStats] = useState<TokenStat[]>([]);

  useEffect(() => {
    fetchTokenStats().then(setStats).catch(() => {});
  }, [activeWorkspaceId]);

  const handleCardClick = (title: string) => {
    if (disabled) return;
    onSend(`使用「${title}」功能，帮我处理`);
  };

  const handleSuggestionClick = (text: string) => {
    if (disabled) return;
    onSend(text);
  };

  return (
    <div className="home-view" style={{ paddingBottom: 8 }}>
      {/* 欢迎语 — 偏上放置 */}
      <h1 className="home-view__title">Hello, {username || 'User'}</h1>
      <p className="home-view__subtitle">有什么可以帮你的？</p>

      {/* 功能卡片 — 2×3 网格，居中 */}
      <div className="home-view__content">
        <div style={{ maxWidth: 680, width: '100%' }}>
          <div className="feature-cards-grid">
            {FEATURE_CARDS.map((card) => (
              <button
                key={card.id}
                onClick={() => handleCardClick(card.title)}
                disabled={disabled}
                className="feature-card"
              >
                <div
                  className="feature-card__icon"
                  style={{ background: card.bgColor, color: card.color }}
                >
                  {card.icon}
                </div>
                <span className="feature-card__title">{card.title}</span>
                <span className="feature-card__desc">{card.desc}</span>
              </button>
            ))}
          </div>

          {/* 快捷提示短句 */}
          <div className="home-suggestions">
            {SUGGESTIONS.map((text, i) => (
              <button
                key={i}
                onClick={() => handleSuggestionClick(text)}
                disabled={disabled}
                className="home-suggestion-chip"
              >
                {text}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* 输入框 — 居中缩短 */}
      <div style={{ width: '100%' }}>
        <ChatInput
          onSend={onSend}
          onCancel={onCancel}
          disabled={disabled}
          isStreaming={isStreaming}
          stats={stats}
          wide
          hideTokens
          models={models}
          selectedModel={selectedModel}
          onModelChange={onModelChange}
        />
      </div>
    </div>
  );
}