import { useEffect, useState } from "react";
import { api, Machine, ScreenshotMeta, AppUsage } from "./api";

type Stage = "login" | "mfa" | "app";

export function App() {
  const [stage, setStage] = useState<Stage>("login");
  const [error, setError] = useState("");

  useEffect(() => {
    api.me().then(() => setStage("app")).catch(() => setStage("login"));
  }, []);

  if (stage === "login") return <Login onLogin={setStage} setError={setError} error={error} />;
  if (stage === "mfa") return <MFA onDone={() => setStage("app")} setError={setError} error={error} />;
  return <Dashboard onLogout={() => setStage("login")} />;
}

function Login({
  onLogin,
  setError,
  error,
}: {
  onLogin: (s: Stage) => void;
  setError: (s: string) => void;
  error: string;
}) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      const r = await api.login(email, password);
      onLogin(r.mfa_required ? "mfa" : "app");
    } catch (err) {
      setError((err as Error).message);
    }
  };
  return (
    <div className="center">
      <form className="card" onSubmit={submit}>
        <h1>SystemCheck</h1>
        <p className="muted">Admin sign in</p>
        <input placeholder="Email" value={email} onChange={(e) => setEmail(e.target.value)} />
        <input
          placeholder="Password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        {error && <div className="error">{error}</div>}
        <button type="submit">Sign in</button>
      </form>
    </div>
  );
}

function MFA({
  onDone,
  setError,
  error,
}: {
  onDone: () => void;
  setError: (s: string) => void;
  error: string;
}) {
  const [code, setCode] = useState("");
  const [enroll, setEnroll] = useState<{ uri: string; secret: string } | null>(null);
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      await api.mfaVerify(code);
      onDone();
    } catch (err) {
      setError((err as Error).message);
    }
  };
  return (
    <div className="center">
      <form className="card" onSubmit={submit}>
        <h1>Two-factor</h1>
        <p className="muted">Enter the 6-digit code from your authenticator app.</p>
        <input placeholder="123456" value={code} onChange={(e) => setCode(e.target.value)} />
        {error && <div className="error">{error}</div>}
        <button type="submit">Verify</button>
        <button
          type="button"
          className="link"
          onClick={async () => setEnroll(await api.mfaEnroll())}
        >
          Set up an authenticator
        </button>
        {enroll && (
          <div className="enroll">
            <p className="muted">Add this secret to your app, then enter a code above:</p>
            <code>{enroll.secret}</code>
          </div>
        )}
      </form>
    </div>
  );
}

function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [machines, setMachines] = useState<Machine[]>([]);
  const [selected, setSelected] = useState<Machine | null>(null);

  useEffect(() => {
    api.machines().then(setMachines).catch(() => setMachines([]));
  }, []);

  return (
    <div className="layout">
      <header>
        <strong>SystemCheck</strong>
        <span className="muted">transparent endpoint monitoring</span>
        <button
          className="link right"
          onClick={async () => {
            await api.logout();
            onLogout();
          }}
        >
          Sign out
        </button>
      </header>
      <div className="body">
        <aside>
          <h3>Machines</h3>
          {machines.length === 0 && <p className="muted">No machines enrolled yet.</p>}
          {machines.map((m) => (
            <div
              key={m.id}
              className={"machine" + (selected?.id === m.id ? " active" : "")}
              onClick={() => setSelected(m)}
            >
              <div className="mono">{m.hostname}</div>
              <div className="muted small">
                {m.assigned_user || "unassigned"} · {m.group || "no group"}
              </div>
              <div className="small">
                {m.last_seen ? "last seen " + new Date(m.last_seen).toLocaleString() : "never seen"}
              </div>
            </div>
          ))}
        </aside>
        <main>{selected ? <MachineView machine={selected} /> : <Empty />}</main>
      </div>
    </div>
  );
}

function Empty() {
  return (
    <div className="muted pad">
      Select a machine to view its screenshots and application usage. Every view is
      recorded in the audit log.
    </div>
  );
}

function MachineView({ machine }: { machine: Machine }) {
  const [shots, setShots] = useState<ScreenshotMeta[]>([]);
  const [apps, setApps] = useState<AppUsage[]>([]);

  useEffect(() => {
    api.screenshots(machine.id).then(setShots).catch(() => setShots([]));
    api.apps(machine.id).then(setApps).catch(() => setApps([]));
  }, [machine.id]);

  return (
    <div className="pad">
      <h2 className="mono">{machine.hostname}</h2>
      <section>
        <h3>Application usage (24h)</h3>
        {apps.length === 0 && <p className="muted">No usage recorded.</p>}
        <table>
          <tbody>
            {apps.map((a) => (
              <tr key={a.process}>
                <td className="mono">{a.process}</td>
                <td className="right">{Math.round(a.active_sec / 60)} min</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
      <section>
        <h3>Screenshots</h3>
        {shots.length === 0 && <p className="muted">No screenshots in range.</p>}
        <div className="grid">
          {shots.map((s) => (
            <figure key={s.id}>
              <img src={api.imageURL(s.id)} alt={s.ts} loading="lazy" />
              <figcaption className="small">{new Date(s.ts).toLocaleString()}</figcaption>
            </figure>
          ))}
        </div>
      </section>
    </div>
  );
}
