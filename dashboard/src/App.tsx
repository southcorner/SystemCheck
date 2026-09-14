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
  formatBytes,
} from "./api";

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

type TopView = "machines" | "alerts" | "approvals" | "users";

function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [machines, setMachines] = useState<Machine[]>([]);
  const [selected, setSelected] = useState<Machine | null>(null);
  const [view, setView] = useState<TopView>("machines");
  const [role, setRole] = useState<string>("viewer");

  useEffect(() => {
    api.machines().then(setMachines).catch(() => setMachines([]));
    api.me().then((m) => setRole(m.role)).catch(() => setRole("viewer"));
  }, []);

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

const TABS = ["apps", "screenshots", "domains", "transfers", "downloads", "security"] as const;
type Tab = (typeof TABS)[number];

function MachineView({ machine }: { machine: Machine }) {
  const [tab, setTab] = useState<Tab>("apps");
  return (
    <div className="pad">
      <h2 className="mono">{machine.hostname}</h2>
      <div className="tabs">
        {TABS.map((t) => (
          <button key={t} className={"tab" + (tab === t ? " sel" : "")} onClick={() => setTab(t)}>
            {t}
          </button>
        ))}
      </div>
      {tab === "apps" && <AppsTab id={machine.id} />}
      {tab === "screenshots" && <ScreenshotsTab id={machine.id} />}
      {tab === "domains" && <DomainsTab id={machine.id} />}
      {tab === "transfers" && <TransfersTab id={machine.id} />}
      {tab === "downloads" && <DownloadsTab id={machine.id} />}
      {tab === "security" && <SecurityTab id={machine.id} />}
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

  const load = () => {
    api
      .screenshots(id)
      .then((s) => {
        setShots(s);
        setNeedApproval(false);
      })
      .catch((err) => {
        if ((err as Error).message === "approval_required") setNeedApproval(true);
        else setShots([]);
      });
  };
  useEffect(load, [id]);

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
  return (
    <div className="grid">
      {shots.map((s) => (
        <figure key={s.id}>
          <img src={api.imageURL(s.id)} alt={s.ts} loading="lazy" />
          <figcaption className="small">{new Date(s.ts).toLocaleString()}</figcaption>
        </figure>
      ))}
    </div>
  );
}

function DomainsTab({ id }: { id: string }) {
  const [rows, setRows] = useState<DomainRow[]>([]);
  useEffect(() => {
    api.domains(id).then(setRows).catch(() => setRows([]));
  }, [id]);
  if (rows.length === 0) return <p className="muted">No domains recorded (enable DNS in policy).</p>;
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
  if (rows.length === 0) return <p className="muted">No transfer data (enable netflow in policy).</p>;
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
  if (rows.length === 0) return <p className="muted">No downloads recorded (enable file watch in policy).</p>;
  return (
    <table>
      <thead>
        <tr>
          <th className="left">File</th>
          <th className="right">Size</th>
          <th className="left">Source</th>
          <th className="right">When</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((d, i) => (
          <tr key={d.path + i}>
            <td className="mono" title={d.url || d.path}>
              {d.name}
            </td>
            <td className="right">{formatBytes(d.size)}</td>
            <td className="small">{d.source}</td>
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
        {!latest && <p className="muted">No posture snapshot (enable posture in policy).</p>}
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
