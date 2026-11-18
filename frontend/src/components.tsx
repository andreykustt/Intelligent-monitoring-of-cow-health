import { useEffect, useRef } from "react";
import * as echarts from "echarts/core";
import { LineChart } from "echarts/charts";
import { GridComponent, TooltipComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
echarts.use([LineChart, GridComponent, TooltipComponent, CanvasRenderer]);
import type { Point } from "./api";
import { statusNames } from "./api";
export function Badge({ status }: { status: string }) {
  return (
    <span className={"badge " + status}>{statusNames[status] || status}</span>
  );
}
export function ErrorBox({ error }: { error: unknown }) {
  return error ? (
    <div className="error" role="alert">
      {error instanceof Error ? error.message : "Ошибка загрузки"}
    </div>
  ) : null;
}
export function Empty({
  children = "Данных пока нет",
}: {
  children?: React.ReactNode;
}) {
  return (
    <div className="empty">
      <span className="empty-icon">◌</span>
      <h3>{children}</h3>
      <p>Здесь появятся сохранённые данные выбранного периода.</p>
    </div>
  );
}
export function Pager({
  page,
  total,
  onChange,
}: {
  page: number;
  total: number;
  onChange: (v: number) => void;
}) {
  return (
    <div className="pager">
      <span>Всего записей: {total}</span>
      <button disabled={page <= 1} onClick={() => onChange(page - 1)}>
        Назад
      </button>
      <span>Страница {page}</span>
      <button disabled={page * 25 >= total} onClick={() => onChange(page + 1)}>
        Далее
      </button>
    </div>
  );
}
export function Chart({ points, unit }: { points: Point[]; unit: string }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!ref.current) return;
    const chart = echarts.init(ref.current);
    chart.setOption({
      color: ["#258364"],
      grid: { left: 55, right: 25, top: 30, bottom: 45 },
      tooltip: { trigger: "axis" },
      xAxis: { type: "time", axisLabel: { hideOverlap: true } },
      yAxis: {
        type: "value",
        scale: true,
        name: unit,
        splitLine: { lineStyle: { color: "#e8eeeb" } },
      },
      series: [
        {
          type: "line",
          smooth: false,
          showSymbol: false,
          connectNulls: false,
          areaStyle: { opacity: 0.08 },
          data: points.map((p) => [p.recorded_at, p.value]),
        },
      ],
    });
    const observer = new ResizeObserver(() => chart.resize());
    observer.observe(ref.current);
    return () => {
      observer.disconnect();
      chart.dispose();
    };
  }, [points, unit]);
  return (
    <div
      ref={ref}
      className="chart"
      role="img"
      aria-label={"История показателя, " + unit}
    />
  );
}
