import React, { useEffect, useState } from "react";
import { fetchIranWarCost } from "./api";
import type { IranWarCost } from "./types";

const usdFormatter = new Intl.NumberFormat("en-US", {
  maximumFractionDigits: 0,
});

const phaseLabel: Record<string, string> = {
  initial_strikes: "初始打击阶段",
  sustained_operations: "持续作战阶段",
  air_dominance_isr: "制空权 / 情报侦察监视阶段",
  unknown: "未知阶段",
};

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

  if (loading && !data) {
    return <div className="status">正在抓取伊朗战争成本数据（首次加载需约 10 秒）...</div>;
  }

  if (!loading && error && !data) {
    return <div className="status error">{error}</div>;
  }

  if (!data) {
    return <div className="status">暂无可用数据</div>;
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
        <h2 className="section-title">伊朗战争成本 · "史诗之怒"行动</h2>
        <div className="iranwar-layout">
          <div className="iranwar-main-card">
            <div className="iranwar-main-header">
              <span className="iranwar-main-title">
                "史诗之怒"行动 — 自开战以来美国估算总成本
              </span>
              <a
                href="https://iran-cost-ticker.com/"
                target="_blank"
                rel="noreferrer"
                className="iranwar-source-link"
              >
                数据来源 →
              </a>
            </div>
            <div className="iranwar-main-total">{totalDisplay}</div>
            <div className="iranwar-main-sub">
              <span className="iranwar-chip">
                日成本（当前阶段）：${usdFormatter.format(data.perDay)}/天
              </span>
              <span className="iranwar-chip">
                持续作战成本：${usdFormatter.format(data.opsTotal)}
              </span>
              <span className="iranwar-chip">
                离散事件成本：{discreteDisplay}
              </span>
            </div>
            <div className="iranwar-main-footer">
              <span>当前阶段：{phaseLabel[data.phase] ?? data.phase}</span>
              <span>数据采集时间：{fetchedAtLocal}</span>
            </div>
          </div>

          <div className="iranwar-rate-grid">
            <div className="iranwar-rate-card">
              <div className="iranwar-rate-label">每秒成本</div>
              <div className="iranwar-rate-value">{perSecondDisplay}</div>
              <div className="iranwar-rate-tag">持续作战阶段</div>
            </div>
            <div className="iranwar-rate-card">
              <div className="iranwar-rate-label">每小时成本</div>
              <div className="iranwar-rate-value">{perHourDisplay}</div>
              <div className="iranwar-rate-tag">持续作战阶段</div>
            </div>
            <div className="iranwar-rate-card">
              <div className="iranwar-rate-label">每日成本</div>
              <div className="iranwar-rate-value">{perDayDisplay}</div>
              <div className="iranwar-rate-tag">持续作战阶段</div>
            </div>
          </div>
        </div>
      </section>

      {/* 作战时间线与离散事件成本 */}
      {timeline.length > 0 && (
        <section className="section">
          <h2 className="section-title">作战时间线与离散事件成本</h2>
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
                    离散事件成本合计
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

      {/* 人员伤亡 */}
      <section className="section">
        <h2 className="section-title">人员伤亡</h2>
        <div className="iranwar-human-grid">
          <div className="iranwar-human-card">
            <div className="iranwar-human-title">美国</div>
            <div className="iranwar-human-row">
              <span>军人阵亡</span>
              <span className="iranwar-human-number iranwar-human-number-critical">
                {hc?.usServiceMembersKilled || "—"}
              </span>
            </div>
            <div className="iranwar-human-row">
              <span>受伤</span>
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
            <div className="iranwar-human-title">伊朗</div>
            <div className="iranwar-human-row">
              <span>军事人员伤亡（估计）</span>
              <span className="iranwar-human-number">
                {hc?.iranMilitaryCasualties || "—"}
              </span>
            </div>
            <div className="iranwar-human-row">
              <span>平民伤亡（估计）</span>
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
