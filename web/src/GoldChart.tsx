import React from "react";
import type { NewsItem } from "./types";

interface Props {
  items: NewsItem[];
}

export const GoldChart: React.FC<Props> = ({ items }) => {
  if (items.length === 0) {
    return <div className="status">暂无黄金价格数据</div>;
  }

  // 按时间从早到晚排序
  const sorted = [...items].sort(
    (a, b) =>
      new Date(a.publishedAt).getTime() - new Date(b.publishedAt).getTime()
  );

  const prices = sorted.map((i) => i.hotScore);
  const min = Math.min(...prices);
  const max = Math.max(...prices);
  const range = max - min || 1;

  const points = sorted.map((item, idx) => {
    const x =
      sorted.length === 1 ? idx * 100 : (idx / (sorted.length - 1)) * 100; // 只有一个点时画水平线
    const norm = (item.hotScore - min) / range;
    const y = 90 - norm * 70; // 留上下边距
    return `${x},${y}`;
  });

  const last = sorted[sorted.length - 1];

  return (
    <div className="gold-wrapper">
      <div className="gold-header">
        <div>
          <div className="gold-title">黄金价格（XAU/USD）</div>
          <div className="gold-subtitle">
            最新价：<span className="gold-price">{last.hotScore.toFixed(2)}</span>
          </div>
        </div>
        <div className="gold-time">
          更新时间：
          {new Date(last.publishedAt).toLocaleString("zh-CN", {
            hour12: false
          })}
        </div>
      </div>

      <div className="gold-chart-container">
        <svg viewBox="0 0 100 100" className="gold-chart">
          <defs>
            <linearGradient id="gold-line" x1="0" y1="0" x2="1" y2="0">
              <stop offset="0%" stopColor="#f59e0b" />
              <stop offset="100%" stopColor="#f97316" />
            </linearGradient>
          </defs>
          <polyline
            fill="none"
            stroke="url(#gold-line)"
            strokeWidth="1.5"
            strokeLinejoin="round"
            strokeLinecap="round"
            points={points.join(" ")}
          />
        </svg>
      </div>
    </div>
  );
};

