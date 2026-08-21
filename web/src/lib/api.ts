export function basePath() {
  let p = location.pathname;
  if (p.endsWith("index.html")) p = p.slice(0, -10);
  if (!p.endsWith("/")) p += "/";
  return p;
}

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

export async function api<T = unknown>(path: string, opts: RequestInit & { json?: unknown } = {}): Promise<T> {
  const url = path.startsWith("/") ? basePath() + path.replace(/^\//, "") : path;
  const headers = new Headers(opts.headers);
  let body = opts.body;
  if (opts.json !== undefined) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(opts.json);
  }
  const res = await fetch(url, {
    credentials: "same-origin",
    ...opts,
    headers,
    body,
  });
  const text = await res.text();
  let data: unknown = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = text;
  }
  if (!res.ok) {
    const rec = data as { error?: string } | null;
    const msg = rec?.error || (typeof data === "string" ? data : "") || String(res.status);
    throw new ApiError(msg, res.status);
  }
  return data as T;
}
