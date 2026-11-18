import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  NavLink,
  Routes,
  Route,
  Navigate,
  useLocation,
} from "react-router-dom";
import { api, setCSRF, type Identity } from "./api";
import { ErrorBox } from "./components";
import { Monitor, CowDetail, OverviewPage } from "./monitor";
import { ManualPage } from "./manual";
import {
  AlertsPage,
  SimulationsPage,
  UsersPage,
  AnimalsPage,
} from "./management";
export function App() {
  const client = useQueryClient();
  const [expired, setExpired] = useState(false);
  const me = useQuery({
    queryKey: ["me"],
    queryFn: () => api<Identity>("/auth/me"),
    retry: false,
    enabled: !expired,
  });
  useEffect(() => {
    const fn = () => {
      setExpired(true);
      client.clear();
    };
    window.addEventListener("session-expired", fn);
    return () => window.removeEventListener("session-expired", fn);
  }, [client]);
  if (me.isPending && !expired)
    return <div className="boot">Загрузка приложения…</div>;
  if (expired || !me.data)
    return (
      <Login
        onLogin={() => {
          setExpired(false);
          void client.invalidateQueries({ queryKey: ["me"] });
        }}
      />
    );
  setCSRF(me.data.csrf_token);
  return (
    <Shell
      user={me.data}
      onLogout={() => {
        setExpired(true);
        client.clear();
      }}
    />
  );
}
function Login({ onLogin }: { onLogin: () => void }) {
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);
  return (
    <main className="login">
      <section className="login-story">
        <div className="brand">
          <span className="brand-mark">С</span> стадо
          <span className="brand-caption">мониторинг животных</span>
        </div>
        <span className="eyebrow">ДАННЫЕ ДЛЯ ВНИМАТЕЛЬНОЙ РАБОТЫ</span>
        <h1>
          Всё стадо.
          <br />В поле зрения.
        </h1>
        <p>Показатели, наблюдения и события — в одном рабочем пространстве.</p>
        <div className="orbit">
          <div />
          <div />
          <div />
          <span>
            200<small>животных в учебном стаде</small>
          </span>
        </div>
        <p className="disclosure">
          Учебный проект · Синтетическая телеметрия
          <br />
          Анализ аномалий не является диагнозом.
        </p>
      </section>
      <section className="login-form">
        <span className="eyebrow">РАБОЧЕЕ ПРОСТРАНСТВО</span>
        <h2>Добро пожаловать</h2>
        <p>Войдите, чтобы продолжить наблюдение.</p>
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            setBusy(true);
            setError(undefined);
            const f = new FormData(e.currentTarget);
            try {
              await api("/auth/login", "POST", {
                username: f.get("username"),
                password: f.get("password"),
              });
              onLogin();
            } catch (err) {
              setError(err);
            } finally {
              setBusy(false);
            }
          }}
        >
          <label>
            Логин
            <input name="username" autoComplete="username" required autoFocus />
          </label>
          <label>
            Пароль
            <input
              type="password"
              name="password"
              autoComplete="current-password"
              required
            />
          </label>
          <ErrorBox error={error} />
          <button className="primary" disabled={busy}>
            {busy ? "Вход…" : "Войти в систему"}
          </button>
        </form>
        <small>Доступ предоставляется администратором фермы.</small>
      </section>
    </main>
  );
}
function Shell({ user, onLogout }: { user: Identity; onLogout: () => void }) {
  const [farm, setFarm] = useState(user.farms[0] || "");
  const [connected, setConnected] = useState(false);
  const [menu, setMenu] = useState(false);
  const client = useQueryClient();
  const location = useLocation();
  useEffect(() => {
    let disposed = false;
    let socket: WebSocket | undefined;
    let timer: ReturnType<typeof setTimeout>;
    let retry = 0;
    let dirty = false;
    const refresh = () => {
      if (dirty) {
        dirty = false;
        void client.invalidateQueries({
          predicate: (q) => q.queryKey[0] !== "me",
        });
      }
    };
    const interval = setInterval(refresh, 1000);
    const connect = () => {
      if (disposed) return;
      socket = new WebSocket(
        `${window.location.protocol === "https:" ? "wss" : "ws"}://${window.location.host}/api/v1/stream?farm_id=${encodeURIComponent(farm)}`,
      );
      socket.onopen = () => {
        retry = 0;
        setConnected(true);
        dirty = true;
      };
      socket.onmessage = () => {
        dirty = true;
      };
      socket.onerror = () => socket?.close();
      socket.onclose = () => {
        setConnected(false);
        if (!disposed)
          timer = setTimeout(
            connect,
            Math.min(30000, 1000 * 2 ** retry++) + Math.random() * 500,
          );
      };
    };
    connect();
    return () => {
      disposed = true;
      clearTimeout(timer);
      clearInterval(interval);
      socket?.close();
    };
  }, [farm, client]);
  useEffect(() => setMenu(false), [location.pathname]);
  const admin = user.role === "admin";
  return (
    <div className="shell">
      <aside className={menu ? "sidebar open" : "sidebar"}>
        <NavLink to="/" className="brand">
          <span className="brand-mark">С</span> стадо
        </NavLink>
        <div className="sidebar-caption">МОНИТОРИНГ ЖИВОТНЫХ</div>
        <nav>
          <NavLink to="/" end>
            ▦ <span>Обзор стада</span>
          </NavLink>
          <NavLink to="/monitor">
            ▤ <span>Монитор</span>
          </NavLink>
          <NavLink to="/records">
            ▧ <span>Ручные записи</span>
          </NavLink>
          <NavLink to="/alerts">
            ◉ <span>Уведомления</span>
          </NavLink>
          {admin && (
            <>
              <span className="nav-section">УПРАВЛЕНИЕ</span>
              <NavLink to="/simulations">
                ▷ <span>Симуляции</span>
              </NavLink>
              <NavLink to="/animals">
                ◇ <span>Животные</span>
              </NavLink>
              <NavLink to="/users">
                ♙ <span>Пользователи</span>
              </NavLink>
            </>
          )}
        </nav>
        <div className="sidebar-bottom">
          <span className="demo-label">УЧЕБНЫЙ ПРОЕКТ</span>
          <p>
            Синтетические данные.
            <br />
            Аномалия — повод для проверки,
            <br />
            не медицинский диагноз.
          </p>
        </div>
      </aside>
      <div className="workspace">
        <header className="topbar">
          <button
            className="mobile-menu"
            onClick={() => setMenu(!menu)}
            aria-label="Открыть меню"
          >
            ☰
          </button>
          <select
            aria-label="Ферма"
            value={farm}
            onChange={(e) => setFarm(e.target.value)}
          >
            {user.farms.map((f) => (
              <option key={f} value={f}>
                {f === "demo" ? "Учебная ферма" : f}
              </option>
            ))}
          </select>
          <span className={"connection " + (connected ? "online" : "")}>
            {connected ? "Поток подключён" : "Переподключение…"}
          </span>
          <div className="user">
            <span className="avatar">{user.username[0].toUpperCase()}</span>
            <span>
              {user.username}
              <small>{admin ? "Администратор" : "Оператор"}</small>
            </span>
          </div>
          <button
            className="quiet"
            onClick={async () => {
              await api("/auth/logout", "POST");
              onLogout();
            }}
          >
            Выйти
          </button>
        </header>
        <main className="content">
          <Routes>
            <Route path="/" element={<OverviewPage farm={farm} />} />
            <Route path="/monitor" element={<Monitor farm={farm} />} />
            <Route path="/cows/:id" element={<CowDetail />} />
            <Route path="/records" element={<ManualPage />} />
            <Route path="/alerts" element={<AlertsPage farm={farm} />} />
            {admin && (
              <>
                <Route
                  path="/simulations"
                  element={<SimulationsPage farm={farm} />}
                />
                <Route path="/animals" element={<AnimalsPage farm={farm} />} />
                <Route path="/users" element={<UsersPage user={user} />} />
              </>
            )}
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </main>
        <footer>
          Стадо · Учебная система мониторинга{" "}
          <span>Время отображается в часовом поясе устройства</span>
        </footer>
      </div>
    </div>
  );
}
