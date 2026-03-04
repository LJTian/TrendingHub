import React, { useEffect, useState } from "react";
import { fetchIranWarCost } from "./api";
import type { IranWarCost } from "./types";

const usdFormatter = new Intl.NumberFormat("en-US", {
  maximumFractionDigits: 0,
});

export const IranWarChannel: React.FC = () => {
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

  if (loading && !data) {
    return <div className="status">正在从 Iran War Cost Tracker 抓取数据（首次加载需约 10 秒）...</div>;
  }

  if (!loading && error && !data) {
    return <div className="status error">{error}</div>;
  }

  if (!data) {
    return <div className="status">暂时没有可用数据</div>;
  }

  const totalValue =
    displayTotal != null && Number.isFinite(displayTotal)
      ? displayTotal
      : data.total;

  const totalDisplay = `$${usdFormatter.format(totalValue)}`;
  const perSecondDisplay = `$${usdFormatter.format(data.perSecond)}`;
  const perHourDisplay = `$${usdFormatter.format(data.perHour)}`;
  const perDayDisplay = `$${usdFormatter.format(data.perDay)}`;
  const discreteDisplay = `$${usdFormatter.format(data.discreteTotal)}`;

  const fetchedAtLocal = new Date(data.fetchedAt).toLocaleString("zh-CN", {
    hour12: false,
  });

  const hc = data.humanCost;
  const timeline = data.timeline ?? [];

  return (
    <>
      {/* 成本概览 */}
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
                日成本（当前阶段）：${usdFormatter.format(data.perDay)}/day
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

      {/* Operation Timeline & Discrete Costs */}
      {timeline.length > 0 && (
        <section className="section">
          <h2 className="section-title">Operation Timeline & Discrete Costs</h2>
          <div className="iranwar-timeline">
            <table className="iranwar-timeline-table">
              <thead>
                <tr>
                  <th className="iranwar-tl-time">时间</th>
                  <th className="iranwar-tl-event">事件</th>
                  <th className="iranwar-tl-cost">成本</th>
                </tr>
              </thead>
              <tbody>
                {timeline.map((ev, idx) => (
                  <tr key={idx}>
                    <td className="iranwar-tl-time">{ev.time}</td>
                    <td className="iranwar-tl-event">{ev.title}</td>
                    <td className="iranwar-tl-cost">
                      {ev.cost ? (
                        <span className="iranwar-tl-cost-val">{ev.cost}</span>
                      ) : (
                        <span className="iranwar-tl-cost-na">--</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
              <tfoot>
                <tr className="iranwar-tl-total-row">
                  <td colSpan={2} className="iranwar-tl-total-label">
                    TOTAL DISCRETE COSTS
                  </td>
                  <td className="iranwar-tl-cost">
                    <span className="iranwar-tl-cost-val">{discreteDisplay}</span>
                  </td>
                </tr>
              </tfoot>
            </table>
          </div>
        </section>
      )}

      {/* Human Cost */}
      <section className="section">
        <h2 className="section-title">Human Cost · 人员伤亡</h2>
        <div className="iranwar-human-grid">
          <div className="iranwar-human-card">
            <div className="iranwar-human-title">United States</div>
            <div className="iranwar-human-row">
              <span>Service members killed</span>
              <span className="iranwar-human-number iranwar-human-number-critical">
                {hc?.usServiceMembersKilled || "—"}
              </span>
            </div>
            <div className="iranwar-human-row">
              <span>Wounded</span>
              <span className="iranwar-human-number">
                {hc?.usWounded || "—"}
              </span>
            </div>
            {hc?.usIncidents && hc.usIncidents.length > 0 && (
              <div className="iranwar-human-incidents">
                {hc.usIncidents.map((inc, i) => (
                  <div key={i} className="iranwar-human-incident">
                    <span className="iranwar-hi-date">{inc.date}</span>
                    <span className="iranwar-hi-desc">{inc.description}</span>
                    <span className="iranwar-hi-count">{inc.count}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
          <div className="iranwar-human-card">
            <div className="iranwar-human-title">Iran</div>
            <div className="iranwar-human-row">
              <span>Military casualties (est.)</span>
              <span className="iranwar-human-number">
                {hc?.iranMilitaryCasualties || "—"}
              </span>
            </div>
            <div className="iranwar-human-row">
              <span>Civilian casualties (est.)</span>
              <span className="iranwar-human-number iranwar-human-number-critical">
                {hc?.iranCivilianCasualties || "—"}
              </span>
            </div>
            {hc?.iranIncidents && hc.iranIncidents.length > 0 && (
              <div className="iranwar-human-incidents">
                {hc.iranIncidents.map((inc, i) => (
                  <div key={i} className="iranwar-human-incident">
                    <span className="iranwar-hi-date">{inc.date}</span>
                    <span className="iranwar-hi-desc">{inc.description}</span>
                    <span className="iranwar-hi-count">{inc.count}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </section>
    </>
  );
};
