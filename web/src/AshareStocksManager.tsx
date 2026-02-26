import React, { useEffect, useState } from "react";
import { fetchAshareStocks, addAshareStock, removeAshareStock } from "./api";
import type { NewsItem } from "./types";
import { DayChart, TIME_AXIS_ASHARE, type TimeAxis } from "./DayChart";

const CODE_REG = /^\d{5,6}$/;

/** 从东方财富 URL 中提取 6 位股票代码，如 sh600050.html -> 600050 */
function codeFromUrl(url: string): string | null {
  const m = url.match(/(?:sh|sz)(\d{6})\.html/);
  return m ? m[1] : null;
}

export interface AshareStocksManagerProps {
  /** 自选股行情（来自金融频道 ashare，不含三大指数） */
  watchlistItems?: NewsItem[];
}

// 自选股迷你图：沿用 A 股交易时间，仅展示起止时间
const ASHARE_TIME_AXIS_START_END: TimeAxis = {
  segments: TIME_AXIS_ASHARE.segments,
  ticks: [
    TIME_AXIS_ASHARE.ticks[0],
    TIME_AXIS_ASHARE.ticks[TIME_AXIS_ASHARE.ticks.length - 1],
  ],
};

// 自选股迷你图平滑窗口：点数越多，窗口越大
function autoSmoothWindowMini(length: number): number {
  if (length <= 8) return 1;
  if (length <= 24) return 3;
  if (length <= 60) return 5;
  if (length <= 120) return 7;
  return 9;
}

export const AshareStocksManager: React.FC<AshareStocksManagerProps> = (props) => {
  const { watchlistItems = [] } = props;
  const [codes, setCodes] = useState<string[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const byCode = React.useMemo(() => {
    const map = new Map<string, NewsItem>();
    for (const item of watchlistItems) {
      const c = codeFromUrl(item.url);
      if (c) map.set(c, item);
    }
    return map;
  }, [watchlistItems]);

  const seriesByCode = React.useMemo(() => {
    const map = new Map<string, NewsItem[]>();
    for (const item of watchlistItems) {
      const c = codeFromUrl(item.url);
      if (!c) continue;
      const list = map.get(c) ?? [];
      list.push(item);
      map.set(c, list);
    }
    for (const [code, list] of map) {
      list.sort(
        (a, b) =>
          new Date(a.publishedAt).getTime() - new Date(b.publishedAt).getTime()
      );
      map.set(code, list);
    }
    return map;
  }, [watchlistItems]);

  const load = async () => {
    const list = await fetchAshareStocks();
    setCodes(list);
  };

  useEffect(() => {
    load();
  }, []);

  const handleAdd = async () => {
    const raw = input.trim();
    if (!raw) return;
    if (!CODE_REG.test(raw)) {
      setError("请输入 5 或 6 位股票代码");
      return;
    }
    setError(null);
    setLoading(true);
    try {
      await addAshareStock(raw);
      setInput("");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "添加失败");
    } finally {
      setLoading(false);
    }
  };

  const handleRemove = async (code: string) => {
    setLoading(true);
    try {
      await removeAshareStock(code);
      await load();
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="ashare-stocks-manager">
      <div className="ashare-stocks-head">
        <h3 className="ashare-stocks-title">自选股</h3>
        <div className="ashare-stocks-form">
          <input
            type="text"
            className="ashare-stocks-input"
            placeholder="输入 6 位代码，如 600519"
            value={input}
            onChange={(e) => setInput(e.target.value.replace(/\D/g, "").slice(0, 6))}
            onKeyDown={(e) => e.key === "Enter" && handleAdd()}
            maxLength={6}
          />
          <button
            type="button"
            className="ashare-stocks-btn"
            onClick={handleAdd}
            disabled={loading || !input.trim()}
          >
            添加
          </button>
        </div>
      </div>
      <p className="ashare-stocks-hint">添加后将在本模块展示行情，约 30 分钟更新一次。</p>
      {error && <div className="ashare-stocks-error">{error}</div>}
      {codes.length > 0 && (
        <ul className="ashare-stocks-list">
          {codes.map((code) => {
            const item = byCode.get(code);
            const change = (item?.extraData?.change as string | undefined) ?? "";
            const changeDisplay = change !== "" ? change : "0.00";
            const isUp = change === "" || !change.startsWith("-");
            const series = seriesByCode.get(code) ?? [];
            
            // 过滤：只保留 A 股交易时间内的数据点（9:30-11:30, 13:00-15:00）
            // 使用北京时间（UTC+8）判断，避免受用户浏览器时区影响
            const filteredSeries = series.filter((s) => {
              const d = new Date(s.publishedAt);
              const beijingH = (d.getUTCHours() + 8) % 24;
              const m = d.getUTCMinutes();
              const min = beijingH * 60 + m;
              return (min >= 9 * 60 + 30 && min <= 11 * 60 + 30) || (min >= 13 * 60 && min <= 15 * 60);
            });
            
            return (
              <li key={code} className="ashare-stocks-item">
                <button
                  type="button"
                  className="ashare-stocks-remove"
                  onClick={() => handleRemove(code)}
                  disabled={loading}
                  aria-label={`移除 ${code}`}
                >
                  ×
                </button>
                {item ? (
                  <>
                    <div className="ashare-stocks-item-row">
                      <div className="ashare-stocks-item-title">
                        <span className="ashare-stocks-code">{code}</span>
                        <a
                          href={item.url}
                          target="_blank"
                          rel="noreferrer"
                          className="ashare-stocks-name"
                        >
                          {item.title}
                        </a>
                      </div>
                    </div>
                    <div className="ashare-stocks-item-row">
                      <div className="ashare-stocks-price-row">
                        <span className={`ashare-stocks-price ${isUp ? "up" : "down"}`}>
                          {item.hotScore.toFixed(2)}
                        </span>
                        <span className={`ashare-stocks-change ${isUp ? "up" : "down"}`}>
                          {isUp && changeDisplay !== "0.00" && "+"}
                          {changeDisplay}%
                        </span>
                      </div>
                    </div>
                    {filteredSeries.length > 0 && (
                      <div className="ashare-stocks-chart">
                        {/*
                          先在这里构造点数组并计算自适应平滑窗口，
                          避免在 JSX 属性里做过多逻辑。
                        */}
                        {(() => {
                          const points = filteredSeries.map((s) => ({
                            time: s.publishedAt,
                            value: s.hotScore,
                          }));
                          const smoothWindow = autoSmoothWindowMini(points.length);
                          return (
                            <DayChart
                              points={points}
                              color={isUp ? ["#22c55e", "#16a34a"] : ["#ef4444", "#dc2626"]}
                              height={48}
                              id={`ashare-watch-${code}`}
                              timeAxis={ASHARE_TIME_AXIS_START_END}
                              showYAxis={false}
                              showXAxis={false}
                              showBaseline={true}
                              showGrid={false}
                              padRatio={0.06}
                              smoothWindow={smoothWindow}
                              className="gc gc-mini"
                            />
                          );
                        })()}
                      </div>
                    )}
                  </>
                ) : (
                  <span className="ashare-stocks-pending">行情更新中…</span>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
};
