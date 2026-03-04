import React, { useEffect, useState } from "react";
import { fetchIranWarCost } from "./api";
import type { IranWarCost } from "./types";

const usdFormatter = new Intl.NumberFormat("en-US", {
  maximumFractionDigits: 0,
});

export const IranCostTicker: React.FC = () => {
  const [data, setData] = useState<IranWarCost | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [displayTotal, setDisplayTotal] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      try {
        setLoading(true);
        setError(null);
        const result = await fetchIranWarCost();
        if (!cancelled) {
          setData(result);
        }
      } catch (e: any) {
        if (!cancelled) {
          setError(e?.message || "获取 Iran War Cost Tracker 数据失败");
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void load();
    // 每分钟自动刷新一次
    const timer = setInterval(() => {
      void load();
    }, 60_000);

    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

  // 使用 perSecond 在前端做线性外推，让数值在两次刷新之间平滑增长
  useEffect(() => {
    if (!data || !Number.isFinite(data.total) || !Number.isFinite(data.perSecond)) {
      setDisplayTotal(null);
      return;
    }
    const baseTotal = data.total;
    const perSec = data.perSecond;
    const baseTime = Date.now();

    const update = () => {
      const elapsed = (Date.now() - baseTime) / 1000;
      setDisplayTotal(baseTotal + perSec * elapsed);
    };

    update();
    // 你选择了“每秒刷新一次”，这里采用 1s 更新节奏，既平滑又不刺眼
    const timer = setInterval(update, 1000);
    return () => clearInterval(timer);
  }, [data?.total, data?.perSecond, data?.fetchedAt]);

  const totalValue =
    displayTotal != null && Number.isFinite(displayTotal)
      ? displayTotal
      : data && Number.isFinite(data.total)
        ? data.total
        : NaN;

  const totalDisplay =
    !Number.isNaN(totalValue) ? `$${usdFormatter.format(totalValue)}` : "-";

  return (
    <div className="home-card iran-cost-card">
      <div className="home-card-header">
        <h3 className="home-card-name">伊朗战争成本（实时抓取）</h3>
        <a
          href="https://iran-cost-ticker.com/"
          target="_blank"
          rel="noreferrer"
          className="home-card-more"
        >
          数据来源 · Iran War Cost Tracker →
        </a>
      </div>
      <div className="iran-cost-body">
        {loading && !data && (
          <div className="iran-cost-main">
            <span className="iran-cost-label">
              正在从 Iran War Cost Tracker 抓取最新估算...
            </span>
          </div>
        )}
        {!loading && error && !data && (
          <div className="iran-cost-main">
            <span className="iran-cost-label">
              暂时无法获取 Iran War Cost Tracker 数据：{error}
            </span>
          </div>
        )}
        {data && (
          <>
            <div className="iran-cost-main">
              <span className="iran-cost-label">
                自 2026-02-28 以来的估算总成本
              </span>
              <div className="iran-cost-total">{totalDisplay}</div>
            </div>
            <p className="iran-cost-note">
              本卡片通过后端服务直接抓取{" "}
              <span className="iran-cost-em">Iran War Cost Tracker</span>{" "}
              页面的「Operation Epic Fury — Est. U.S. Cost Since Strikes
              Began」数值，并定期刷新展示。实际财政成本可能因更多隐性开支而更高。
            </p>
          </>
        )}
      </div>
    </div>
  );
}

