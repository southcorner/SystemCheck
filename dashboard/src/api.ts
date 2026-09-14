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
  imageURL: (screenshotID: string) => `/api/screenshots/${screenshotID}/image`,
};
