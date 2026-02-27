import React from "react";
import type { NewsItem } from "./types";
import { DayChart, TIME_AXIS_ASHARE, type TimeAxis } from "./DayChart";

interface Props {
  items: NewsItem[];
}

const INDEX_COLORS: Record<string, [string, string]> = {
  "上证指数": ["#ef4444", "#dc2626"],
  "深证成指": ["#3b82f6", "#2563eb"],
  "创业板指": ["#8b5cf6", "#7c3aed"],
};
const DEFAULT_COLOR: [string, string] = ["#6b7280", "#4b5563"];

// A 股指数主图的时间轴：去掉中午停市时间，仅在关键点标注 X 轴刻度
const ASHARE_TIME_AXIS_MAIN: TimeAxis = {
  segments: TIME_AXIS_ASHARE.segments,
  ticks: ["9:30", "11:30", "13:00", "15:00"],
};

// 根据数据点数量自动选择平滑窗口大小：
// - 点很少时不过度平滑，保留形状
// - 点越多时窗口越大，整体更平缓
function autoSmoothWindow(length: number): number {
  if (length <= 10) return 1;
  if (length <= 30) return 3;
  if (length <= 60) return 5;
  if (length <= 120) return 7;
  return 9;
}

export const AshareBlock: React.FC<Props> = ({ items }) => {
  if (items.length === 0) {
    return (
      <div className="ashare-wrapper">
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

  // 固定顺序：上证、深证、创业板，保证并排一行
  const order = ["上证指数", "深证成指", "创业板指"];
  const entries = [...groups.entries()].sort(
    (a, b) => order.indexOf(a[0]) - order.indexOf(b[0])
  );

  return (
    <div className="ashare-wrapper">
      <div className="ashare-charts">
        {entries.map(([title, groupItems]) => {
          const sorted = [...groupItems].sort(
            (a, b) => new Date(a.publishedAt).getTime() - new Date(b.publishedAt).getTime()
          );
          
          // 过滤：只保留 A 股交易时间内的数据点（9:30-11:30, 13:00-15:00）
          // 使用北京时间（UTC+8）判断，避免受用户浏览器时区影响
          const filtered = sorted.filter((item) => {
            const d = new Date(item.publishedAt);
            const beijingH = (d.getUTCHours() + 8) % 24;
            const m = d.getUTCMinutes();
            const min = beijingH * 60 + m;
            return (min >= 9 * 60 + 30 && min <= 11 * 60 + 30) || (min >= 13 * 60 && min <= 15 * 60);
          });
          
          const last = filtered[filtered.length - 1] || sorted[sorted.length - 1];
          const change = (last.extraData?.change as string | undefined) ?? "";
          const preClose = last.extraData?.preClose as number | undefined;
          const isUp = change === "" || !change.startsWith("-");
          const changeDisplay = change !== "" ? change : "0.00";
          const color = INDEX_COLORS[title] ?? DEFAULT_COLOR;
          const points = filtered.map((i) => ({
            time: i.publishedAt,
            value: i.hotScore,
          }));
          const smoothWindow = autoSmoothWindow(points.length);
          const idSlug = title.replace(/[^a-zA-Z0-9]/g, "");

          return (
            <div key={title} className="ashare-chart-item">
              <div className="ashare-chart-header">
                <span className="ashare-chart-name">{title}</span>
                <span className={`ashare-chart-price ${isUp ? "up" : "down"}`}>
                  {last.hotScore.toFixed(2)}
                </span>
                <span className={`ashare-chart-change ${isUp ? "up" : "down"}`}>
                  {isUp && changeDisplay !== "0.00" && "+"}
                  {changeDisplay}%
                </span>
              </div>
              <DayChart
                points={points}
                color={color}
                height={60}
                id={`ashare-${idSlug}`}
                timeAxis={ASHARE_TIME_AXIS_MAIN}
                showYAxis={false}
                showXAxis
                showBaseline={true}
                baselineValue={preClose}
                smoothWindow={smoothWindow}
              />
            </div>
          );
        })}
      </div>
    </div>
  );
};
