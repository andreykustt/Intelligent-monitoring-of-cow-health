import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { api, type RecordEntry } from "./api";
import { ErrorBox } from "./components";
export const kinds: Record<string, string> = {
  milkings: "Доение",
  inspections: "Визуальный осмотр",
  "laboratory-results": "Лабораторный результат",
  "body-condition-records": "Упитанность и копыта",
  "veterinary-examinations": "Ветеринарный осмотр",
};
type Field = {
  key: string;
  label: string;
  unit?: string;
  options?: string[];
  optional?: boolean;
};
const fields: Record<string, Field[]> = {
  milkings: [
    { key: "yield_l", label: "Объём молока", unit: "л" },
    { key: "flow_l_min", label: "Скорость потока", unit: "л/мин" },
    { key: "temperature_c", label: "Температура молока", unit: "°C" },
    ...["fl", "fr", "rl", "rr"].map((q, i) => ({
      key: "conductivity_" + q,
      label:
        "Проводимость: " +
        ["передняя левая", "передняя правая", "задняя левая", "задняя правая"][
          i
        ],
      unit: "мСм/см",
    })),
    { key: "weight_kg", label: "Масса животного", unit: "кг", optional: true },
  ],
  inspections: [
    {
      key: "appetite",
      label: "Аппетит",
      options: ["обычный", "снижен", "отсутствует"],
    },
    {
      key: "gait",
      label: "Походка",
      options: ["без замечаний", "изменена", "затруднена"],
    },
    {
      key: "condition",
      label: "Общее состояние",
      options: ["без замечаний", "требует внимания", "требует осмотра"],
    },
    { key: "notes", label: "Комментарий", optional: true },
  ],
  "laboratory-results": [
    {
      key: "test",
      label: "Анализ",
      options: ["bhb", "calcium", "scc", "fat", "protein", "lactose"],
    },
    { key: "value", label: "Результат", unit: "в единицах выбранного анализа" },
  ],
  "body-condition-records": [
    { key: "bcs", label: "Упитанность BCS", unit: "шкала 1–5; шаг 0,25" },
    {
      key: "hoof",
      label: "Состояние копыт",
      options: ["без замечаний", "требует внимания", "требует осмотра"],
    },
  ],
  "veterinary-examinations": [
    {
      key: "rectal_temperature_c",
      label: "Ректальная температура",
      unit: "°C",
      optional: true,
    },
    {
      key: "reproductive_status",
      label: "Репродуктивный статус",
      options: ["не указан", "стельная", "не стельная", "после отёла"],
    },
    { key: "notes", label: "Заметки" },
  ],
};
export function ManualPage() {
  const [params] = useSearchParams();
  return (
    <>
      <div className="page-heading">
        <div>
          <span className="eyebrow">НАБЛЮДЕНИЯ</span>
          <h1>Новая ручная запись</h1>
          <p>Сохраните измерение с фактическим временем наблюдения.</p>
        </div>
      </div>
      <section className="panel form-panel">
        <ManualForm cow={params.get("cow_id") || undefined} />
      </section>
    </>
  );
}
export function ManualForm({
  cow,
  record,
  onSaved,
}: {
  cow?: string;
  record?: RecordEntry;
  onSaved?: () => void;
}) {
  const [kind, setKind] = useState(record?.kind || "milkings");
  const [error, setError] = useState<unknown>();
  const [success, setSuccess] = useState(false);
  const [busy, setBusy] = useState(false);
  const [dirty, setDirty] = useState(false);
  const key = useRef(crypto.randomUUID());
  const client = useQueryClient();
  useEffect(() => {
    const fn = (e: BeforeUnloadEvent) => {
      if (dirty) {
        e.preventDefault();
      }
    };
    window.addEventListener("beforeunload", fn);
    return () => window.removeEventListener("beforeunload", fn);
  }, [dirty]);
  const date = new Date(record?.observed_at || Date.now());
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
    .toISOString()
    .slice(0, 16);
  return (
    <form
      className="manual-form"
      onChange={() => {
        setDirty(true);
        setSuccess(false);
      }}
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        setError(undefined);
        const f = new FormData(e.currentTarget);
        const payload: Record<string, unknown> = {};
        for (const field of fields[kind]) {
          const raw = String(f.get(field.key) || "");
          if (raw !== "") payload[field.key] = field.unit ? Number(raw) : raw;
        }
        if (kind === "laboratory-results") {
          payload.unit = (
            {
              bhb: "mmol/L",
              calcium: "mmol/L",
              scc: "cells/mL",
              fat: "%",
              protein: "%",
              lactose: "%",
            } as Record<string, string>
          )[String(payload.test)];
        }
        try {
          await api(
            "/" + kind + (record ? "/" + record.id : ""),
            record ? "PATCH" : "POST",
            {
              cow_id: String(f.get("cow_id")),
              observed_at: new Date(String(f.get("observed_at"))).toISOString(),
              payload,
              ...(record
                ? { version: record.version, reason: String(f.get("reason")) }
                : {}),
            },
            key.current,
          );
          setDirty(false);
          setSuccess(true);
          key.current = crypto.randomUUID();
          await client.invalidateQueries();
          onSaved?.();
        } catch (err) {
          setError(err);
        } finally {
          setBusy(false);
        }
      }}
    >
      <div className="form-grid">
        <label>
          Тип записи
          <select
            value={kind}
            disabled={!!record}
            onChange={(e) => setKind(e.target.value)}
          >
            {Object.entries(kinds).map(([k, label]) => (
              <option key={k} value={k}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <label>
          Животное
          <input
            name="cow_id"
            defaultValue={cow}
            readOnly={!!cow}
            required
            placeholder="COW-001"
          />
        </label>
        <label>
          Время измерения
          <input
            type="datetime-local"
            name="observed_at"
            defaultValue={local}
            required
          />
        </label>
      </div>
      <h3>{kinds[kind]}</h3>
      <div className="form-grid" key={kind}>
        {fields[kind].map((field) => (
          <label key={field.key}>
            {field.label}
            {field.unit && <small>{field.unit}</small>}
            {field.options ? (
              <select
                name={field.key}
                defaultValue={String(
                  record?.payload[field.key] || field.options[0],
                )}
              >
                {field.options.map((o) => (
                  <option key={o}>{o}</option>
                ))}
              </select>
            ) : (
              <input
                name={field.key}
                type={field.unit ? "number" : "text"}
                min={field.key === "bcs" ? 1 : 0}
                max={field.key === "bcs" ? 5 : undefined}
                step={field.key === "bcs" ? 0.25 : "any"}
                defaultValue={String(record?.payload[field.key] ?? "")}
                required={!field.optional}
              />
            )}
          </label>
        ))}
      </div>
      {record && (
        <label>
          Причина исправления
          <input name="reason" required />
        </label>
      )}
      <p className="note">
        Автор определяется вашей учётной записью. Исправления сохраняются в
        истории.
      </p>
      <ErrorBox error={error} />
      {success && (
        <div className="success" role="status">
          Запись сохранена
        </div>
      )}
      <button className="primary" disabled={busy}>
        {busy
          ? "Сохранение…"
          : record
            ? "Сохранить исправление"
            : "Сохранить запись"}
      </button>
    </form>
  );
}

export const fieldLabels: Record<string, string> = {
  cow_id: "Животное",
  observed_at: "Время наблюдения",
  yield_l: "Объём молока, л",
  flow_l_min: "Скорость, л/мин",
  temperature_c: "Температура, °C",
  conductivity_fl: "Проводимость ПЛ, мСм/см",
  conductivity_fr: "Проводимость ПП, мСм/см",
  conductivity_rl: "Проводимость ЗЛ, мСм/см",
  conductivity_rr: "Проводимость ЗП, мСм/см",
  weight_kg: "Масса, кг",
  appetite: "Аппетит",
  gait: "Походка",
  condition: "Общее состояние",
  notes: "Заметки",
  test: "Анализ",
  value: "Результат",
  unit: "Единица",
  bcs: "Упитанность BCS (1–5)",
  hoof: "Состояние копыт",
  rectal_temperature_c: "Ректальная температура, °C",
  reproductive_status: "Репродуктивный статус",
};
