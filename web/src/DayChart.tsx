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

/** A 股交易时间 9:30–11:30, 13:00–15:00 */
export const TIME_AXIS_ASHARE: TimeAxis = {
  segments: [
    [9 * 60 + 30, 11 * 60 + 30], // 9:30 – 11:30
    [13 * 60, 15 * 60],           // 13:00 – 15:00
  ],
  ticks: ["9:30", "10:00", "10:30", "11:00", "11:30", "13:00", "13:30", "14:00", "14:30", "15:00"],
};

interface Props {
  points: DataPoint[];
  color?: [string, string];
  height?: number;
  id?: string;
  timeAxis?: TimeAxis;
}

function minutesOfDay(iso: string): number {
  const d = new Date(iso);
  return d.getHours() * 60 + d.getMinutes() + d.getSeconds() / 60;
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
}) => {
  if (points.length === 0) return null;

  const { segments, ticks } = timeAxis;

  const sorted = [...points].sort(
    (a, b) => new Date(a.time).getTime() - new Date(b.time).getTime()
  );

  const values = sorted.map((p) => p.value);
  const rawMin = Math.min(...values);
  const rawMax = Math.max(...values);
  const rawRange = rawMax - rawMin || 1;
  const pad = rawRange * 0.12;
  const yMin = rawMin - pad;
  const yRange = (rawMax + pad) - yMin;

  const dataPoints = sorted.map((p) => ({
    nx: minutesToNx(minutesOfDay(p.time), segments),
    ny: 1 - (p.value - yMin) / yRange,
  }));

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

  return (
    <div className="gc">
      <div className="gc-yaxis">
        {priceTicks.map((p, i) => (
          <span key={i} className="gc-ylabel" style={{ top: `${priceTickPcts[i]}%` }}>
            {p.toFixed(2)}
          </span>
        ))}
      </div>
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
          {gridYs.map((y, i) => (
            <line key={i} x1={0} y1={y} x2={W} y2={y} stroke="#e5e7eb" strokeWidth="1" vectorEffect="non-scaling-stroke" />
          ))}
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
        <div className="gc-xaxis">
          {tickPositions.map(({ label, pct }) => (
            <span key={label} className="gc-xlabel" style={{ position: "absolute", left: `${pct}%`, transform: "translateX(-50%)" }}>
              {label}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
};
