export interface NewsItem {
  id: string;
  title: string;
  url: string;
  source: string;
  description?: string;
  publishedAt: string;
  publishedDate?: string;
  hotScore: number;
  extraData?: Record<string, unknown>;
}

export interface ApiResponse<T> {
  code: string;
  message: string;
  data: T;
}

