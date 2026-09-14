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
  source: string;
};
export type EventRow = { ts: string; kind: string; data: Record<string, any> };

export type Alert = {
  id: string;
  machine_id: string;
  ts: string;
  severity: string;
  message: string;
  data: Record<string, unknown>;
  acknowledged: boolean;
};

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
  machines: () => req<Machine[]>("/api/machines"),
  screenshots: (id: string) =>
    req<ScreenshotMeta[]>(`/api/machines/${id}/screenshots`),
  apps: (id: string) => req<AppUsage[]>(`/api/machines/${id}/apps`),
  domains: (id: string) => req<DomainRow[]>(`/api/machines/${id}/domains`),
  transfers: (id: string) => req<TransferRow[]>(`/api/machines/${id}/transfers`),
  downloads: (id: string) => req<DownloadRow[]>(`/api/machines/${id}/downloads`),
  events: (id: string, kind: string) =>
    req<EventRow[]>(`/api/machines/${id}/events?kind=${encodeURIComponent(kind)}`),
  alerts: (unacked = false) =>
    req<Alert[]>(`/api/alerts${unacked ? "?unacked=true" : ""}`),
  ackAlert: (id: string) =>
    req<{ ok: boolean }>(`/api/alerts/${id}/ack`, { method: "POST" }),
  imageURL: (screenshotID: string) => `/api/screenshots/${screenshotID}/image`,
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
