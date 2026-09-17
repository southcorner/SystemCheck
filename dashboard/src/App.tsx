import { useEffect, useState } from "react";
import {
  api,
  Machine,
  ScreenshotMeta,
  AppUsage,
  DomainRow,
  TransferRow,
  DownloadRow,
  EventRow,
  Alert,
  AdminUser,
  ViewRequest,
  Policy,
  VisitRow,
  MachineHealth,
  formatBytes,
} from "./api";

// A machine is "online" if its last heartbeat was within this window.
const ONLINE_WINDOW_MS = 15 * 60 * 1000;
function isOnline(lastSeen: string | null): boolean {
  if (!lastSeen) return false;
  return Date.now() - new Date(lastSeen).getTime() < ONLINE_WINDOW_MS;
}

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

type TopView = "machines" | "alerts" | "approvals" | "users" | "settings";

function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [machines, setMachines] = useState<Machine[]>([]);
  const [selected, setSelected] = useState<Machine | null>(null);
  const [view, setView] = useState<TopView>("machines");
  const [role, setRole] = useState<string>("viewer");

  const loadMachines = () => api.machines().then(setMachines).catch(() => setMachines([]));
  useEffect(() => {
    loadMachines();
    api.me().then((m) => setRole(m.role)).catch(() => setRole("viewer"));
    // Refresh periodically so the online/offline indicator stays current.
    const t = setInterval(loadMachines, 60 * 1000);
    return () => clearInterval(t);
  }, []);

  const editNickname = async (m: Machine) => {
    const name = window.prompt(`Nickname for ${m.hostname} (e.g. employee name):`, m.nickname || "");
    if (name === null) return; // cancelled
    try {
      await api.setNickname(m.id, name.trim());
      loadMachines();
    } catch (e) {
      alert("Could not save nickname: " + (e as Error).message);
    }
  };

  const navBtn = (v: TopView, label: string) => (
    <button className={"link" + (view === v ? " sel" : "")} onClick={() => setView(v)}>
      {label}
    </button>
  );

  return (
    <div className="layout">
      <header>
        <strong>SystemCheck</strong>
        <span className="muted">transparent endpoint monitoring</span>
        <nav>
          {navBtn("machines", "Machines")}
          {navBtn("alerts", "Alerts")}
          {navBtn("approvals", "Approvals")}
          {role === "admin" && navBtn("users", "Users")}
          {role === "admin" && navBtn("settings", "Settings")}
        </nav>
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
      {view === "alerts" && <AlertsView />}
      {view === "approvals" && <ApprovalsView />}
      {view === "users" && <UsersView />}
      {view === "settings" && <SettingsView />}
      {view === "machines" && (
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
                <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
                  <span
                    title={isOnline(m.last_seen) ? "Online (seen in last 15 min)" : "Offline"}
                    style={{
                      display: "inline-block",
                      width: 9,
                      height: 9,
                      borderRadius: "50%",
                      background: isOnline(m.last_seen) ? "#22c55e" : "#ef4444",
                      flex: "0 0 auto",
                    }}
                  />
                  <span className={m.nickname ? "" : "mono"} style={{ fontWeight: m.nickname ? 600 : 400 }}>
                    {m.nickname || m.hostname}
                  </span>
                  <button
                    className="link"
                    title="Set nickname"
                    onClick={(e) => { e.stopPropagation(); editNickname(m); }}
                    style={{ marginLeft: "auto", padding: "0 4px" }}
                  >
                    ✎
                  </button>
                </div>
                {m.nickname && <div className="muted small mono">{m.hostname}</div>}
                <div className="muted small">
                  {m.assigned_user || "unassigned"} · {m.group || "no group"}
                </div>
                <div className="small">
                  <span
                    className="mono"
                    title="Agent version"
                    style={{
                      fontSize: 11,
                      padding: "0 5px",
                      borderRadius: 4,
                      border: "1px solid currentColor",
                      opacity: 0.7,
                      marginRight: 6,
                    }}
                  >
                    v{m.agent_version || "?"}
                  </span>
                  {m.last_seen ? "seen " + new Date(m.last_seen).toLocaleString() : "never seen"}
                </div>
              </div>
            ))}
          </aside>
          <main>{selected ? <MachineView machine={selected} /> : <Empty />}</main>
        </div>
      )}
    </div>
  );
}

function Empty() {
  return (
    <div className="muted pad">
      Select a machine to view its activity. Every view is recorded in the audit log.
    </div>
  );
}

const COLLECTORS: { key: keyof Policy; label: string; note?: string }[] = [
  { key: "screenshot", label: "Screenshots" },
  { key: "foreground", label: "App usage (foreground)" },
  { key: "dns", label: "DNS lookups" },
  { key: "netflow", label: "Network transfers" },
  { key: "fswatch", label: "File watch / downloads" },
  { key: "browsing", label: "Browsing history (URLs visited)" },
  { key: "usb", label: "USB / removable media" },
  { key: "printjobs", label: "Print jobs" },
  { key: "installs", label: "Software installs" },
  { key: "posture", label: "Device posture" },
  { key: "seclog", label: "Security log" },
];

function SettingsView() {
  const [group, setGroup] = useState("default");
  const [pol, setPol] = useState<Policy | null>(null);
  const [status, setStatus] = useState<string>("");
  const [loading, setLoading] = useState(true);

  const load = (g: string) => {
    setLoading(true);
    setStatus("");
    api
      .getPolicy(g)
      .then((r) => {
        setPol(r.policy);
        setStatus(r.version ? `Editing policy v${r.version} for group “${g}”.` : `No saved policy for “${g}” — showing defaults.`);
      })
      .catch((e) => setStatus("Load failed: " + (e as Error).message))
      .finally(() => setLoading(false));
  };
  useEffect(() => load(group), []); // eslint-disable-line react-hooks/exhaustive-deps

  const setEnabled = (key: keyof Policy, v: boolean) => {
    if (!pol) return;
    setPol({ ...pol, [key]: { ...(pol[key] as object), enabled: v } });
  };
  const setShot = (field: string, v: number) => {
    if (!pol) return;
    setPol({ ...pol, screenshot: { ...pol.screenshot, [field]: v } });
  };

  const save = () => {
    if (!pol) return;
    setStatus("Saving…");
    api
      .setPolicy(group, pol)
      .then((r) => setStatus(`Saved as v${r.version}. Agents in “${group}” apply it within ~5 minutes.`))
      .catch((e) => setStatus("Save failed: " + (e as Error).message));
  };

  return (
    <div className="pad" style={{ maxWidth: 620 }}>
      <h2>Monitoring settings</h2>
      <div style={{ display: "flex", gap: 8, alignItems: "center", margin: "8px 0 4px" }}>
        <label>Group</label>
        <input value={group} onChange={(e) => setGroup(e.target.value)} style={{ width: 160 }} />
        <button className="link" onClick={() => load(group)}>Load</button>
      </div>
      <p className="muted small">
        Settings apply to all machines enrolled in this group. Collection still
        requires each user's recorded consent.
      </p>

      {loading || !pol ? (
        <p className="muted">{status || "Loading…"}</p>
      ) : (
        <>
          <h3>Collectors</h3>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 6 }}>
            {COLLECTORS.map(({ key, label }) => (
              <label key={String(key)} style={{ display: "flex", gap: 6, alignItems: "center" }}>
                <input
                  type="checkbox"
                  checked={(pol[key] as { enabled: boolean }).enabled}
                  onChange={(e) => setEnabled(key, e.target.checked)}
                />
                {label}
              </label>
            ))}
          </div>

          <h3 style={{ marginTop: 16 }}>Screenshot interval</h3>
          <p className="muted small">
            A screenshot is taken at a random moment between the min and max
            interval, then it repeats. Set min = max for a fixed interval.
          </p>
          <div style={{ display: "flex", gap: 12, flexWrap: "wrap", alignItems: "center" }}>
            <label>Min (sec)
              <input type="number" min={5} value={pol.screenshot.min_interval_sec}
                onChange={(e) => setShot("min_interval_sec", Number(e.target.value))}
                style={{ width: 90, marginLeft: 6 }} />
            </label>
            <label>Max (sec)
              <input type="number" min={5} value={pol.screenshot.max_interval_sec}
                onChange={(e) => setShot("max_interval_sec", Number(e.target.value))}
                style={{ width: 90, marginLeft: 6 }} />
            </label>
            <label>Quality (1–100)
              <input type="number" min={1} max={100} value={pol.screenshot.quality}
                onChange={(e) => setShot("quality", Number(e.target.value))}
                style={{ width: 80, marginLeft: 6 }} />
            </label>
          </div>
          <p className="muted small" style={{ marginTop: 6 }}>
            ≈ every {Math.round(pol.screenshot.min_interval_sec / 60)}–
            {Math.round(pol.screenshot.max_interval_sec / 60)} min.
          </p>

          <div style={{ marginTop: 16 }}>
            <button onClick={save}>Save settings</button>
          </div>
        </>
      )}
      {status && !loading && <p className="muted small" style={{ marginTop: 8 }}>{status}</p>}
    </div>
  );
}

const TABS = ["apps", "screenshots", "browsing", "domains", "transfers", "downloads", "security", "health"] as const;
type Tab = (typeof TABS)[number];

function MachineView({ machine }: { machine: Machine }) {
  const [tab, setTab] = useState<Tab>("apps");
  return (
    <div className="pad">
      <h2 style={{ marginBottom: 0 }}>{machine.nickname || machine.hostname}</h2>
      <div className="muted small mono">
        {machine.hostname}{machine.assigned_user ? " · " + machine.assigned_user : ""} · agent v{machine.agent_version || "?"}
      </div>
      <div className="tabs">
        {TABS.map((t) => (
          <button key={t} className={"tab" + (tab === t ? " sel" : "")} onClick={() => setTab(t)}>
            {t}
          </button>
        ))}
      </div>
      {tab === "apps" && <AppsTab id={machine.id} />}
      {tab === "screenshots" && <ScreenshotsTab id={machine.id} />}
      {tab === "browsing" && <BrowsingTab id={machine.id} />}
      {tab === "domains" && <DomainsTab id={machine.id} />}
      {tab === "transfers" && <TransfersTab id={machine.id} />}
      {tab === "downloads" && <DownloadsTab id={machine.id} />}
      {tab === "security" && <SecurityTab id={machine.id} />}
      {tab === "health" && <HealthTab id={machine.id} />}
    </div>
  );
}

function AppsTab({ id }: { id: string }) {
  const [rows, setRows] = useState<AppUsage[]>([]);
  useEffect(() => {
    api.apps(id).then(setRows).catch(() => setRows([]));
  }, [id]);
  if (rows.length === 0) return <p className="muted">No usage recorded.</p>;
  return (
    <table>
      <tbody>
        {rows.map((a) => (
          <tr key={a.process}>
            <td className="mono">{a.process}</td>
            <td className="right">{Math.round(a.active_sec / 60)} min</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function ScreenshotsTab({ id }: { id: string }) {
  const [shots, setShots] = useState<ScreenshotMeta[]>([]);
  const [needApproval, setNeedApproval] = useState(false);
  const [requested, setRequested] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);

  const load = () => {
    api
      .screenshots(id)
      .then((s) => {
        setShots(s);
        setSelected(new Set());
        setNeedApproval(false);
      })
      .catch((err) => {
        if ((err as Error).message === "approval_required") setNeedApproval(true);
        else setShots([]);
      });
  };
  useEffect(load, [id]);

  const toggle = (sid: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(sid) ? next.delete(sid) : next.add(sid);
      return next;
    });
  };
  const deleteOne = async (sid: string) => {
    if (!confirm("Delete this screenshot permanently?")) return;
    setBusy(true);
    try {
      await api.deleteScreenshot(sid);
      load();
    } finally {
      setBusy(false);
    }
  };
  const deleteSelected = async () => {
    if (selected.size === 0) return;
    if (!confirm(`Delete ${selected.size} screenshot(s) permanently?`)) return;
    setBusy(true);
    try {
      await api.deleteScreenshots([...selected]);
      load();
    } finally {
      setBusy(false);
    }
  };

  if (needApproval) {
    return (
      <div className="approval-gate">
        <p>
          Viewing this person's screenshots requires a second administrator's approval
          (dual control). Every view is recorded in the audit log.
        </p>
        {requested ? (
          <p className="muted">Access requested. Ask another admin to approve it under Approvals, then reload.</p>
        ) : (
          <button
            onClick={async () => {
              await api.requestView(id, "screenshot review");
              setRequested(true);
            }}
          >
            Request access
          </button>
        )}
        <button className="link" onClick={load}>
          Reload
        </button>
      </div>
    );
  }

  if (shots.length === 0) return <p className="muted">No screenshots in range.</p>;
  const allSelected = selected.size === shots.length && shots.length > 0;
  return (
    <div>
      <div style={{ display: "flex", gap: 10, alignItems: "center", marginBottom: 10, flexWrap: "wrap" }}>
        <label style={{ display: "flex", gap: 6, alignItems: "center" }}>
          <input
            type="checkbox"
            checked={allSelected}
            onChange={(e) => setSelected(e.target.checked ? new Set(shots.map((s) => s.id)) : new Set())}
          />
          Select all ({shots.length})
        </label>
        <button
          onClick={deleteSelected}
          disabled={selected.size === 0 || busy}
          style={{ color: selected.size ? "#b91c1c" : undefined }}
        >
          Delete selected{selected.size ? ` (${selected.size})` : ""}
        </button>
        {busy && <span className="muted small">working…</span>}
      </div>
      <div className="grid">
        {shots.map((s) => (
          <figure key={s.id} style={{ position: "relative", outline: selected.has(s.id) ? "2px solid #3b82f6" : "none" }}>
            <input
              type="checkbox"
              checked={selected.has(s.id)}
              onChange={() => toggle(s.id)}
              title="Select"
              style={{ position: "absolute", top: 6, left: 6, width: 18, height: 18, zIndex: 1 }}
            />
            <button
              onClick={() => deleteOne(s.id)}
              disabled={busy}
              title="Delete this screenshot"
              style={{
                position: "absolute", top: 4, right: 4, zIndex: 1,
                border: "none", borderRadius: 4, cursor: "pointer",
                background: "rgba(0,0,0,0.6)", color: "#fff", lineHeight: 1, padding: "2px 6px",
              }}
            >
              ✕
            </button>
            <img src={api.imageURL(s.id)} alt={s.ts} loading="lazy" />
            <figcaption className="small">{new Date(s.ts).toLocaleString()}</figcaption>
          </figure>
        ))}
      </div>
    </div>
  );
}

function HealthTab({ id }: { id: string }) {
  const [h, setH] = useState<MachineHealth | null>(null);
  const load = () => api.health(id).then(setH).catch(() => setH({ collectors: null, health_at: null }));
  useEffect(() => { load(); }, [id]); // eslint-disable-line react-hooks/exhaustive-deps
  const cols = h?.collectors || [];
  return (
    <div>
      <div className="muted small" style={{ marginBottom: 8 }}>
        {h?.health_at ? "Last report " + new Date(h.health_at).toLocaleString() : "No health report yet."}{" "}
        <button className="link" onClick={load}>Refresh</button>
      </div>
      {cols.length === 0 ? (
        <p className="muted">No collector status reported yet.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th className="left">Collector</th>
              <th className="left">Status</th>
              <th className="left">Where</th>
              <th className="left">Last error</th>
            </tr>
          </thead>
          <tbody>
            {cols.map((c, i) => (
              <tr key={c.name + i}>
                <td className="mono">{c.name}</td>
                <td style={{ fontWeight: 600, color: c.running ? "#16a34a" : "#dc2626" }}>
                  {c.running ? "● running" : "● stopped"}
                </td>
                <td className="small">{c.role}</td>
                <td className="small" style={{ color: "#dc2626" }}>{c.error || ""}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function BrowsingTab({ id }: { id: string }) {
  const [rows, setRows] = useState<VisitRow[]>([]);
  const [q, setQ] = useState("");
  useEffect(() => {
    api.visits(id).then(setRows).catch(() => setRows([]));
  }, [id]);
  const filtered = q
    ? rows.filter(
        (v) =>
          v.url.toLowerCase().includes(q.toLowerCase()) ||
          (v.title || "").toLowerCase().includes(q.toLowerCase()) ||
          v.domain.toLowerCase().includes(q.toLowerCase())
      )
    : rows;
  return (
    <div>
      <input
        placeholder="Filter by URL, title, or domain…"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        style={{ marginBottom: 8, width: "min(420px, 100%)" }}
      />
      {rows.length === 0 ? (
        <p className="muted">No browsing history recorded yet.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th className="left">Page</th>
              <th className="left">Domain</th>
              <th className="left">Browser</th>
              <th className="right">When</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((v, i) => (
              <tr key={v.url + i}>
                <td title={v.url} style={{ maxWidth: 380, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  <a href={v.url} target="_blank" rel="noreferrer">{v.title || v.url}</a>
                </td>
                <td className="mono small">{v.domain}</td>
                <td className="small">{v.browser}</td>
                <td className="right small">{new Date(v.ts).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function DomainsTab({ id }: { id: string }) {
  const [rows, setRows] = useState<DomainRow[]>([]);
  useEffect(() => {
    api.domains(id).then(setRows).catch(() => setRows([]));
  }, [id]);
  if (rows.length === 0) return <p className="muted">No domains recorded yet.</p>;
  return (
    <table>
      <thead>
        <tr>
          <th className="left">Domain</th>
          <th className="right">Queries</th>
          <th className="right">Last seen</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((d) => (
          <tr key={d.domain}>
            <td className="mono">{d.domain}</td>
            <td className="right">{d.count}</td>
            <td className="right small">{new Date(d.last_seen).toLocaleString()}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function TransfersTab({ id }: { id: string }) {
  const [rows, setRows] = useState<TransferRow[]>([]);
  useEffect(() => {
    api.transfers(id).then(setRows).catch(() => setRows([]));
  }, [id]);
  if (rows.length === 0) return <p className="muted">No transfer data yet.</p>;
  return (
    <table>
      <thead>
        <tr>
          <th className="left">Process</th>
          <th className="right">Uploaded</th>
          <th className="right">Downloaded</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((t) => (
          <tr key={t.process}>
            <td className="mono">{t.process || "(unknown)"}</td>
            <td className="right">{formatBytes(t.sent_bytes)}</td>
            <td className="right">{formatBytes(t.recv_bytes)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function DownloadsTab({ id }: { id: string }) {
  const [rows, setRows] = useState<DownloadRow[]>([]);
  useEffect(() => {
    api.downloads(id).then(setRows).catch(() => setRows([]));
  }, [id]);
  if (rows.length === 0) return <p className="muted">No downloads recorded yet.</p>;
  return (
    <table>
      <thead>
        <tr>
          <th className="left">File</th>
          <th className="right">Size</th>
          <th className="left">From (domain)</th>
          <th className="right">When</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((d, i) => (
          <tr key={d.path + i}>
            <td className="mono" title={d.path}>
              {d.name}
            </td>
            <td className="right">{formatBytes(d.size)}</td>
            <td className="small" title={d.url}>{d.domain || "—"}</td>
            <td className="right small">{new Date(d.ts).toLocaleString()}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function SecurityTab({ id }: { id: string }) {
  const [usb, setUsb] = useState<EventRow[]>([]);
  const [prints, setPrints] = useState<EventRow[]>([]);
  const [installs, setInstalls] = useState<EventRow[]>([]);
  const [posture, setPosture] = useState<EventRow[]>([]);
  const [seclog, setSeclog] = useState<EventRow[]>([]);

  useEffect(() => {
    api.events(id, "usb").then(setUsb).catch(() => setUsb([]));
    api.events(id, "printjob").then(setPrints).catch(() => setPrints([]));
    api.events(id, "install").then(setInstalls).catch(() => setInstalls([]));
    api.events(id, "posture").then(setPosture).catch(() => setPosture([]));
    api.events(id, "seclog").then(setSeclog).catch(() => setSeclog([]));
  }, [id]);

  const latest = posture[0]?.data;

  return (
    <div>
      <section>
        <h3>Device posture</h3>
        {!latest && <p className="muted">No posture snapshot yet.</p>}
        {latest && (
          <div className="posture">
            <Badge ok={!!latest.bitlocker && latest.bitlocker !== "Off"} label={`Encryption: ${latest.bitlocker ?? "?"}`} />
            <Badge ok={!!latest.defender_enabled} label={`Antivirus: ${latest.defender_enabled ? "on" : "off"}`} />
            <Badge ok={!!latest.realtime} label={`Realtime: ${latest.realtime ? "on" : "off"}`} />
            <Badge ok={!!latest.firewall_on} label={`Firewall: ${latest.firewall_on ? "on" : "off"}`} />
            <Badge ok={true} label={`Last patch: ${latest.last_hotfix ?? "?"}`} />
          </div>
        )}
      </section>

      <section>
        <h3>Removable media</h3>
        {usb.length === 0 && <p className="muted">No USB events.</p>}
        {usb.length > 0 && (
          <table>
            <tbody>
              {usb.map((e, i) => (
                <tr key={i}>
                  <td>{String(e.data.action)}</td>
                  <td className="mono">{String(e.data.drive ?? "")}</td>
                  <td>{String(e.data.label ?? "")}</td>
                  <td className="right small">{new Date(e.ts).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section>
        <h3>Print jobs</h3>
        {prints.length === 0 && <p className="muted">No print jobs.</p>}
        {prints.length > 0 && (
          <table>
            <tbody>
              {prints.map((e, i) => (
                <tr key={i}>
                  <td className="mono">{String(e.data.document ?? "")}</td>
                  <td>{String(e.data.printer ?? "")}</td>
                  <td className="right">{String(e.data.pages ?? "")} pp</td>
                  <td className="right small">{new Date(e.ts).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section>
        <h3>New installs</h3>
        {installs.length === 0 && <p className="muted">No new installs.</p>}
        {installs.length > 0 && (
          <table>
            <tbody>
              {installs.map((e, i) => (
                <tr key={i}>
                  <td className="mono">{String(e.data.name ?? "")}</td>
                  <td>{String(e.data.version ?? "")}</td>
                  <td className="right small">{new Date(e.ts).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section>
        <h3>Sign-in events</h3>
        {seclog.length === 0 && <p className="muted">No sign-in events (enable security log in policy).</p>}
        {seclog.length > 0 && (
          <table>
            <tbody>
              {seclog.map((e, i) => (
                <tr key={i}>
                  <td>{String(e.data.kind ?? "")}</td>
                  <td className="mono">{String(e.data.account ?? "")}</td>
                  <td className="small">{String(e.data.source_ip ?? "")}</td>
                  <td className="right small">{new Date(e.ts).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}

function Badge({ ok, label }: { ok: boolean; label: string }) {
  return <span className={"badge " + (ok ? "badge-ok" : "badge-bad")}>{label}</span>;
}

function UsersView() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState("viewer");
  const [error, setError] = useState("");
  const load = () => api.users().then(setUsers).catch(() => setUsers([]));
  useEffect(() => { load(); }, []);

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      await api.createUser(email, password, role);
      setEmail("");
      setPassword("");
      load();
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <div className="pad">
      <h2>Administrators</h2>
      <table>
        <thead>
          <tr>
            <th className="left">Email</th>
            <th className="left">Role</th>
            <th className="left">MFA</th>
            <th className="right">Actions</th>
          </tr>
        </thead>
        <tbody>
          {users.map((u) => (
            <tr key={u.id} className={u.disabled ? "muted" : ""}>
              <td className="mono">{u.email}</td>
              <td>
                <select value={u.role} onChange={async (e) => { await api.setUserRole(u.id, e.target.value); load(); }}>
                  <option value="admin">admin</option>
                  <option value="auditor">auditor</option>
                  <option value="viewer">viewer</option>
                </select>
              </td>
              <td>{u.mfa_enabled ? "on" : "off"}</td>
              <td className="right">
                <button className="link" onClick={async () => { await api.disableUser(u.id, !u.disabled); load(); }}>
                  {u.disabled ? "Enable" : "Disable"}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <h3>Add administrator</h3>
      <form className="row" onSubmit={create}>
        <input placeholder="Email" value={email} onChange={(e) => setEmail(e.target.value)} />
        <input placeholder="Temp password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
        <select value={role} onChange={(e) => setRole(e.target.value)}>
          <option value="admin">admin</option>
          <option value="auditor">auditor</option>
          <option value="viewer">viewer</option>
        </select>
        <button type="submit">Add</button>
      </form>
      {error && <div className="error">{error}</div>}
    </div>
  );
}

function ApprovalsView() {
  const [rows, setRows] = useState<ViewRequest[]>([]);
  const load = () => api.viewRequests().then(setRows).catch(() => setRows([]));
  useEffect(() => { load(); }, []);
  const active = (r: ViewRequest) =>
    r.approved_by && new Date(r.expires_at) > new Date();
  return (
    <div className="pad">
      <h2>Screenshot view requests</h2>
      <p className="muted">
        Viewing an individual's screenshots requires approval by a different administrator.
      </p>
      {rows.length === 0 && <p className="muted">No requests.</p>}
      <table>
        <thead>
          <tr>
            <th className="left">Requested by</th>
            <th className="left">Machine</th>
            <th className="left">Reason</th>
            <th className="left">Status</th>
            <th className="right">Action</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.id}>
              <td className="mono">{r.requested_by}</td>
              <td className="mono small">{r.machine_id.slice(0, 8)}</td>
              <td>{r.reason}</td>
              <td>{active(r) ? "approved" : r.approved_by ? "expired" : "pending"}</td>
              <td className="right">
                {!r.approved_by && (
                  <button className="link" onClick={async () => { await api.approveView(r.id); load(); }}>
                    Approve
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function AlertsView() {
  const [rows, setRows] = useState<Alert[]>([]);
  const load = () => api.alerts().then(setRows).catch(() => setRows([]));
  useEffect(() => {
    load();
  }, []);
  return (
    <div className="pad">
      <h2>Alerts</h2>
      {rows.length === 0 && <p className="muted">No alerts.</p>}
      <table>
        <tbody>
          {rows.map((a) => (
            <tr key={a.id} className={a.acknowledged ? "muted" : ""}>
              <td>
                <span className={"sev sev-" + a.severity}>{a.severity}</span>
              </td>
              <td>{a.message}</td>
              <td className="right small">{new Date(a.ts).toLocaleString()}</td>
              <td className="right">
                {!a.acknowledged && (
                  <button
                    className="link"
                    onClick={async () => {
                      await api.ackAlert(a.id);
                      load();
                    }}
                  >
                    Acknowledge
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
