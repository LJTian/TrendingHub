import React from "react";
import type { NewsItem } from "./types";
import { DayChart, TIME_AXIS_ASHARE } from "./DayChart";

interface Props {
  items: NewsItem[];
}

const INDEX_COLORS: Record<string, [string, string]> = {
  "上证指数": ["#ef4444", "#dc2626"],
  "深证成指": ["#3b82f6", "#2563eb"],
  "创业板指": ["#8b5cf6", "#7c3aed"],
};
const DEFAULT_COLOR: [string, string] = ["#6b7280", "#4b5563"];

export const AshareBlock: React.FC<Props> = ({ items }) => {
  if (items.length === 0) {
    return (
      <div className="ashare-wrapper">
        <h3 className="ashare-title">A 股指数</h3>
        <div className="ashare-empty">暂无 A 股指数数据，请稍后刷新或检查网络。</div>
      </div>
    );
  }

  // 按 title 分组，每个指数一条时间序列
  const groups = new Map<string, NewsItem[]>();
  for (const item of items) {
    const list = groups.get(item.title) ?? [];
    list.push(item);
    groups.set(item.title, list);
  }

  return (
    <div className="ashare-wrapper">
      <h3 className="ashare-title">A 股指数</h3>
      <div className="ashare-charts">
        {[...groups.entries()].map(([title, groupItems]) => {
          const sorted = [...groupItems].sort(
            (a, b) => new Date(a.publishedAt).getTime() - new Date(b.publishedAt).getTime()
          );
          const last = sorted[sorted.length - 1];
          const change = last.extraData?.change as string | undefined;
          const isUp = change ? !change.startsWith("-") : true;
          const color = INDEX_COLORS[title] ?? DEFAULT_COLOR;
          const points = sorted.map((i) => ({
            time: i.publishedAt,
            value: i.hotScore,
          }));
          const idSlug = title.replace(/[^a-zA-Z0-9]/g, "");

          return (
            <div key={title} className="ashare-chart-item">
              <div className="ashare-chart-header">
                <div>
                  <span className="ashare-chart-name">{title}</span>
                  <span className={`ashare-chart-price ${isUp ? "up" : "down"}`}>
                    {last.hotScore.toFixed(2)}
                  </span>
                  {change && (
                    <span className={`ashare-chart-change ${isUp ? "up" : "down"}`}>
                      {isUp && "+"}{change}%
                    </span>
                  )}
                </div>
                <span className="ashare-chart-time">
                  {new Date(last.publishedAt).toLocaleString("zh-CN", { hour12: false })}
                </span>
              </div>
              <DayChart points={points} color={color} height={120} id={`ashare-${idSlug}`} timeAxis={TIME_AXIS_ASHARE} />
            </div>
          );
        })}
      </div>
    </div>
  );
};
