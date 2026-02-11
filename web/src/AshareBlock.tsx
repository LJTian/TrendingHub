import React from "react";
import type { NewsItem } from "./types";

interface Props {
  items: NewsItem[];
}

export const AshareBlock: React.FC<Props> = ({ items }) => {
  return (
    <div className="ashare-wrapper">
      <h3 className="ashare-title">A 股指数</h3>
      {items.length === 0 ? (
        <div className="ashare-empty">暂无 A 股指数数据，请稍后刷新或检查网络。</div>
      ) : (
      <div className="ashare-grid">
        {items.map((item) => (
          <a
            key={item.id}
            href={item.url}
            target="_blank"
            rel="noreferrer"
            className="ashare-card"
          >
            <div className="ashare-name">{item.title}</div>
            <div className="ashare-price">{item.hotScore.toFixed(2)}</div>
            {item.summary && item.summary !== item.title && (
              <div className="ashare-meta">{item.summary}</div>
            )}
          </a>
        ))}
      </div>
      )}
    </div>
  );
};
