import React, { useEffect, useState, useCallback } from "react";
import { fetchAllWeather, addWeatherCity, removeWeatherCity } from "./api";
import type { WeatherItem, WttrResponse } from "./types";

const WEATHER_ICONS: Record<string, string> = {
  // 原 wttr.in 常用编码
  "113": "☀️",
  "116": "⛅",
  "119": "☁️",
  "122": "☁️",
  "143": "🌫️",
  "176": "🌦️",
  "179": "🌨️",
  "182": "🌨️",
  "185": "🌨️",
  "200": "⛈️",
  "227": "🌨️",
  "230": "❄️",
  "248": "🌫️",
  "260": "🌫️",
  "263": "🌦️",
  "266": "🌧️",
  "281": "🌧️",
  "284": "🌧️",
  "293": "🌦️",
  "296": "🌧️",
  "299": "🌧️",
  "302": "🌧️",
  "305": "🌧️",
  "308": "🌧️",
  "311": "🌧️",
  "314": "🌧️",
  "317": "🌨️",
  "320": "🌨️",
  "323": "🌨️",
  "326": "🌨️",
  "329": "❄️",
  "332": "❄️",
  "335": "❄️",
  "338": "❄️",
  "350": "🌨️",
  "353": "🌦️",
  "356": "🌧️",
  "359": "🌧️",
  "362": "🌨️",
  "365": "🌨️",
  "368": "🌨️",
  "371": "❄️",
  "374": "🌨️",
  "377": "🌨️",
  "386": "⛈️",
  "389": "⛈️",
  "392": "⛈️",
  "395": "❄️",
  // QWeather 常用图标编码
  "100": "☀️", // 晴（白天）
  "101": "⛅", // 多云
  "102": "⛅",
  "103": "⛅",
  "104": "☁️", // 阴
  "150": "🌙", // 晴（夜间）
  "151": "☁️",
  "300": "🌦️",
  "301": "🌧️",
  "303": "⛈️",
  "304": "⛈️",
  "306": "🌧️",
  "307": "🌧️",
  "400": "🌨️",
  "401": "❄️",
  "402": "❄️",
  "403": "❄️",
  "404": "🌨️",
  "500": "🌫️",
  "501": "🌫️",
  "502": "🌫️",
  "503": "🌫️",
};

function icon(code: string): string {
  // 优先支持 Open-Meteo 的 WMO weathercode（数值）
  const n = Number.parseInt(code, 10);
  if (!Number.isNaN(n)) {
    if (n === 0) return "☀️"; // 晴
    if (n === 1 || n === 2) return "⛅"; // 少云 / 多云
    if (n === 3) return "☁️"; // 阴
    if (n === 45 || n === 48) return "🌫️"; // 雾
    if (n >= 51 && n <= 57) return "🌦️"; // 毛毛雨
    if (n >= 61 && n <= 67) return "🌧️"; // 雨
    if (n >= 71 && n <= 77) return "🌨️"; // 雪
    if (n >= 80 && n <= 82) return "🌧️"; // 阵雨
    if (n >= 95 && n <= 99) return "⛈️"; // 雷暴
  }

  // 兼容老的 wttr.in / QWeather 编码
  return WEATHER_ICONS[code] ?? "🌡️";
}

function getDayOfWeek(dateStr: string): string {
  const d = new Date(dateStr + "T00:00:00");
  return ["周日", "周一", "周二", "周三", "周四", "周五", "周六"][d.getDay()];
}

function getMidDayWeather(day: WttrResponse["weather"][0]) {
  const noon = day.hourly.find((h) => h.time === "1200") ?? day.hourly[0];
  if (!noon) return { desc: "—", code: "113" };
  return { desc: noon.weatherDesc[0]?.value?.trim() ?? "—", code: noon.weatherCode };
}

export const WeatherCard: React.FC = () => {
  const [items, setItems] = useState<WeatherItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeCity, setActiveCity] = useState<string>("");
  const [input, setInput] = useState("");
  const [adding, setAdding] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await fetchAllWeather();
      setItems(data);
      if (data.length > 0 && !data.find((d) => d.city === activeCity)) {
        setActiveCity(data[0].city);
      }
    } finally {
      setLoading(false);
    }
  }, [activeCity]);

  useEffect(() => { void load(); }, [load]);

  const handleAdd = async () => {
    const city = input.trim();
    if (!city) return;
    setAdding(true);
    try {
      await addWeatherCity(city);
      setInput("");
      setTimeout(() => {
        void load().then(() => setActiveCity(city));
      }, 2000);
    } finally {
      setAdding(false);
    }
  };

  const handleRemove = async (city: string) => {
    await removeWeatherCity(city);
    setItems((prev) => {
      const next = prev.filter((i) => i.city !== city);
      if (activeCity === city && next.length > 0) {
        setActiveCity(next[0].city);
      }
      return next;
    });
  };

  if (loading && items.length === 0) {
    return <div className="weather-card weather-card--loading">加载天气中...</div>;
  }

  const active = items.find((i) => i.city === activeCity);
  const cur = active?.weather?.current_condition?.[0];

  return (
    <div className="weather-card">
      {/* 标签栏 */}
      <div className="weather-tabs">
        <div className="weather-tabs-list">
          {items.map((item) => (
            <button
              key={item.city}
              type="button"
              className={`weather-tab ${item.city === activeCity ? "active" : ""}`}
              onClick={() => setActiveCity(item.city)}
            >
              {item.city}
              <span
                className="weather-tab-close"
                onClick={(e) => { e.stopPropagation(); handleRemove(item.city); }}
                title="移除"
              >
                ×
              </span>
            </button>
          ))}
        </div>
        <div className="weather-add-bar">
          <input
            className="weather-city-input"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleAdd()}
            placeholder="添加城市"
          />
          <button
            type="button"
            className="weather-add-btn"
            onClick={handleAdd}
            disabled={adding || !input.trim()}
          >
            {adding ? "..." : "+"}
          </button>
        </div>
      </div>

      {/* 内容区 */}
      {!active || !cur ? (
        <div className="weather-empty">暂无天气数据</div>
      ) : (
        <div className="weather-panel">
          <div className="weather-current">
            <span className="weather-current-icon">{icon(cur.weatherCode)}</span>
            <span className="weather-current-temp">{cur.temp_C}°</span>
            <div className="weather-current-info">
              <span className="weather-current-desc">
                {cur.weatherDesc[0]?.value?.trim()}
              </span>
              <span className="weather-current-meta">
                体感{cur.FeelsLikeC}° 湿度{cur.humidity}% 风{cur.winddir16Point} {cur.windspeedKmph}km/h
              </span>
            </div>
          </div>
          <div className="weather-forecast">
            {active.weather.weather.map((day) => {
              const m = getMidDayWeather(day);
              return (
                <div key={day.date} className="weather-forecast-day">
                  <span className="weather-forecast-date">{getDayOfWeek(day.date)}</span>
                  <span className="weather-forecast-icon">{icon(m.code)}</span>
                  <span className="weather-forecast-temp">
                    {day.mintempC}°/{day.maxtempC}°
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
};
