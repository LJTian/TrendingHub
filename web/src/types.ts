export interface NewsItem {
  id: string;
  title: string;
  url: string;
  source: string;
  summary: string;
  description?: string;
  publishedAt: string;
  publishedDate?: string; // 日期 YYYY-MM-DD，用于按日期展示
  hotScore: number;
}

export interface ApiResponse<T> {
  code: string;
  message: string;
  data: T;
}

