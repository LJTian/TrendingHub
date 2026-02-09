import React, { useEffect, useState } from "react";
import { fetchNews } from "./api";
import type { NewsItem } from "./types";
import { GoldChart } from "./GoldChart";

const CHANNELS = [
  { code: "", label: "全部" },
  { code: "github", label: "GitHub Trending" },
  { code: "baidu", label: "百度热搜" },
  { code: "gold", label: "黄金价格" }
];

type SortOption = "latest" | "hot";

export const App: React.FC = () => {
  const [channel, setChannel] = useState<string>("");
  const [sort, setSort] = useState<SortOption>("hot");
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
        limit: isGold ? 500 : 30
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
  }, [channel, sort]);

  return (
    <div className="page">
      <header className="header">
        <div>
          <h1>TrendingHub</h1>
          <p className="subtitle">一站式查看多源热点（GitHub / 百度 / 黄金等）</p>
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
          <GoldChart items={items} />
        ) : (
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
                </div>
                {item.summary && item.summary !== item.title && (
                  <p className="summary">{item.summary}</p>
                )}
              </li>
            ))}
          </ul>
        )}
      </main>
    </div>
  );
};

