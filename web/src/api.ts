import type { ApiResponse, NewsItem } from "./types";

const BASE_URL = "";

export async function fetchNews(params: {
  channel?: string;
  sort?: "latest" | "hot";
  limit?: number;
  date?: string; // 可选，YYYY-MM-DD，按日期展示
}): Promise<NewsItem[]> {
  const search = new URLSearchParams();
  if (params.channel) search.set("channel", params.channel);
  if (params.sort) search.set("sort", params.sort);
  if (params.limit) search.set("limit", String(params.limit));
  if (params.date) search.set("date", params.date);

  const res = await fetch(`${BASE_URL}/api/v1/news?${search.toString()}`);
  const contentType = res.headers.get("content-type") ?? "";
  const text = await res.text();

  if (!contentType.includes("application/json")) {
    if (text.trimStart().toLowerCase().startsWith("<!doctype") || text.trimStart().startsWith("<html")) {
      throw new Error("后端未启动或未响应，请先运行 cmd/api 下的 Go 服务（默认端口 9000）");
    }
    throw new Error(`请求失败: ${res.status}，返回非 JSON`);
  }
  if (!res.ok) {
    throw new Error(`请求失败: ${res.status}`);
  }
  let json: ApiResponse<NewsItem[]>;
  try {
    json = JSON.parse(text) as ApiResponse<NewsItem[]>;
  } catch {
    throw new Error("后端未启动或未响应，请先运行 cmd/api 下的 Go 服务（默认端口 9000）");
  }
  if (json.code !== "ok") {
    throw new Error(json.message || "接口返回错误");
  }
  if (json.data == null) {
    return [];
  }
  return json.data;
}

/** 获取有数据的日期列表（倒序），用于按日期展示 */
export async function fetchNewsDates(params: {
  channel?: string;
  limit?: number;
}): Promise<string[]> {
  const search = new URLSearchParams();
  if (params.channel) search.set("channel", params.channel);
  if (params.limit) search.set("limit", String(params.limit ?? 31));

  const res = await fetch(`${BASE_URL}/api/v1/news/dates?${search.toString()}`);
  const contentType = res.headers.get("content-type") ?? "";
  const text = await res.text();

  if (!contentType.includes("application/json") || !res.ok) {
    return [];
  }
  let json: ApiResponse<string[]>;
  try {
    json = JSON.parse(text) as ApiResponse<string[]>;
  } catch {
    return [];
  }
  if (json.code !== "ok" || json.data == null) {
    return [];
  }
  return json.data;
}

