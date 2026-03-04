import React, { useEffect, useState } from "react";
import { fetchIranWarCost } from "./api";
import type { IranWarCost } from "./types";

const usdFormatter = new Intl.NumberFormat("en-US", {
  maximumFractionDigits: 0,
});

const usdSmallFormatter = new Intl.NumberFormat("en-US", {
  maximumFractionDigits: 0,
});

export const IranWarChannel: React.FC = () => {
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
    const timer = setInterval(() => {
      void load();
    }, 60_000);

    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

  if (loading && !data) {
    return <div className="status">正在从 Iran War Cost Tracker 抓取数据...</div>;
  }

  if (!loading && error && !data) {
    return <div className="status error">{error}</div>;
  }

  if (!data) {
    return <div className="status">暂时没有可用数据</div>;
  }

  const totalDisplay = `$${usdFormatter.format(data.total)}`;
  const perSecondDisplay = `$${usdSmallFormatter.format(data.perSecond)}`;
  const perHourDisplay = `$${usdSmallFormatter.format(data.perHour)}`;
  const perDayDisplay = `$${usdSmallFormatter.format(data.perDay)}`;
  const discreteDisplay = `$${usdFormatter.format(data.discreteTotal)}`;

  const fetchedAtLocal = new Date(data.fetchedAt).toLocaleString("zh-CN", {
    hour12: false,
  });

  return (
    <>
      <section className="section">
        <h2 className="section-title">伊朗战争成本 · Operation Epic Fury</h2>
        <div className="iranwar-layout">
          <div className="iranwar-main-card">
            <div className="iranwar-main-header">
              <span className="iranwar-main-title">
                Operation Epic Fury — Est. U.S. Cost Since Strikes Began
              </span>
              <a
                href="https://iran-cost-ticker.com/"
                target="_blank"
                rel="noreferrer"
                className="iranwar-source-link"
              >
                数据来源 · Iran War Cost Tracker →
              </a>
            </div>
            <div className="iranwar-main-total">{totalDisplay}</div>
            <div className="iranwar-main-sub">
              <span className="iranwar-chip">
                日成本（当前阶段）：${usdSmallFormatter.format(data.perDay)}/day
              </span>
              <span className="iranwar-chip">
                作战成本小计：${usdFormatter.format(data.opsTotal)}
              </span>
              <span className="iranwar-chip">
                离散事件成本小计：{discreteDisplay}
              </span>
            </div>
            <div className="iranwar-main-footer">
              <span>当前阶段：{data.phase}</span>
              <span>数据时间：{fetchedAtLocal}（服务器时间）</span>
            </div>
          </div>

          <div className="iranwar-rate-grid">
            <div className="iranwar-rate-card">
              <div className="iranwar-rate-label">每秒成本</div>
              <div className="iranwar-rate-value">{perSecondDisplay}</div>
              <div className="iranwar-rate-tag">Sustained Operations</div>
            </div>
            <div className="iranwar-rate-card">
              <div className="iranwar-rate-label">每小时成本</div>
              <div className="iranwar-rate-value">{perHourDisplay}</div>
              <div className="iranwar-rate-tag">Sustained Operations</div>
            </div>
            <div className="iranwar-rate-card">
              <div className="iranwar-rate-label">每天成本</div>
              <div className="iranwar-rate-value">{perDayDisplay}</div>
              <div className="iranwar-rate-tag">Sustained Operations</div>
            </div>
          </div>
        </div>
      </section>

      <section className="section">
        <h2 className="section-title">Human Cost · 人员伤亡</h2>
        <div className="iranwar-human-grid">
          <div className="iranwar-human-card">
            <div className="iranwar-human-title">United States</div>
            <div className="iranwar-human-row">
              <span>Service members killed</span>
              <span className="iranwar-human-number iranwar-human-number-critical">
                {data.usServiceMembersKilled ?? "—"}
              </span>
            </div>
            <div className="iranwar-human-row">
              <span>Wounded</span>
              <span className="iranwar-human-number">
                {data.usWounded ?? "—"}
              </span>
            </div>
          </div>
          <div className="iranwar-human-card">
            <div className="iranwar-human-title">Iran</div>
            <div className="iranwar-human-row">
              <span>Military casualties (est.)</span>
              <span className="iranwar-human-number">
                {data.iranMilitaryCasualties ?? "—"}
              </span>
            </div>
            <div className="iranwar-human-row">
              <span>Civilian casualties (est.)</span>
              <span className="iranwar-human-number iranwar-human-number-critical">
                {data.iranCivilianCasualties ?? "—"}
              </span>
            </div>
          </div>
        </div>
      </section>
    </>
  );
};

