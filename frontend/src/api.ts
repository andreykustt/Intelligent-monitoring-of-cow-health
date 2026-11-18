import type { components } from "./api-schema";
export type Cow = components["schemas"]["Cow"];
export type Metric = components["schemas"]["Metric"];
export type Point = components["schemas"]["Point"];
export type Identity = components["schemas"]["Identity"];
export type RecordEntry = components["schemas"]["ManualRecord"];
export type Alert = components["schemas"]["Alert"];
export type Simulation = components["schemas"]["Simulation"];
export type SimConfig = components["schemas"]["SimulationConfig"];
export type User = components["schemas"]["User"];
export type Overview = components["schemas"]["Overview"];
export type Page<T> = {
  items: T[];
  page: number;
  page_size: number;
  total: number;
};
let csrf = "";
export function setCSRF(v: string) {
  csrf = v;
}
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}
export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
  idempotency?: string,
): Promise<T> {
  const response = await fetch("/api/v1" + path, {
    method,
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      ...(method === "GET" ? {} : { "X-CSRF-Token": csrf }),
      ...(idempotency ? { "Idempotency-Key": idempotency } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) {
    const data = await response.json().catch(() => null);
    if (response.status === 401)
      window.dispatchEvent(new Event("session-expired"));
    throw new ApiError(
      response.status,
      data?.error?.message || "Не удалось выполнить запрос",
    );
  }
  return response.json();
}
export const metricNames: Record<string, string> = {
  body_temperature_c: "Температура тела",
  heart_rate_bpm: "Пульс",
  respiratory_rate_bpm: "Частота дыхания",
  activity_index: "Активность",
  steps_per_min: "Шаги",
  rumen_ph: "pH рубца",
  rumination_min_1h: "Жвачка",
  feed_intake_kg_1h: "Потребление корма",
  water_intake_l_1h: "Потребление воды",
};
export const statusNames: Record<string, string> = {
  anomaly: "Аномалия",
  no_anomaly: "Нет аномалии",
  insufficient_data: "Недостаточно данных",
  open: "Открыто",
  acknowledged: "Подтверждено",
  resolved: "Закрыто",
  starting: "Запускается",
  running: "Работает",
  paused: "Приостановлено",
  completed: "Завершено",
  stopped: "Остановлено",
  failed: "Ошибка",
};
export function time(value?: string | null) {
  return value
    ? new Date(value).toLocaleString("ru-RU", { timeZoneName: "short" })
    : "Нет данных";
}

export type TimelineEntry = components["schemas"]["TimelineEntry"];
