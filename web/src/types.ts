export interface NewsItem {
  id: string;
  title: string;
  url: string;
  source: string;
  summary: string;
  publishedAt: string;
  hotScore: number;
}

export interface ApiResponse<T> {
  code: string;
  message: string;
  data: T;
}

