import React, { useEffect, useState, useMemo } from "react";
import { fetchNews, fetchNewsDates } from "./api";
import type { NewsItem } from "./types";
import { GoldChart } from "./GoldChart";
import { AshareBlock } from "./AshareBlock";
import { Calendar } from "./Calendar";
import { WeatherCard } from "./WeatherCard";

const CHANNELS = [
  { code: "github", label: "GitHub Trending", sources: ["github"] },
  { code: "baidu", label: "百度热搜", sources: ["baidu"] },
  { code: "hackernews", label: "Hacker News", sources: ["hackernews"] },
  { code: "gold", label: "金融", sources: ["gold", "ashare"] }
];

const HOME_PREVIEW_COUNT = 5;
const GRAMS_PER_OUNCE = 31.1034768;

/** 东八区今天的日期 YYYY-MM-DD，与后端一致 */
function todayEast8(): string {
  return new Date().toLocaleDateString("sv-SE", {
    timeZone: "Asia/Shanghai"
  });
}

/** 将文本按字符数截断（按 Unicode 字符计算，避免中文被截断成半个字） */
function truncateText(text: string | undefined, limit: number): string {
  if (!text) return "";
  const chars = Array.from(text);
  if (chars.length <= limit) return text;
  return chars.slice(0, limit).join("") + "…";
}

export const App: React.FC = () => {
  const [channel, setChannel] = useState<string>("");
  const [date, setDate] = useState<string>(() => todayEast8());
  const [dates, setDates] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [items, setItems] = useState<NewsItem[]>([]);

  const isHome = channel === "";

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      if (isHome) {
        const results = await Promise.all(
          CHANNELS.map((ch) =>
            fetchNews({
              channel: ch.code,
              sort: "hot",
              limit: ch.code === "gold" ? 500 : HOME_PREVIEW_COUNT * 2,
              date: date || undefined
            })
          )
        );
        setItems(results.flat());
      } else {
        const isGold = channel === "gold";
        const data = await fetchNews({
          channel,
          sort: "hot",
          limit: isGold ? 500 : 30,
          date: date || undefined
        });
        setItems(data);
      }
    } catch (e: any) {
      setError(e.message || "未知错误");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [channel, date]);

  useEffect(() => {
    fetchNewsDates({ channel: channel || undefined, limit: 31 })
      .then((list) => {
        const today = todayEast8();
        if (list.length === 0) {
          setDates([today]);
        } else {
          setDates(list.includes(today) ? list : [today, ...list]);
        }
      })
      .catch(() => {
        setDates([todayEast8()]);
      });
  }, [channel]);

  const channelGroups = useMemo(() => {
    if (!isHome) return [];
    return CHANNELS.map((ch) => ({
      ...ch,
      items: items.filter((item) => ch.sources.includes(item.source))
    }));
  }, [items, isHome]);

  const renderFinancePreview = (financeItems: NewsItem[]) => {
    const goldItems = financeItems.filter((i) => i.source === "gold");
    const ashareItems = financeItems.filter((i) => i.source === "ashare");

    const sorted = [...goldItems].sort(
      (a, b) => new Date(a.publishedAt).getTime() - new Date(b.publishedAt).getTime()
    );
    const latestGold = sorted.length > 0 ? sorted[sorted.length - 1] : null;

    const ashareLatest = new Map<string, NewsItem>();
    for (const item of ashareItems) {
      const existing = ashareLatest.get(item.title);
      if (
        !existing ||
        new Date(item.publishedAt) > new Date(existing.publishedAt)
      ) {
        ashareLatest.set(item.title, item);
      }
    }

    if (!latestGold && ashareLatest.size === 0) {
      return <div className="home-card-empty">暂无数据</div>;
    }

    const toPerGram = (v: number) => (v > 10000 ? v / GRAMS_PER_OUNCE : v);

    return (
      <div className="home-finance">
        {latestGold && (
          <div className="home-finance-row">
            <span className="home-finance-label">黄金（元/克）</span>
            <span className="home-finance-value gold">
              ¥{toPerGram(latestGold.hotScore).toFixed(2)}
            </span>
          </div>
        )}
        {Array.from(ashareLatest.values())
          .slice(0, 4)
          .map((item) => {
            const change = item.extraData?.change as string | undefined;
            const isUp = change ? !change.startsWith("-") : true;
            return (
              <div key={item.id} className="home-finance-row">
                <span className="home-finance-label">{item.title}</span>
                <span className={`home-finance-value ${isUp ? "up" : "down"}`}>
                  {item.hotScore.toFixed(2)}
                </span>
                {change && (
                  <span
                    className={`home-finance-change ${isUp ? "up" : "down"}`}
                  >
                    {isUp && "+"}
                    {change}%
                  </span>
                )}
              </div>
            );
          })}
      </div>
    );
  };

  const renderHome = () => (
    <div className="home">
      <h2 className="home-title">今日热点总览</h2>
      <WeatherCard />
      <div className="home-grid">
        {channelGroups.map((group) => (
          <div key={group.code} className="home-card">
            <div className="home-card-header">
              <h3 className="home-card-name">{group.label}</h3>
              <button
                type="button"
                className="home-card-more"
                onClick={() => setChannel(group.code)}
              >
                查看更多 →
              </button>
            </div>
            {group.code === "gold" ? (
              renderFinancePreview(group.items)
            ) : (
              <ul className="home-card-list">
                {group.items
                  .slice(0, HOME_PREVIEW_COUNT)
                  .map((item, idx) => (
                    <li key={item.id} className="home-card-item">
                      <span className="home-card-rank">{idx + 1}</span>
                      <a
                        href={item.url}
                        target="_blank"
                        rel="noreferrer"
                        className="home-card-link"
                      >
                        {truncateText(item.title, 50)}
                      </a>
                      {item.source === "github" && (
                        <span className="home-card-stars">
                          ★ {Math.round(item.hotScore).toLocaleString()}
                        </span>
                      )}
                    </li>
                  ))}
                {group.items.length === 0 && (
                  <li className="home-card-empty">暂无数据</li>
                )}
              </ul>
            )}
          </div>
        ))}
      </div>
    </div>
  );

  return (
    <div className="page">
      <aside className="sidebar">
        <div
          className="site-title"
          onClick={() => setChannel("")}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => e.key === "Enter" && setChannel("")}
        >
          <h1>TrendingHub</h1>
          <p className="subtitle">多源热点聚合</p>
        </div>

        <nav className="sidebar-nav">
          <span className="sidebar-label">频道</span>
          <ul>
            {CHANNELS.map((c) => (
              <li key={c.code}>
                <button
                  type="button"
                  className={channel === c.code ? "active" : ""}
                  onClick={() => setChannel(c.code)}
                >
                  {c.label}
                </button>
              </li>
            ))}
          </ul>
        </nav>

        <div className="sidebar-block sidebar-block-calendar">
          <span className="sidebar-label">日期</span>
          <Calendar
            value={date}
            onChange={setDate}
            availableDates={dates}
          />
        </div>
        <button type="button" className="refresh" onClick={() => void load()}>
          手动刷新
        </button>
      </aside>

      <div className="content">
        <main className="main">
          {loading && <div className="status">加载中...</div>}
          {error && !loading && <div className="status error">{error}</div>}
          {!loading && !error && items.length === 0 && (
            <div className="status">暂无数据</div>
          )}

          {isHome ? (
            !loading && !error && items.length > 0 && renderHome()
          ) : channel === "gold" ? (
            <>
              <section className="section">
                <h2 className="section-title">黄金 · 当日走势</h2>
                <GoldChart items={items.filter((i) => i.source === "gold")} />
              </section>
              <section className="section">
                <AshareBlock
                  items={items.filter((i) => i.source === "ashare")}
                />
              </section>
            </>
          ) : (
            <section className="section">
              <h2 className="section-title">
                {CHANNELS.find((c) => c.code === channel)?.label ?? "今日热点"}
              </h2>
              <ul className="list">
                {items.map((item, index) => (
                  <li key={item.id} className="card">
                    <div className="card-header">
                      <span className="rank">{index + 1}</span>
                      <a
                        href={item.url}
                        target="_blank"
                        rel="noreferrer"
                        className="title"
                      >
                        {item.title}
                      </a>
                    </div>
                    <div className="card-meta">
                      <span className="badge">{item.source}</span>
                      <span className="dot" />
                      <span>热度 {Math.round(item.hotScore)}</span>
                      <span className="dot" />
                      <span>
                        {new Date(item.publishedAt).toLocaleString("zh-CN", {
                          hour12: false
                        })}
                      </span>
                      {item.source === "github" && (
                        <>
                          <span className="dot" />
                          <span className="card-stars" title="Stars">
                            ★ {Math.round(item.hotScore).toLocaleString()}
                          </span>
                        </>
                      )}
                      {item.source === "hackernews" && item.extraData && (
                        <>
                          <span className="dot" />
                          <span className="card-comments" title="Comments">
                            💬 {Number(item.extraData.comments ?? 0)}
                          </span>
                        </>
                      )}
                    </div>
                    <div className="card-tooltip" role="tooltip">
                      <div className="card-tooltip-title">详细信息</div>
                      <div className="card-tooltip-body">
                        <div className="card-tooltip-main">
                          {truncateText(
                            item.description || item.title || "暂无介绍",
                            600
                          )}
                        </div>
                        <div className="card-tooltip-extra">
                          <p>
                            来源：{item.source} · 热度：
                            {Math.round(item.hotScore)} · 发布时间：
                            {new Date(item.publishedAt).toLocaleString(
                              "zh-CN",
                              { hour12: false }
                            )}
                          </p>
                        </div>
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            </section>
          )}
        </main>
      </div>
    </div>
  );
};
