import React, { useEffect, useState } from "react";
import { fetchNews, fetchNewsDates } from "./api";
import type { NewsItem } from "./types";
import { GoldChart } from "./GoldChart";
import { AshareBlock } from "./AshareBlock";
import { Calendar } from "./Calendar";

const CHANNELS = [
  { code: "", label: "首页" },
  { code: "github", label: "GitHub Trending" },
  { code: "baidu", label: "百度热搜" },
  { code: "gold", label: "金融" }
];

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

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const isGold = channel === "gold";
      const data = await fetchNews({
        channel: channel || undefined,
        sort: "hot",
        limit: isGold ? 500 : 30,
        date: date || undefined
      });
      setItems(data);
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
          // 始终在列表最前增加「今天」选项（若尚未包含）
          setDates(list.includes(today) ? list : [today, ...list]);
        }
      })
      .catch(() => {
        setDates([todayEast8()]);
      });
  }, [channel]);

  return (
    <div className="page">
      <aside className="sidebar">
        <div className="site-title">
          <h1>TrendingHub</h1>
          <p className="subtitle">多源热点聚合</p>
        </div>
        <nav className="sidebar-nav">
          <span className="sidebar-label">频道</span>
          <ul>
            {CHANNELS.map((c) => (
              <li key={c.code || "all"}>
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

        {channel === "gold" ? (
          <>
            <section className="section">
              <h2 className="section-title">黄金 · 当日走势</h2>
              <GoldChart items={items.filter((i) => i.source === "gold")} />
            </section>
            <section className="section">
              <AshareBlock items={items.filter((i) => i.source === "ashare")} />
            </section>
          </>
        ) : (
          <section className="section">
            <h2 className="section-title">
              {channel
                ? CHANNELS.find((c) => c.code === channel)?.label ?? "今日热点"
                : "今日热点"}
            </h2>
            <ul className="list">
            {items.map((item, index) => (
              <li
                key={item.id}
                className="card"
              >
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
                        {new Date(item.publishedAt).toLocaleString("zh-CN", {
                          hour12: false
                        })}
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

