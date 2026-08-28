export function bytes(n: number) {
  n = Number(n || 0);
  const u = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (n >= 1024 && i < u.length - 1) {
    n /= 1024;
    i++;
  }
  return n.toFixed(i ? 1 : 0) + " " + u[i];
}

export function ymd(t?: string | null) {
  if (!t) return "";
  return String(t).slice(0, 10);
}

export function fmtExpire(t?: string | null) {
  return ymd(t) || "—";
}

export function trafficParts(limit: number) {
  const n = Number(limit || 0);
  if (!n) return { value: "", unit: "unlimited" as const };
  const TB = 1024 ** 4,
    GB = 1024 ** 3,
    MB = 1024 ** 2;
  if (n % TB === 0 || n >= TB) return { value: +(n / TB).toFixed(2), unit: "TB" as const };
  if (n % GB === 0 || n >= GB) return { value: +(n / GB).toFixed(2), unit: "GB" as const };
  return { value: +(n / MB).toFixed(2), unit: "MB" as const };
}

export function trafficLimitFrom(value: string, unit: string) {
  if (unit === "unlimited") return 0;
  const n = Number(value || 0);
  if (!(n > 0)) return 0;
  const mul: Record<string, number> = { MB: 1024 ** 2, GB: 1024 ** 3, TB: 1024 ** 4 };
  return Math.round(n * (mul[unit] || mul.GB));
}

export function remainingDays(expire?: string | null) {
  if (!expire) return null;
  const end = Date.parse(expire);
  if (!Number.isFinite(end)) return null;
  return Math.ceil((end - Date.now()) / 86_400_000);
}

export function trafficProgress(up = 0, down = 0, limit = 0) {
  const used = Number(up || 0) + Number(down || 0);
  if (!limit) return { used, limit: 0, pct: null as number | null };
  return { used, limit, pct: Math.min(100, Math.max(0, (used / limit) * 100)) };
}

export type UserStatus = { t: "on" | "quota" | "expired" | "off"; cls: "ok" | "bad" };

export function userStatus(u: {
  traffic_up?: number;
  traffic_down?: number;
  traffic_limit?: number;
  expire_at?: string | null;
  enable?: boolean;
}): UserStatus {
  const used = Number(u.traffic_up || 0) + Number(u.traffic_down || 0);
  if (u.traffic_limit && used >= u.traffic_limit) return { t: "quota", cls: "bad" };
  if (u.expire_at && Date.parse(u.expire_at) < Date.now()) return { t: "expired", cls: "bad" };
  if (!u.enable) return { t: "off", cls: "bad" };
  return { t: "on", cls: "ok" };
}
