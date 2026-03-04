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

  const totalDisplay =
    data && Number.isFinite(data.total)
      ? `$${usdFormatter.format(data.total)}`
      : "-";

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

