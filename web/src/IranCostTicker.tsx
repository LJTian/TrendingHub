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
          setError(e?.message || "获取伊朗战争成本数据失败");
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void load();
    const timer = setInterval(() => {
      void load();
    }, 60_000);

    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

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
          数据来源 →
        </a>
      </div>
      <div className="iran-cost-body">
        {loading && !data && (
          <div className="iran-cost-main">
            <span className="iran-cost-label">
              正在抓取伊朗战争成本最新估算...
            </span>
          </div>
        )}
        {!loading && error && !data && (
          <div className="iran-cost-main">
            <span className="iran-cost-label">
              暂时无法获取数据：{error}
            </span>
          </div>
        )}
        {data && (
          <>
            <div className="iran-cost-main">
              <span className="iran-cost-label">
                自 2026-02-28 以来美国估算总成本
              </span>
              <div className="iran-cost-total">{totalDisplay}</div>
            </div>
            <p className="iran-cost-note">
              本卡片通过后端服务抓取{" "}
              <span className="iran-cost-em">Iran War Cost Tracker</span>{" "}
              页面的"史诗之怒"行动估算成本，每 2 小时采集一次并在前端实时外推。
              实际财政成本可能更高。
            </p>
          </>
        )}
      </div>
    </div>
  );
};
