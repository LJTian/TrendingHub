import React from "react";

export interface DataPoint {
  time: string; // ISO date string
  value: number;
}

/**
 * 时间轴区间定义。
 * A 股有午休，所以用 segments 表示多段连续交易时间，
 * 各段在横轴上无缝拼接，中间跳过非交易时间。
 */
export interface TimeAxis {
  /** 多段交易时间，每段 [startMin, endMin]（当日 0 点起的分钟数） */
  segments: [number, number][];
  /** 横轴刻度标签 */
  ticks: string[];
}

/** 全天 0:00–24:00 */
export const TIME_AXIS_24H: TimeAxis = {
  segments: [[0, 1440]],
  ticks: ["00:00", "04:00", "08:00", "12:00", "16:00", "20:00", "24:00"],
};

/** A 股交易时间 9:30–11:30, 13:00–15:00（中午停市时间在 X 轴上被“压缩”掉） */
export const TIME_AXIS_ASHARE: TimeAxis = {
  segments: [
    [9 * 60 + 30, 11 * 60 + 30], // 9:30 – 11:30
    [13 * 60, 15 * 60], // 13:00 – 15:00
  ],
  // 刻度包含上午和下午的关键时间点，11:30 / 13:00 会在压缩后出现在“中间”位置
  ticks: ["9:30", "10:00", "10:30", "11:30", "13:00", "13:30", "14:00", "15:00"],
};

interface Props {
  points: DataPoint[];
  color?: [string, string];
  height?: number;
  id?: string;
  timeAxis?: TimeAxis;
  /** 是否显示左侧 Y 轴数值，默认 true */
  showYAxis?: boolean;
  /** 是否显示底部 X 轴时间标签，默认 true */
  showXAxis?: boolean;
  /** 额外的根节点 className，默认 'gc' */
  className?: string;
  /** 是否显示基准线（第一个点的水平线），默认 false */
  showBaseline?: boolean;
  /** 是否显示背景网格线，默认 true；迷你图可以关闭以更简洁 */
  showGrid?: boolean;
  /** Y 轴上下留白比例，默认 0.12；迷你图可以调小一点 */
  padRatio?: number;
  /**
   * X 轴模式：
   * - 'time'（默认）：按交易时间映射到 0–1（会保留空白时间段，例如午休）
   * - 'sequence'：按点序号均匀铺满 0–1，更适合迷你图，避免看起来“偏右”
   */
  xMode?: "time" | "sequence";
  /**
   * 平滑窗口大小（点数）。>1 时使用简单滑动平均对 value 做平滑，仅影响折线展示，
   * 不改变原始数据点的数值（如标题中的最新价仍使用原始 hotScore）。
   */
  smoothWindow?: number;
}

function minutesOfDay(iso: string): number {
  const d = new Date(iso);
  // 强制使用 Asia/Shanghai (UTC+8) 时区计算小时数
  // 因为 A 股交易时间基于北京时间，不应受用户浏览器时区影响
  const utcHours = d.getUTCHours();
  const utcMinutes = d.getUTCMinutes();
  const utcSeconds = d.getUTCSeconds();
  // UTC+8 转换
  const beijingHours = (utcHours + 8) % 24;
  return beijingHours * 60 + utcMinutes + utcSeconds / 60;
}

/** 把当日分钟数映射到 0–1 横轴位置（跳过非交易时间） */
function minutesToNx(min: number, segments: [number, number][]): number {
  const totalLen = segments.reduce((s, [a, b]) => s + (b - a), 0);
  if (totalLen === 0) return 0;
  let elapsed = 0;
  for (const [start, end] of segments) {
    if (min <= start) return elapsed / totalLen;
    if (min >= end) {
      elapsed += end - start;
      continue;
    }
    elapsed += min - start;
    return elapsed / totalLen;
  }
  return 1;
}

/** 把刻度标签 "HH:MM" 转成当日分钟数 */
function tickToMinutes(label: string): number {
  const [h, m] = label.split(":").map(Number);
  return h * 60 + m;
}

export const DayChart: React.FC<Props> = ({
  points,
  color = ["#f59e0b", "#ef7d1a"],
  height = 160,
  id = "dc",
  timeAxis = TIME_AXIS_24H,
  showYAxis = true,
  showXAxis = true,
  className,
  showBaseline = false,
  showGrid = true,
  padRatio = 0.12,
  xMode = "time",
  smoothWindow = 1,
}) => {
  if (points.length === 0) return null;

  const { segments, ticks } = timeAxis;

  const sorted = [...points].sort(
    (a, b) => new Date(a.time).getTime() - new Date(b.time).getTime()
  );

  // 基于简单滑动平均的平滑序列，仅用于绘图
  const k = Math.max(1, Math.floor(smoothWindow));
  const series =
    k <= 1
      ? sorted
      : sorted.map((p, idx) => {
          const start = Math.max(0, idx - k + 1);
          const slice = sorted.slice(start, idx + 1);
          const avg =
            slice.reduce((sum, item) => sum + item.value, 0) / slice.length;
          return { time: p.time, value: avg };
        });

  const values = series.map((p) => p.value);
  const rawMin = Math.min(...values);
  const rawMax = Math.max(...values);
  const rawRange = rawMax - rawMin || 1;
  const pad = rawRange * padRatio;
  const yMin = rawMin - pad;
  const yRange = (rawMax + pad) - yMin;

  // 基准线：第一个点的价格（开盘价），使用平滑后的第一个点
  const baselineValue = series[0].value;
  const baselineY = (1 - (baselineValue - yMin) / yRange) * 100;

  const count = series.length;
  const dataPoints = series.map((p, idx) => {
    const nx =
      xMode === "sequence"
        ? count === 1
          ? 0.5
          : idx / (count - 1)
        : minutesToNx(minutesOfDay(p.time), segments);
    return {
      nx,
      ny: 1 - (p.value - yMin) / yRange,
    };
  });

  const W = 1000;
  const H = 300;

  const svgPts = dataPoints.map((p) => `${p.nx * W},${p.ny * H}`);
  const polylineStr =
    svgPts.length === 1
      ? `0,${dataPoints[0].ny * H} ${W},${dataPoints[0].ny * H}`
      : svgPts.join(" ");

  const areaD =
    dataPoints.length === 1
      ? `M 0 ${H} L 0 ${dataPoints[0].ny * H} L ${W} ${dataPoints[0].ny * H} L ${W} ${H} Z`
      : `M 0 ${H} L ${dataPoints.map((p) => `${p.nx * W} ${p.ny * H}`).join(" L ")} L ${dataPoints[dataPoints.length - 1].nx * W} ${H} Z`;

  const priceTicks = [rawMax, (rawMin + rawMax) / 2, rawMin];
  const priceTickPcts = priceTicks.map((p) => (1 - (p - yMin) / yRange) * 100);
  const gridYs = priceTickPcts.map((pct) => (pct / 100) * H);

  // X 轴刻度位置（百分比）
  const tickPositions = ticks.map((label) => ({
    label,
    pct: minutesToNx(tickToMinutes(label), segments) * 100,
  }));

  const strokeId = `${id}-stroke`;
  const fillId = `${id}-fill`;
  const rootClass = className ?? "gc";

  return (
    <div className={rootClass}>
      {showYAxis && (
        <div className="gc-yaxis">
          {priceTicks.map((p, i) => (
            <span key={i} className="gc-ylabel" style={{ top: `${priceTickPcts[i]}%` }}>
              {p.toFixed(2)}
            </span>
          ))}
        </div>
      )}
      <div className="gc-plot">
        <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" className="gc-svg" style={{ height }}>
          <defs>
            <linearGradient id={strokeId} x1="0" y1="0" x2="1" y2="0">
              <stop offset="0%" stopColor={color[0]} />
              <stop offset="100%" stopColor={color[1]} />
            </linearGradient>
            <linearGradient id={fillId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color[0]} stopOpacity="0.18" />
              <stop offset="100%" stopColor={color[0]} stopOpacity="0.02" />
            </linearGradient>
          </defs>
          {showGrid &&
            gridYs.map((y, i) => (
              <line
                key={i}
                x1={0}
                y1={y}
                x2={W}
                y2={y}
                stroke="#e5e7eb"
                strokeWidth="1"
                vectorEffect="non-scaling-stroke"
              />
            ))}
          {showBaseline && (
            <line 
              x1={0} 
              y1={(baselineY / 100) * H} 
              x2={W} 
              y2={(baselineY / 100) * H} 
              stroke="currentColor" 
              strokeWidth="1" 
              strokeDasharray="4,4" 
              opacity="0.5" 
              vectorEffect="non-scaling-stroke" 
            />
          )}
          <path d={areaD} fill={`url(#${fillId})`} />
          <polyline
            fill="none"
            stroke={`url(#${strokeId})`}
            strokeWidth="2"
            strokeLinejoin="round"
            strokeLinecap="round"
            vectorEffect="non-scaling-stroke"
            points={polylineStr}
          />
        </svg>
        {showXAxis && (
          <div className="gc-xaxis">
            {tickPositions.map(({ label, pct }) => (
              <span key={label} className="gc-xlabel" style={{ position: "absolute", left: `${pct}%`, transform: "translateX(-50%)" }}>
                {label}
              </span>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};
