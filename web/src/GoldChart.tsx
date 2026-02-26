import React from "react";
import type { NewsItem } from "./types";
import { DayChart } from "./DayChart";

const GRAMS_PER_OUNCE = 31.1034768;

interface Props {
  items: NewsItem[];
}

export const GoldChart: React.FC<Props> = ({ items }) => {
  if (items.length === 0) {
    return <div className="status">暂无黄金价格数据</div>;
  }

  const toPerGram = (v: number) => (v > 10000 ? v / GRAMS_PER_OUNCE : v);

  const sorted = [...items].sort(
    (a, b) => new Date(a.publishedAt).getTime() - new Date(b.publishedAt).getTime()
  );

  const points = sorted.map((i) => ({
    time: i.publishedAt,
    value: toPerGram(i.hotScore),
  }));

  const first = sorted[0];
  const last = sorted[sorted.length - 1];
  const firstPrice = toPerGram(first.hotScore);
  const lastPrice = toPerGram(last.hotScore);
  const goldChangePct = firstPrice > 0 ? ((lastPrice - firstPrice) / firstPrice) * 100 : 0;
  const goldChangeDisplay = goldChangePct.toFixed(2);
  const goldUp = goldChangePct >= 0;

  return (
    <div className="gold-wrapper">
      <div className="gold-header">
        <div>
          <div className="gold-title">黄金价格（XAU/人民币）</div>
          <div className="gold-subtitle">
            最新价：<span className="gold-price">{lastPrice.toFixed(2)}</span> 元/克
            <span
              className={`home-finance-change ${goldUp ? "up" : "down"}`}
              style={{ marginLeft: 8 }}
            >
              {goldUp && goldChangeDisplay !== "0.00" && "+"}
              {goldChangeDisplay}%
            </span>
          </div>
        </div>
        <div className="gold-time">
          {new Date(last.publishedAt).toLocaleString("zh-CN", { hour12: false })}
        </div>
      </div>
      <DayChart points={points} color={["#f59e0b", "#ef7d1a"]} id="gold" />
    </div>
  );
};
