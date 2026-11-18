import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import {
  api,
  type Alert,
  type Page,
  type SimConfig,
  type Simulation,
  type Identity,
  type User,
  type Cow,
  metricNames,
  time,
} from "./api";
import { Badge, ErrorBox, Empty, Pager } from "./components";
export function AlertsPage({ farm }: { farm: string }) {
  const [state, setState] = useState("");
  const [page, setPage] = useState(1);
  const [error, setError] = useState<unknown>();
  const [selected, setSelected] = useState("");
  const [note, setNote] = useState("");
  const client = useQueryClient();
  const q = useQuery({
    queryKey: ["alerts", farm, state, page],
    queryFn: () =>
      api<Page<Alert>>(`/alerts?farm_id=${farm}&state=${state}&page=${page}`),
  });
  const detail = useQuery({
    queryKey: ["alert", selected],
    queryFn: () =>
      api<
        Alert & {
          actions: { action: string; note: string; created_at: string }[];
          prediction: {
            anomaly_score: number;
            model_version: string;
            window_end: string;
          };
        }
      >("/alerts/" + selected),
    enabled: !!selected,
  });
  async function act(id: string, action: string) {
    setError(undefined);
    try {
      await api(`/alerts/${id}/${action}`, "POST", { note });
      setNote("");
      await client.invalidateQueries();
    } catch (e) {
      setError(e);
    }
  }
  return (
    <>
      <div className="page-heading">
        <div>
          <span className="eyebrow">ТРЕБУЕТ ВНИМАНИЯ</span>
          <h1>Уведомления</h1>
          <p>
            Необычные изменения показателей. Каждое событие требует проверки.
          </p>
        </div>
      </div>
      <section className="panel">
        <div className="toolbar">
          <select
            aria-label="Состояние уведомления"
            value={state}
            onChange={(e) => {
              setState(e.target.value);
              setPage(1);
            }}
          >
            <option value="">Все состояния</option>
            <option value="open">Открытые</option>
            <option value="acknowledged">Подтверждённые</option>
            <option value="resolved">Закрытые</option>
          </select>
        </div>
        <ErrorBox error={q.error || error} />
        {q.isPending ? (
          <p>Загрузка…</p>
        ) : !q.data?.items.length ? (
          <Empty>Уведомлений пока нет</Empty>
        ) : (
          q.data.items.map((a) => (
            <article className="record" key={a.id}>
              <div className="section-heading">
                <Link to={"/cows/" + a.cow_id}>{a.cow_id}</Link>
                <Badge status={a.state} />
              </div>
              <h3>Необычное сочетание показателей</h3>
              <p>Последнее обнаружение: {time(a.last_seen)}</p>
              <button
                onClick={() => setSelected(selected === a.id ? "" : a.id)}
              >
                Подробности и действия
              </button>
              {selected === a.id && (
                <div className="alert-detail">
                  <ErrorBox error={detail.error} />
                  <p>
                    Модель: {detail.data?.prediction.model_version} · оценка
                    аномальности:{" "}
                    {detail.data?.prediction.anomaly_score?.toFixed(3)}
                  </p>
                  <p className="note">
                    Оценка не является вероятностью заболевания.
                  </p>
                  {a.state !== "resolved" && (
                    <>
                      <label>
                        Комментарий
                        <input
                          value={note}
                          onChange={(e) => setNote(e.target.value)}
                          placeholder="Что проверено или предпринято"
                        />
                      </label>
                      <div className="actions">
                        {a.state === "open" && (
                          <button onClick={() => void act(a.id, "acknowledge")}>
                            Подтвердить
                          </button>
                        )}
                        <button
                          disabled={!note.trim()}
                          onClick={() => void act(a.id, "resolve")}
                        >
                          Закрыть с комментарием
                        </button>
                      </div>
                    </>
                  )}
                  {detail.data?.actions.map((x, i) => (
                    <p key={i}>
                      {time(x.created_at)} · {x.action} · {x.note}
                    </p>
                  ))}
                </div>
              )}
            </article>
          ))
        )}
        <Pager page={page} total={q.data?.total || 0} onChange={setPage} />
      </section>
    </>
  );
}
const defaultConfig: SimConfig = {
  cow_count: 200,
  metrics_enabled: Object.keys(metricNames),
  interval_ms_by_metric: Object.fromEntries(
    Object.keys(metricNames).map((m) => [m, 5000]),
  ),
  duration_seconds: 7200,
  mode: "synthetic",
  playback_speed: 1,
  scenario: "normal",
  seed: 42,
};
export function SimulationsPage({ farm }: { farm: string }) {
  const [page, setPage] = useState(1);
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);
  const [edit, setEdit] = useState<Simulation>();
  const [interval, setIntervalValue] = useState(5000);
  const [mode, setMode] = useState("synthetic");
  const client = useQueryClient();
  const q = useQuery({
    queryKey: ["simulations", farm, page],
    queryFn: () =>
      api<Page<Simulation>>(
        "/simulations?farm_id=" + encodeURIComponent(farm) + "&page=" + page,
      ),
    refetchInterval: 3000,
  });
  async function command(id: string, action: string) {
    try {
      await api(`/simulations/${id}/${action}`, "POST");
      await client.invalidateQueries();
    } catch (e) {
      setError(e);
    }
  }
  return (
    <>
      <div className="page-heading">
        <div>
          <span className="eyebrow">ИСТОЧНИК ДАННЫХ</span>
          <h1>Симуляции</h1>
          <p>Управление учебной телеметрией без подключения оборудования.</p>
        </div>
      </div>
      <section className="panel">
        <h2>{edit ? "Изменение конфигурации" : "Новая симуляция"}</h2>
        <form
          key={edit?.id || "new"}
          onSubmit={async (e) => {
            e.preventDefault();
            setError(undefined);
            setBusy(true);
            const f = new FormData(e.currentTarget);
            try {
              const config: SimConfig = {
                ...defaultConfig,
                cow_count: Number(f.get("count")),
                duration_seconds: Number(f.get("duration")),
                scenario: String(f.get("scenario")) as SimConfig["scenario"],
                mode: mode as SimConfig["mode"],
                seed: Number(f.get("seed")),
                metrics_enabled: f.getAll("metrics").map(String),
                interval_ms_by_metric: Object.fromEntries(
                  f
                    .getAll("metrics")
                    .map((m) => [String(m), Number(f.get("interval_" + m))]),
                ),
                playback_speed: Number(f.get("speed") || 1),
                replay:
                  mode === "replay"
                    ? JSON.parse(String(f.get("replay")))
                    : undefined,
              };
              await api(
                edit ? `/simulations/${edit.id}/config` : "/simulations",
                edit ? "PATCH" : "POST",
                edit ? config : { farm_id: farm, config },
              );
              setEdit(undefined);
              await client.invalidateQueries();
            } catch (err) {
              setError(err);
            } finally {
              setBusy(false);
            }
          }}
        >
          <div className="form-grid">
            <label>
              Животных
              <input
                type="number"
                name="count"
                min={1}
                max={1000}
                defaultValue={edit?.config.cow_count || 200}
                readOnly={!!edit}
                required
              />
            </label>
            <label>
              Длительность, секунд
              <input
                type="number"
                name="duration"
                min={1}
                max={86400}
                defaultValue={edit?.config.duration_seconds || 7200}
                required
              />
            </label>
            <label>
              Сценарий
              <select
                name="scenario"
                defaultValue={edit?.config.scenario || "normal"}
              >
                <option value="normal">Обычные колебания</option>
                <option value="anomaly">Устойчивые отклонения</option>
                <option value="missing">Пропуски показателей</option>
              </select>
            </label>
            <label>
              Режим
              <select
                value={mode}
                disabled={!!edit}
                onChange={(e) => setMode(e.target.value)}
              >
                <option value="synthetic">Генерация</option>
                <option value="replay">Повтор истории</option>
              </select>
            </label>
            <label>
              Seed
              <input
                type="number"
                name="seed"
                defaultValue={edit?.config.seed || 42}
              />
            </label>
            <label>
              Общий интервал, мс
              <input
                type="number"
                value={interval}
                min={1000}
                max={3600000}
                onChange={(e) => setIntervalValue(Number(e.target.value))}
              />
            </label>
          </div>
          <div className="metric-options">
            {Object.entries(metricNames).map(([key, label]) => (
              <label key={key} className="metric-option">
                <span>
                  <input
                    type="checkbox"
                    name="metrics"
                    value={key}
                    defaultChecked={
                      edit ? edit.config.metrics_enabled.includes(key) : true
                    }
                  />
                  {label}
                </span>
                <input
                  aria-label={"Интервал: " + label}
                  key={key + interval + edit?.id}
                  type="number"
                  name={"interval_" + key}
                  min={1000}
                  max={3600000}
                  defaultValue={
                    edit?.config.interval_ms_by_metric[key] || interval
                  }
                />
                <small>мс</small>
              </label>
            ))}
          </div>
          {mode === "replay" && (
            <>
              <label>
                Скорость воспроизведения
                <input
                  name="speed"
                  type="number"
                  min={0.01}
                  max={100}
                  step="any"
                  defaultValue={1}
                />
              </label>
              <label>
                Записи JSON (до 1000)
                <textarea
                  name="replay"
                  required
                  placeholder='[{"cow_id":"COW-001","metric_type":"body_temperature_c","value":38.6,"recorded_at":"2026-11-11T10:00:00Z"}]'
                />
              </label>
            </>
          )}
          <ErrorBox error={error} />
          <div className="actions">
            <button className="primary" disabled={busy}>
              {busy
                ? "Отправка…"
                : edit
                  ? "Изменить конфигурацию"
                  : "Запустить симуляцию"}
            </button>
            {edit && (
              <button type="button" onClick={() => setEdit(undefined)}>
                Отмена
              </button>
            )}
          </div>
        </form>
      </section>
      <section className="panel">
        <h2>Запуски</h2>
        <ErrorBox error={q.error} />
        {q.data?.items.length ? (
          q.data.items.map((s) => (
            <article className="record" key={s.id}>
              <div className="section-heading">
                <strong>
                  {s.config.cow_count} животных · {s.config.scenario}
                </strong>
                <Badge status={s.state} />
              </div>
              <small>
                {time(s.created_at)} · {s.id}
              </small>
              <p>
                Отправлено событий: {String(s.progress.sent || 0)} · повторов:{" "}
                {String(s.progress.retries || 0)} · время:{" "}
                {String(s.progress.elapsed_seconds || 0)} с
              </p>
              <div className="actions">
                {s.state === "running" && (
                  <button onClick={() => void command(s.id, "pause")}>
                    Пауза
                  </button>
                )}
                {s.state === "paused" && (
                  <button onClick={() => void command(s.id, "resume")}>
                    Продолжить
                  </button>
                )}
                {["running", "paused"].includes(s.state) && (
                  <>
                    <button onClick={() => void command(s.id, "stop")}>
                      Остановить
                    </button>
                    <button
                      onClick={() => {
                        setEdit(s);
                        setMode(s.config.mode);
                        window.scrollTo({ top: 0, behavior: "smooth" });
                      }}
                    >
                      Настроить
                    </button>
                  </>
                )}
              </div>
            </article>
          ))
        ) : (
          <Empty>Симуляций пока нет</Empty>
        )}
        <Pager page={page} total={q.data?.total || 0} onChange={setPage} />
      </section>
    </>
  );
}
export function AnimalsPage({ farm }: { farm: string }) {
  const [error, setError] = useState<unknown>();
  const [saved, setSaved] = useState(false);
  const [editing, setEditing] = useState<Cow>();
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const client = useQueryClient();
  const q = useQuery({
    queryKey: ["animals", farm, page, search],
    queryFn: () =>
      api<Page<Cow>>(
        `/cows?farm_id=${farm}&page=${page}&q=${encodeURIComponent(search)}`,
      ),
  });
  return (
    <>
      <div className="page-heading">
        <div>
          <span className="eyebrow">СПРАВОЧНИК</span>
          <h1>Животные</h1>
          <p>Регистрация и изменение сведений о животных.</p>
        </div>
      </div>
      <section className="panel">
        <h2>{editing ? "Изменить животное" : "Добавить животное"}</h2>
        <form
          key={editing?.id || "new"}
          onSubmit={async (e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget);
            try {
              await api(
                "/cows" + (editing ? "/" + editing.id : ""),
                editing ? "PATCH" : "POST",
                { id: f.get("id"), name: f.get("name"), farm_id: farm },
              );
              setSaved(true);
              setEditing(undefined);
              await client.invalidateQueries();
            } catch (err) {
              setError(err);
            }
          }}
        >
          <div className="form-grid">
            <label>
              Номер животного
              <input
                name="id"
                defaultValue={editing?.id}
                readOnly={!!editing}
                required
                maxLength={64}
              />
            </label>
            <label>
              Имя
              <input
                name="name"
                defaultValue={editing?.name}
                required
                maxLength={120}
              />
            </label>
          </div>
          <ErrorBox error={error} />
          {saved && <p className="success">Данные сохранены</p>}
          <div className="actions">
            <button className="primary">Сохранить</button>
            {editing && (
              <button type="button" onClick={() => setEditing(undefined)}>
                Отмена
              </button>
            )}
          </div>
        </form>
      </section>
      <section className="panel">
        <input
          placeholder="Поиск животных…"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(1);
          }}
          aria-label="Поиск животных"
        />
        <ErrorBox error={q.error} />
        {q.data?.items.map((c) => (
          <div className="summary-row" key={c.id}>
            <Link to={"/cows/" + c.id}>
              {c.id} · {c.name}
            </Link>
            <button
              onClick={() => {
                setEditing(c);
                setSaved(false);
              }}
            >
              Изменить
            </button>
          </div>
        ))}
        <Pager page={page} total={q.data?.total || 0} onChange={setPage} />
      </section>
    </>
  );
}
export function UsersPage({ user }: { user: Identity }) {
  const [page, setPage] = useState(1);
  const [editing, setEditing] = useState<User>();
  const [error, setError] = useState<unknown>();
  const [reset, setReset] = useState("");
  const client = useQueryClient();
  const q = useQuery({
    queryKey: ["users", page],
    queryFn: () => api<Page<User>>("/users?page=" + page),
  });
  return (
    <>
      <div className="page-heading">
        <div>
          <span className="eyebrow">ДОСТУП</span>
          <h1>Пользователи</h1>
          <p>Роли и доступ к фермам.</p>
        </div>
      </div>
      <section className="panel">
        <h2>{editing ? "Изменение доступа" : "Новый пользователь"}</h2>
        <form
          key={editing?.id || "new"}
          onSubmit={async (e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget);
            try {
              await api(
                "/users" + (editing ? "/" + editing.id : ""),
                editing ? "PATCH" : "POST",
                {
                  username: f.get("username"),
                  password: f.get("password") || "",
                  role: f.get("role"),
                  farms: f.getAll("farms"),
                  active: f.get("active") === "on",
                },
              );
              setEditing(undefined);
              await client.invalidateQueries();
            } catch (err) {
              setError(err);
            }
          }}
        >
          <div className="form-grid">
            <label>
              Логин
              <input
                name="username"
                defaultValue={editing?.username}
                readOnly={!!editing}
                required
              />
            </label>
            {!editing && (
              <label>
                Пароль
                <input
                  name="password"
                  type="password"
                  minLength={12}
                  maxLength={72}
                  required
                  autoComplete="new-password"
                />
              </label>
            )}
            <label>
              Роль
              <select name="role" defaultValue={editing?.role || "operator"}>
                <option value="operator">Оператор</option>
                <option value="admin">Администратор</option>
              </select>
            </label>
          </div>
          <div className="actions">
            {user.farms.map((f) => (
              <label className="check" key={f}>
                <input
                  type="checkbox"
                  name="farms"
                  value={f}
                  defaultChecked={editing ? editing.farms.includes(f) : true}
                />
                {f}
              </label>
            ))}
            <label className="check">
              <input
                type="checkbox"
                name="active"
                defaultChecked={editing?.active ?? true}
              />
              Активен
            </label>
          </div>
          <ErrorBox error={error} />
          <div className="actions">
            <button className="primary">Сохранить</button>
            {editing && (
              <button type="button" onClick={() => setEditing(undefined)}>
                Отмена
              </button>
            )}
          </div>
        </form>
      </section>
      <section className="panel">
        <ErrorBox error={q.error} />
        {q.data?.items.map((u) => (
          <article className="record" key={u.id}>
            <div className="section-heading">
              <strong>{u.username}</strong>
              <span>
                {u.role} · {u.active ? "Активен" : "Заблокирован"}
              </span>
            </div>
            <p>Фермы: {u.farms.join(", ")}</p>
            <div className="actions">
              <button onClick={() => setEditing(u)}>Изменить доступ</button>
              <button onClick={() => setReset(reset === u.id ? "" : u.id)}>
                Сбросить пароль
              </button>
            </div>
            {reset === u.id && (
              <form
                onSubmit={async (e) => {
                  e.preventDefault();
                  const f = new FormData(e.currentTarget);
                  try {
                    await api("/users/" + u.id + "/password", "POST", {
                      password: f.get("password"),
                    });
                    setReset("");
                  } catch (err) {
                    setError(err);
                  }
                }}
              >
                <label>
                  Новый пароль
                  <input
                    name="password"
                    type="password"
                    minLength={12}
                    maxLength={72}
                    required
                  />
                </label>
                <button>Установить и отозвать сессии</button>
              </form>
            )}
          </article>
        ))}
        <Pager page={page} total={q.data?.total || 0} onChange={setPage} />
      </section>
    </>
  );
}
