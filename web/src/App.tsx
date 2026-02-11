import React, { useEffect, useState } from "react";
import { fetchNews, fetchNewsDates } from "./api";
import type { NewsItem } from "./types";
import { GoldChart } from "./GoldChart";
import { AshareBlock } from "./AshareBlock";

const CHANNELS = [
  { code: "", label: "全部" },
  { code: "github", label: "GitHub Trending" },
  { code: "baidu", label: "百度热搜" },
  { code: "gold", label: "金融" }
];

type SortOption = "latest" | "hot";

/** 东八区今天的日期 YYYY-MM-DD，与后端一致 */
function todayEast8(): string {
  return new Date()
    .toLocaleDateString("sv-SE", { timeZone: "Asia/Shanghai" });
}

function formatDateLabel(d: string) {
  if (d === todayEast8()) return "今天";
  const [, m, day] = d.split("-");
  return `${m}/${day}`;
}

export const App: React.FC = () => {
  const [channel, setChannel] = useState<string>("");
  const [sort, setSort] = useState<SortOption>("hot");
  const [date, setDate] = useState<string>("");
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
        sort,
        limit: isGold ? 100 : 30,
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
  }, [channel, sort, date]);

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
      <header className="header">
        <div>
          <h1>TrendingHub</h1>
          <p className="subtitle">一站式查看多源热点（GitHub / 百度 / 金融等）</p>
        </div>
        <div className="controls">
          <select
            value={channel}
            onChange={(e) => setChannel(e.target.value)}
          >
            {CHANNELS.map((c) => (
              <option key={c.code || "all"} value={c.code}>
                {c.label}
              </option>
            ))}
          </select>
          <select
            value={date}
            onChange={(e) => setDate(e.target.value)}
            title="按日期展示"
          >
            <option value="">全部日期</option>
            {dates.map((d) => (
              <option key={d} value={d}>
                {formatDateLabel(d)}
              </option>
            ))}
          </select>
          <div className="segmented">
            <button
              className={sort === "latest" ? "active" : ""}
              onClick={() => setSort("latest")}
            >
              最新
            </button>
            <button
              className={sort === "hot" ? "active" : ""}
              onClick={() => setSort("hot")}
            >
              热度
            </button>
          </div>
          <button className="refresh" onClick={() => void load()}>
            手动刷新
          </button>
        </div>
      </header>

      <main className="main">
        {loading && <div className="status">加载中...</div>}
        {error && !loading && <div className="status error">{error}</div>}
        {!loading && !error && items.length === 0 && (
          <div className="status">暂无数据</div>
        )}

        {channel === "gold" ? (
          <>
            <GoldChart items={items.filter((i) => i.source === "gold")} />
            <AshareBlock items={items.filter((i) => i.source === "ashare")} />
          </>
        ) : (
          <ul className="list">
            {items.map((item, index) => (
              <li
                key={item.id}
                className="card"
                title={item.summary || item.title}
              >
                <div className="card-header">
                  <span className="rank">{index + 1}</span>
                  <a
                    href={item.url}
                    target="_blank"
                    rel="noreferrer"
                    className="title"
                    title={item.summary || item.title}
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
                </div>
                {(item.summary && item.summary !== item.title) && (
                  <p className="summary">{item.summary}</p>
                )}
                <div className="card-tooltip" role="tooltip">
                  <div className="card-tooltip-title">详细信息</div>
                  <div className="card-tooltip-body">
                    {item.description || item.summary || item.title || "暂无介绍"}
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </main>
    </div>
  );
};

