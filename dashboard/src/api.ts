// Minimal typed client for the SystemCheck admin API.

export type Machine = {
  id: string;
  hostname: string;
  os: string;
  os_version: string;
  assigned_user: string;
  group: string;
  last_seen: string | null;
  agent_version: string;
  active: boolean;
  nickname: string;
};

export type ScreenshotMeta = {
  id: string;
  ts: string;
  object_key: string;
  width: number;
  height: number;
  bytes: number;
  format: string;
};

export type AppUsage = { process: string; active_sec: number };

export type DomainRow = { domain: string; count: number; last_seen: string };
export type TransferRow = { process: string; sent_bytes: number; recv_bytes: number };
export type DownloadRow = {
  ts: string;
  name: string;
  path: string;
  size: number;
  url: string;
  domain: string;
  source: string;
};
export type EventRow = { ts: string; kind: string; data: Record<string, any> };
export type VisitRow = { ts: string; url: string; title: string; domain: string; browser: string };
export type CollectorStatus = { name: string; running: boolean; error?: string; role: string };
export type MachineHealth = { collectors: CollectorStatus[] | null; health_at: string | null };

export type AdminUser = {
  id: string;
  email: string;
  role: string;
  mfa_enabled: boolean;
  disabled: boolean;
};

export type ViewRequest = {
  id: string;
  machine_id: string;
  requested_by: string;
  reason: string;
  approved_by: string;
  created_at: string;
  approved_at: string | null;
  expires_at: string;
};

export type Alert = {
  id: string;
  machine_id: string;
  ts: string;
  severity: string;
  message: string;
  data: Record<string, unknown>;
  acknowledged: boolean;
};

export type Policy = {
  version: number;
  active?: boolean;
  screenshot: {
    enabled: boolean;
    min_interval_sec: number;
    max_interval_sec: number;
    max_width: number;
    format: string;
    quality: number;
    blur_regions?: unknown[];
  };
  foreground: { enabled: boolean; poll_sec: number; idle_threshold_sec: number };
  dns: { enabled: boolean };
  netflow: { enabled: boolean; rollup_sec: number };
  fswatch: { enabled: boolean; folders: string[]; include_browser_history: boolean };
  browsing: { enabled: boolean };
  usb: { enabled: boolean };
  printjobs: { enabled: boolean };
  installs: { enabled: boolean };
  posture: { enabled: boolean; interval_sec: number };
  seclog: { enabled: boolean };
  exclusions?: { domains: string[]; processes: string[] };
  heartbeat_sec: number;
};
export type GroupPolicy = { group: string; version: number; policy: Policy };

async function req<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    ...opts,
    headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
    credentials: "include",
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as any).error || `HTTP ${res.status}`);
  }
  return res.json() as Promise<T>;
}

// List endpoints: the server serializes an empty slice as JSON `null`, which
// would make callers crash on `.length`/`.map`. Coerce null to an empty array
// so list results are always safe to iterate.
async function reqList<T>(path: string, opts: RequestInit = {}): Promise<T[]> {
  const r = await req<T[] | null>(path, opts);
  return r ?? [];
}

export const api = {
  login: (email: string, password: string) =>
    req<{ mfa_required: boolean; role: string }>("/api/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),
  mfaVerify: (code: string) =>
    req<{ ok: boolean }>("/api/mfa/verify", {
      method: "POST",
      body: JSON.stringify({ code }),
    }),
  mfaEnroll: () =>
    req<{ secret: string; uri: string }>("/api/mfa/enroll", { method: "POST" }),
  me: () => req<{ email: string; role: string; mfa_enabled: boolean }>("/api/me"),
  logout: () => req<{ ok: boolean }>("/api/logout", { method: "POST" }),
  machines: () => reqList<Machine>("/api/machines"),
  setNickname: (id: string, nickname: string) =>
    req<{ ok: boolean }>(`/api/machines/${id}/nickname`, {
      method: "POST",
      body: JSON.stringify({ nickname }),
    }),
  screenshots: (id: string) =>
    reqList<ScreenshotMeta>(`/api/machines/${id}/screenshots`),
  apps: (id: string) => reqList<AppUsage>(`/api/machines/${id}/apps`),
  domains: (id: string) => reqList<DomainRow>(`/api/machines/${id}/domains`),
  visits: (id: string) => reqList<VisitRow>(`/api/machines/${id}/visits`),
  health: (id: string) => req<MachineHealth>(`/api/machines/${id}/health`),
  transfers: (id: string) => reqList<TransferRow>(`/api/machines/${id}/transfers`),
  downloads: (id: string) => reqList<DownloadRow>(`/api/machines/${id}/downloads`),
  events: (id: string, kind: string) =>
    reqList<EventRow>(`/api/machines/${id}/events?kind=${encodeURIComponent(kind)}`),
  alerts: (unacked = false) =>
    reqList<Alert>(`/api/alerts${unacked ? "?unacked=true" : ""}`),
  ackAlert: (id: string) =>
    req<{ ok: boolean }>(`/api/alerts/${id}/ack`, { method: "POST" }),
  imageURL: (screenshotID: string) => `/api/screenshots/${screenshotID}/image`,
  deleteScreenshot: (id: string) =>
    req<{ ok: boolean; deleted: number }>(`/api/screenshots/${id}`, { method: "DELETE" }),
  deleteScreenshots: (ids: string[]) =>
    req<{ ok: boolean; deleted: number }>(`/api/screenshots/delete`, {
      method: "POST",
      body: JSON.stringify({ ids }),
    }),

  // User management (admin only).
  users: () => reqList<AdminUser>("/api/users"),
  createUser: (email: string, password: string, role: string) =>
    req<{ id: string }>("/api/users", {
      method: "POST",
      body: JSON.stringify({ email, password, role }),
    }),
  setUserRole: (id: string, role: string) =>
    req<{ ok: boolean }>(`/api/users/${id}/role`, {
      method: "POST",
      body: JSON.stringify({ role }),
    }),
  disableUser: (id: string, disabled: boolean) =>
    req<{ ok: boolean }>(`/api/users/${id}/disable`, {
      method: "POST",
      body: JSON.stringify({ disabled }),
    }),

  // Dual-approval view requests.
  viewRequests: () => reqList<ViewRequest>("/api/view-requests"),
  requestView: (machineID: string, reason: string) =>
    req<{ id: string }>("/api/view-requests", {
      method: "POST",
      body: JSON.stringify({ machine_id: machineID, reason }),
    }),
  approveView: (id: string) =>
    req<{ ok: boolean }>(`/api/view-requests/${id}/approve`, { method: "POST" }),

  // Policy settings (admin).
  getPolicy: (group = "default") =>
    req<GroupPolicy>(`/api/policy?group=${encodeURIComponent(group)}`),
  setPolicy: (group: string, policy: Policy) =>
    req<GroupPolicy>(`/api/policy`, {
      method: "PUT",
      body: JSON.stringify({ group, policy }),
    }),
};

// formatBytes renders a byte count in human units.
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(1)} ${units[i]}`;
}
