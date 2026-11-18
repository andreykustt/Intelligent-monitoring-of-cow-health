import { useMemo, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  type ColumnDef,
} from "@tanstack/react-table";
import {
  api,
  type Cow,
  type Page,
  type Overview,
  type Metric,
  type Point,
  type RecordEntry,
  type TimelineEntry,
  metricNames,
  time,
} from "./api";
import { Badge, Chart, Empty, ErrorBox, Pager } from "./components";
import { ManualForm, kinds, fieldLabels } from "./manual";
export function OverviewPage({ farm }: { farm: string }) {
  const q = useQuery({
    queryKey: ["overview", farm],
    queryFn: () => api<Overview>("/analytics/overview?farm_id=" + farm),
  });
  return (
    <>
      <div className="page-heading">
        <div>
          <span className="eyebrow">РАБОЧЕЕ ПРОСТРАНСТВО</span>
          <h1>Обзор стада</h1>
          <p>Главное о показателях и событиях на вашей ферме.</p>
        </div>
        <Link className="button primary" to="/monitor">
          Открыть монитор
        </Link>
      </div>
      <ErrorBox error={q.error} />
      <div className="stats">
        {[
          ["Животных в стаде", q.data?.total, "Всего зарегистрировано"],
          ["С показаниями", q.data?.with_data, "Есть сохранённая телеметрия"],
          [
            "Обнаружены аномалии",
            q.data?.anomaly,
            "Последний результат анализа",
          ],
          ["Открытых уведомлений", q.data?.alerts, "Ожидают внимания"],
        ].map(([label, value, desc]) => (
          <article className="stat" key={label}>
            <span>{label}</span>
            <strong>{value ?? "—"}</strong>
            <small>{desc}</small>
          </article>
        ))}
      </div>
      <div className="split">
        <section className="panel">
          <h2>Результаты анализа</h2>
          <p className="muted">Последние завершённые окна по животным</p>
          {[
            ["no_anomaly", q.data?.no_anomaly],
            ["anomaly", q.data?.anomaly],
            ["insufficient_data", q.data?.insufficient_data],
          ].map(([s, n]) => (
            <div className="summary-row" key={s}>
              <Badge status={String(s)} />
              <strong>{n ?? "—"}</strong>
            </div>
          ))}
          <p className="note">
            Отсутствие аномалии не является заключением о здоровье животного.
          </p>
        </section>
        <section className="panel">
          <h2>История показателей</h2>
          <History farm={farm} />
        </section>
      </div>
      <section className="banner">
        <span className="banner-icon">◌</span>
        <div>
          <h3>Наблюдайте изменения, сохраняйте контекст</h3>
          <p>
            Добавляйте результаты доения и осмотров в карточках животных. Ручные
            записи дополняют телеметрию.
          </p>
        </div>
        <Link className="button" to="/records">
          Добавить запись
        </Link>
      </section>
    </>
  );
}
export function Monitor({ farm }: { farm: string }) {
  const [params, setParams] = useSearchParams();
  const page = Math.max(1, Number(params.get("page") || 1));
  const sort = params.get("sort") || "id";
  const search = params.get("q") || "";
  const status = params.get("status") || "";
  const query = useQuery({
    queryKey: ["cows", farm, params.toString()],
    queryFn: () =>
      api<Page<Cow>>(
        "/cows?" +
          new URLSearchParams({
            farm_id: farm,
            page: String(page),
            sort,
            q: search,
            status,
          }),
      ),
  });
  const update = (k: string, v: string) =>
    setParams((p) => {
      p.set(k, v);
      if (k !== "page") p.set("page", "1");
      return p;
    });
  const columns = useMemo<ColumnDef<Cow>[]>(
    () => [
      {
        accessorKey: "id",
        header: "Животное",
        cell: (c) => (
          <Link className="cow-link" to={"/cows/" + c.row.original.id}>
            <span className="cow-avatar">◇</span>
            <span>
              {c.row.original.id}
              <small>{c.row.original.name}</small>
            </span>
          </Link>
        ),
      },
      {
        accessorKey: "status",
        header: "Анализ",
        cell: (c) => <Badge status={c.row.original.status} />,
      },
      ...["body_temperature_c", "heart_rate_bpm", "activity_index"].map(
        (metric) => ({
          id: metric,
          header: metricNames[metric],
          cell: ({ row }: { row: { original: Cow } }) => {
            const m = row.original.latest[metric];
            return m ? (
              <span className={m.stale ? "stale" : ""}>
                {m.value.toFixed(metric === "activity_index" ? 2 : 1)}{" "}
                <small>{m.unit}</small>
                {m.stale && <small className="block">Устарело</small>}
              </span>
            ) : (
              <span className="muted">Нет показаний</span>
            );
          },
        }),
      ),
    ],
    [],
  );
  const table = useReactTable({
    data: query.data?.items || [],
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualSorting: true,
  });
  return (
    <>
      <div className="page-heading">
        <div>
          <span className="eyebrow">ТЕЛЕМЕТРИЯ</span>
          <h1>Монитор животных</h1>
          <p>Последние сохранённые показатели. Источник — симулятор.</p>
        </div>
        <span className="count-label">{query.data?.total ?? "—"} животных</span>
      </div>
      <section className="panel table-panel">
        <div className="toolbar">
          <input
            aria-label="Поиск животных"
            placeholder="Найти по номеру или имени…"
            value={search}
            onChange={(e) => update("q", e.target.value)}
          />
          <select
            aria-label="Статус анализа"
            value={status}
            onChange={(e) => update("status", e.target.value)}
          >
            <option value="">Все результаты анализа</option>
            <option value="anomaly">Аномалия</option>
            <option value="no_anomaly">Нет аномалии</option>
            <option value="insufficient_data">Недостаточно данных</option>
          </select>
          <select
            aria-label="Сортировка"
            value={sort}
            onChange={(e) => update("sort", e.target.value)}
          >
            <option value="id">Номер: по возрастанию</option>
            <option value="-id">Номер: по убыванию</option>
            <option value="name">По имени</option>
            <option value="status">По результату анализа</option>
          </select>
        </div>
        <ErrorBox error={query.error} />
        {query.isPending ? (
          <p className="loading">Загрузка животных…</p>
        ) : !query.data?.items.length ? (
          <Empty>Животные не найдены</Empty>
        ) : (
          <div className="table-scroll">
            <table>
              <thead>
                {table.getHeaderGroups().map((g) => (
                  <tr key={g.id}>
                    {g.headers.map((h) => (
                      <th key={h.id}>
                        {flexRender(h.column.columnDef.header, h.getContext())}
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody>
                {table.getRowModel().rows.map((row) => (
                  <tr key={row.id}>
                    {row.getVisibleCells().map((c) => (
                      <td key={c.id}>
                        {flexRender(c.column.columnDef.cell, c.getContext())}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <Pager
          page={page}
          total={query.data?.total || 0}
          onChange={(p) => update("page", String(p))}
        />
      </section>
    </>
  );
}
export function History({ cow, farm }: { cow?: string; farm?: string }) {
  const [metric, setMetric] = useState("body_temperature_c");
  const [hours, setHours] = useState(1);
  const [anchor] = useState(() => Date.now());
  const metrics = useQuery({
    queryKey: ["metrics"],
    queryFn: () => api<Metric[]>("/metrics"),
  });
  const q = useQuery({
    queryKey: ["history", cow, farm, metric, hours, anchor],
    queryFn: () =>
      api<Point[]>(
        "/telemetry/history?" +
          new URLSearchParams({
            cow_id: cow || "",
            farm_id: farm || "",
            metric_type: metric,
            from: new Date(Date.now() - hours * 3600000).toISOString(),
            to: new Date().toISOString(),
          }),
      ),
  });
  return (
    <>
      <div className="toolbar">
        <select
          aria-label="Метрика"
          value={metric}
          onChange={(e) => setMetric(e.target.value)}
        >
          {metrics.data?.map((m) => (
            <option key={m.code} value={m.code}>
              {m.name}
            </option>
          ))}
          <option value="milk_yield_l">Надой по ручным записям</option>
          {null}
        </select>
        <select
          aria-label="Период"
          value={hours}
          onChange={(e) => setHours(Number(e.target.value))}
        >
          <option value={1}>Последний час</option>
          <option value={24}>Сутки</option>
          <option value={168}>Неделя</option>
        </select>
      </div>
      <ErrorBox error={q.error} />
      {q.isPending ? (
        <p>Загрузка истории…</p>
      ) : q.data?.some((p) => p.value !== null) ? (
        <Chart
          points={q.data}
          unit={
            metric === "milk_yield_l"
              ? "л"
              : metrics.data?.find((m) => m.code === metric)?.unit || ""
          }
        />
      ) : (
        <Empty>Нет показаний за выбранный период</Empty>
      )}
    </>
  );
}
export function CowDetail() {
  const { id } = useParams();
  const [form, setForm] = useState(false);
  const [recordPage, setRecordPage] = useState(1);
  const q = useQuery({
    queryKey: ["cow", id],
    queryFn: () => api<Cow>("/cows/" + id),
  });
  const records = useQuery({
    queryKey: ["records", id, recordPage],
    queryFn: () =>
      api<Page<TimelineEntry>>(
        "/cows/" + id + "/health-summary?page=" + recordPage,
      ),
  });
  return (
    <>
      <Link className="back-link" to="/monitor">
        Вернуться к монитору
      </Link>
      <ErrorBox error={q.error} />
      {q.data && (
        <>
          <div className="page-heading">
            <div>
              <span className="eyebrow">
                КАРТОЧКА ЖИВОТНОГО · {q.data.farm_id}
              </span>
              <h1>
                {q.data.id} <span className="light-title">{q.data.name}</span>
              </h1>
              <p>
                Синтетическая телеметрия · последнее окно анализа:{" "}
                {time(q.data.analysis_at)}
              </p>
            </div>
            <Badge status={q.data.status} />
          </div>
          {q.data.analysis_delayed && (
            <div className="note">
              Анализ задерживается. Показан последний завершённый результат.
            </div>
          )}
          <div className="metric-grid">
            {Object.entries(metricNames).map(([key, label]) => {
              const m = q.data.latest[key];
              return (
                <article className="panel metric-card" key={key}>
                  <span>{label}</span>
                  <strong>
                    {m ? m.value.toFixed(2) : "—"} <small>{m?.unit}</small>
                  </strong>
                  <small className={m?.stale ? "stale" : "muted"}>
                    {m
                      ? m.stale
                        ? "Показание устарело"
                        : "Последнее показание"
                      : "Нет показаний"}
                  </small>
                  <small>{time(m?.recorded_at)}</small>
                </article>
              );
            })}
          </div>
          <section className="panel">
            <h2>История показателей</h2>
            <History cow={id} />
          </section>
          <section className="panel">
            <div className="section-heading">
              <h2>Журнал наблюдений</h2>
              <button onClick={() => setForm(!form)}>
                {form ? "Свернуть форму" : "Добавить запись"}
              </button>
            </div>
            {form && (
              <ManualForm
                cow={id}
                onSaved={() => {
                  setForm(false);
                  void records.refetch();
                }}
              />
            )}
            <ErrorBox error={records.error} />
            {records.data?.items.length ? (
              records.data.items.map((rec) =>
                rec.entry_type === "manual" ? (
                  <RecordCard key={rec.data.id} record={rec.data} />
                ) : (
                  <article className="record" key={rec.data.id}>
                    <div className="section-heading">
                      <strong>
                        {rec.entry_type === "prediction"
                          ? "Результат анализа"
                          : "Уведомление"}{" "}
                        · {time(rec.event_at)}
                      </strong>
                      <Badge
                        status={
                          rec.entry_type === "prediction"
                            ? rec.data.status
                            : rec.data.state
                        }
                      />
                    </div>
                    {rec.entry_type === "prediction" ? (
                      <p>
                        Модель: {rec.data.model_version}. Оценка аномальности:{" "}
                        {rec.data.anomaly_score?.toFixed(3) ?? "нет"}.
                      </p>
                    ) : (
                      <Link to="/alerts">Открыть уведомления</Link>
                    )}
                  </article>
                ),
              )
            ) : (
              <Empty>Наблюдений пока нет</Empty>
            )}
            <Pager
              page={recordPage}
              total={records.data?.total || 0}
              onChange={setRecordPage}
            />
          </section>
        </>
      )}
    </>
  );
}
function RecordCard({ record }: { record: RecordEntry }) {
  const [edit, setEdit] = useState(false);
  const [history, setHistory] = useState(false);
  const q = useQuery({
    queryKey: ["revisions", record.id],
    queryFn: () =>
      api<Record<string, unknown>[]>(
        "/" + record.kind + "/" + record.id + "/revisions",
      ),
    enabled: history,
  });
  return (
    <article className="record">
      <div className="section-heading">
        <strong>
          {kinds[record.kind] || record.kind} · {time(record.observed_at)}
        </strong>
        <span>Версия {record.version}</span>
      </div>
      <dl>
        {Object.entries(record.payload).map(([k, v]) => (
          <div key={k}>
            <dt>{fieldLabels[k] || k}</dt>
            <dd>{String(v)}</dd>
          </div>
        ))}
      </dl>
      <small>
        Создано {time(record.created_at)} · автор{" "}
        {record.created_by_name || record.created_by}
      </small>
      <div className="actions">
        <button onClick={() => setEdit(!edit)}>Исправить</button>
        <button onClick={() => setHistory(!history)}>
          История исправлений
        </button>
      </div>
      {edit && (
        <ManualForm
          cow={record.cow_id}
          record={record}
          onSaved={() => setEdit(false)}
        />
      )}
      <ErrorBox error={q.error} />
      {history &&
        q.data?.map((v, i) => (
          <div className="revision" key={i}>
            Версия {String(v.version)} · {String(v.reason)} ·{" "}
            {time(String(v.created_at))}
            <dl>
              {Object.entries((v.payload || {}) as Record<string, unknown>).map(
                ([key, value]) => (
                  <div key={key}>
                    <dt>{fieldLabels[key] || key}</dt>
                    <dd>{String(value)}</dd>
                  </div>
                ),
              )}
            </dl>
          </div>
        ))}
    </article>
  );
}
