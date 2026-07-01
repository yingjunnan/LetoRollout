import type {
  DeploymentSummary,
  DeploymentDetail,
  ImageUpdateRequest,
  RolloutResult,
  TokenRecord,
  TokenScope,
  VerifyResponse,
} from "./types";

// A thrown ApiError carries the HTTP status so callers can branch on 401/403.
export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(
  method: string,
  path: string,
  token: string,
  body?: unknown
): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  const parsed = text ? JSON.parse(text) : null;
  if (!res.ok) {
    const msg =
      (parsed && (parsed as { error?: string }).error) || res.statusText;
    throw new ApiError(res.status, msg);
  }
  return parsed as T;
}

export const api = {
  verify: (token: string) =>
    request<VerifyResponse>("POST", "/api/v1/auth/verify", token),

  listDeployments: (token: string, ns: string) =>
    request<DeploymentSummary[]>(
      "GET",
      `/api/v1/namespaces/${encodeURIComponent(ns)}/deployments`,
      token
    ),

  getDeployment: (token: string, ns: string, name: string) =>
    request<DeploymentDetail>(
      "GET",
      `/api/v1/namespaces/${encodeURIComponent(ns)}/deployments/${encodeURIComponent(name)}`,
      token
    ),

  updateImage: (
    token: string,
    ns: string,
    name: string,
    req: ImageUpdateRequest
  ) =>
    request<RolloutResult>(
      "POST",
      `/api/v1/namespaces/${encodeURIComponent(ns)}/deployments/${encodeURIComponent(name)}/image`,
      token,
      req
    ),

  // Logs one-shot returns text/plain, not JSON.
  fetchLogs: async (
    token: string,
    ns: string,
    name: string,
    params: { container?: string; tailLines?: number; previous?: boolean }
  ): Promise<string> => {
    const qs = new URLSearchParams();
    if (params.container) qs.set("container", params.container);
    if (params.tailLines) qs.set("tailLines", String(params.tailLines));
    if (params.previous) qs.set("previous", "1");
    const res = await fetch(
      `/api/v1/namespaces/${encodeURIComponent(ns)}/deployments/${encodeURIComponent(name)}/logs?${qs}`,
      { headers: { Authorization: `Bearer ${token}` } }
    );
    if (!res.ok) {
      const t = await res.text().catch(() => "");
      throw new ApiError(res.status, t || res.statusText);
    }
    return res.text();
  },

  // SSE follow: EventSource cannot set headers, so the token rides on ?token=.
  // Returns the EventSource so the caller can close() to stop following.
  streamLogs: (
    token: string,
    ns: string,
    name: string,
    params: { container?: string; tailLines?: number; previous?: boolean },
    onLine: (line: string) => void,
    onError: (err: string) => void
  ): EventSource => {
    const qs = new URLSearchParams({ token });
    if (params.container) qs.set("container", params.container);
    if (params.tailLines) qs.set("tailLines", String(params.tailLines));
    if (params.previous) qs.set("previous", "1");
    const url = `/api/v1/namespaces/${encodeURIComponent(ns)}/deployments/${encodeURIComponent(name)}/logs/stream?${qs}`;
    const es = new EventSource(url);
    es.addEventListener("log", (e) => {
      try {
        const data = JSON.parse((e as MessageEvent).data) as { line: string };
        onLine(data.line);
      } catch {
        onLine((e as MessageEvent).data);
      }
    });
    es.addEventListener("error", (e) => {
      const me = e as MessageEvent;
      try {
        const data = me.data ? JSON.parse(me.data) : null;
        onError(data?.error || "stream ended");
      } catch {
        onError("stream ended");
      }
      es.close();
    });
    es.onerror = () => {
      // browser-level error (e.g. network drop) — surface once and stop
      es.close();
    };
    return es;
  },

  // ---- admin ----
  adminListNamespaces: async (token: string): Promise<string[]> => {
    const res = await fetch("/api/v1/namespaces", {
      headers: { Authorization: `Bearer ${token}` },
    });
    const text = await res.text();
    const parsed = text ? JSON.parse(text) : null;
    if (!res.ok) {
      const msg =
        (parsed && (parsed as { error?: string }).error) || res.statusText;
      throw new ApiError(res.status, msg);
    }
    return (parsed as { namespaces?: string[] }).namespaces ?? [];
  },

  adminListTokens: (token: string) =>
    request<TokenRecord[]>("GET", "/api/v1/admin/tokens", token),

  adminCreateToken: (
    token: string,
    body: { label: string; scopes: TokenScope[]; expiresAt?: string | null }
  ) => request<TokenRecord>("POST", "/api/v1/admin/tokens", token, body),

  adminDeleteToken: (token: string, id: string) =>
    fetch(`/api/v1/admin/tokens/${encodeURIComponent(id)}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${token}` },
    }).then((res) => {
      if (!res.ok) throw new ApiError(res.status, res.statusText);
    }),
};
