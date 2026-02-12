import React, { useMemo } from "react";

const WEEKDAYS = ["日", "一", "二", "三", "四", "五", "六"];

/** 东八区今天的 YYYY-MM-DD */
function todayEast8(): string {
  return new Date()
    .toLocaleDateString("sv-SE", { timeZone: "Asia/Shanghai" });
}

/** 解析 YYYY-MM-DD 为东八区该日 0 点的 Date（用于当月日历） */
function parseLocalDate(dateStr: string): Date {
  const [y, m, d] = dateStr.split("-").map(Number);
  return new Date(y, m - 1, d);
}

interface CalendarProps {
  value: string;
  onChange: (date: string) => void;
  availableDates?: string[];
}

export const Calendar: React.FC<CalendarProps> = ({
  value,
  onChange,
  availableDates = []
}) => {
  const viewMonth = value ? parseLocalDate(value) : new Date();
  const viewYear = viewMonth.getFullYear();
  const viewMonthIdx = viewMonth.getMonth();

  const { firstDay, daysInMonth, padding } = useMemo(() => {
    const first = new Date(viewYear, viewMonthIdx, 1);
    const last = new Date(viewYear, viewMonthIdx + 1, 0);
    const firstDay = first.getDay();
    const daysInMonth = last.getDate();
    const padding = firstDay;
    return { firstDay, daysInMonth, padding };
  }, [viewYear, viewMonthIdx]);

  const prevMonth = () => {
    const d = new Date(viewYear, viewMonthIdx - 1, 1);
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, "0");
    onChange(`${y}-${m}-01`);
  };

  const nextMonth = () => {
    const d = new Date(viewYear, viewMonthIdx + 1, 1);
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, "0");
    onChange(`${y}-${m}-01`);
  };

  const today = todayEast8();
  const canPrev = viewYear > 2020 || viewMonthIdx > 0;
  const canNext = viewYear < 2030 || viewMonthIdx < 11;

  const cells: (string | null)[] = [];
  for (let i = 0; i < padding; i++) cells.push(null);
  for (let d = 1; d <= daysInMonth; d++) {
    const dateStr = `${viewYear}-${String(viewMonthIdx + 1).padStart(2, "0")}-${String(d).padStart(2, "0")}`;
    cells.push(dateStr);
  }

  return (
    <div className="calendar">
      <div className="calendar-header">
        <button
          type="button"
          className="calendar-nav"
          onClick={prevMonth}
          disabled={!canPrev}
          aria-label="上月"
        >
          ‹
        </button>
        <span className="calendar-title">
          {viewYear}年{viewMonthIdx + 1}月
        </span>
        <button
          type="button"
          className="calendar-nav"
          onClick={nextMonth}
          disabled={!canNext}
          aria-label="下月"
        >
          ›
        </button>
      </div>
      <button
        type="button"
        className="calendar-all-dates"
        onClick={() => onChange("")}
      >
        全部日期
      </button>
      <table className="calendar-table" role="grid">
        <thead>
          <tr>
            {WEEKDAYS.map((w) => (
              <th key={w} scope="col">
                {w}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {(() => {
            const rows: React.ReactNode[] = [];
            let row: React.ReactNode[] = [];
            cells.forEach((cell, i) => {
              row.push(
                <td key={i}>
                  {cell ? (() => {
                    const noData = availableDates.length > 0 && !availableDates.includes(cell);
                    return (
                      <button
                        type="button"
                        className={`calendar-day ${value === cell ? "selected" : ""} ${cell === today ? "today" : ""} ${noData ? "no-data" : ""}`}
                        onClick={() => !noData && onChange(cell)}
                        disabled={noData}
                        title={noData ? "该日期暂无数据" : undefined}
                      >
                        {parseLocalDate(cell).getDate()}
                      </button>
                    );
                  })() : null}
                </td>
              );
              if (row.length === 7) {
                rows.push(<tr key={rows.length}>{row}</tr>);
                row = [];
              }
            });
            if (row.length > 0) {
              while (row.length < 7) row.push(<td key={`pad-${row.length}`} />);
              rows.push(<tr key={rows.length}>{row}</tr>);
            }
            return rows;
          })()}
        </tbody>
      </table>
    </div>
  );
};
