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

export interface WttrCondition {
  temp_C: string;
  FeelsLikeC: string;
  humidity: string;
  weatherDesc: { value: string }[];
  weatherCode: string;
  windspeedKmph: string;
  winddir16Point: string;
  uvIndex: string;
}export interface WttrDay {
  date: string;
  maxtempC: string;
  mintempC: string;
  astronomy: { sunrise: string; sunset: string }[];
  hourly: {
    time: string;
    weatherCode: string;
    weatherDesc: { value: string }[];
  }[];
}

export interface WttrResponse {
  current_condition: WttrCondition[];
  nearest_area: { areaName: { value: string }[] }[];
  weather: WttrDay[];
}

export interface WeatherItem {
  city: string;
  fetchedAt: string;
  weather: WttrResponse;
}

export interface IranTimelineEvent {
  time: string;
  title: string;
  description?: string;
  cost: string;
}

export interface IranHumanIncident {
  date: string;
  description: string;
  count: string;
}

export interface IranHumanCost {
  usServiceMembersKilled: string;
  usWounded: string;
  iranMilitaryCasualties: string;
  iranCivilianCasualties: string;
  usIncidents?: IranHumanIncident[];
  iranIncidents?: IranHumanIncident[];
}

export interface IranWarCost {
  total: number;
  opsTotal: number;
  discreteTotal: number;
  perSecond: number;
  perHour: number;
  perDay: number;
  phase: string;
  currency: string;
  fetchedAt: string;
  sourceUrl: string;
  timeline?: IranTimelineEvent[];
  humanCost?: IranHumanCost;
}
