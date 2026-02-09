import type { ApiResponse, NewsItem } from "./types";

const BASE_URL = "";

export async function fetchNews(params: {
  channel?: string;
  sort?: "latest" | "hot";
  limit?: number;
}): Promise<NewsItem[]> {
  const search = new URLSearchParams();
  if (params.channel) search.set("channel", params.channel);
  if (params.sort) search.set("sort", params.sort);
  if (params.limit) search.set("limit", String(params.limit));

  const res = await fetch(`${BASE_URL}/api/v1/news?${search.toString()}`);
  if (!res.ok) {
    throw new Error(`请求失败: ${res.status}`);
  }
  const json = (await res.json()) as ApiResponse<NewsItem[]>;
  if (json.code !== "ok") {
    throw new Error(json.message || "接口返回错误");
  }
  return json.data;
}

