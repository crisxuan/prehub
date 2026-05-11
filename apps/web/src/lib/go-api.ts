type CachedResponse = {
  body: string;
  contentType: string;
  expiresAt: number;
  status: number;
};

type FetchGoAPIInit = RequestInit & {
  timeoutMs?: number;
};

const defaultAPITimeoutMs = 8_000;
const responseCache = new Map<string, CachedResponse>();

export async function fetchGoAPI(path: string, init?: FetchGoAPIInit) {
  const baseURL = resolveGoAPIBaseURL();
  if (!baseURL) {
    console.warn("PreHub Go API base URL is not configured", { path });
    return null;
  }

  const { timeoutMs, ...requestInit } = init ?? {};
  const method = requestInit.method?.toUpperCase() ?? "GET";
  const cacheKey = `${method}:${path}`;
  const ttlMs = method === "GET" ? cacheTTLForPath(path) : 0;
  if (ttlMs > 0) {
    const cached = responseCache.get(cacheKey);
    if (cached && cached.expiresAt > Date.now()) {
      return cachedResponse(cached);
    }
  }

  const controller = new AbortController();
  const timeout = setTimeout(
    () => controller.abort(),
    timeoutMs ?? timeoutForPath(path, method),
  );
  const upstreamSignal = requestInit.signal;
  if (upstreamSignal) {
    if (upstreamSignal.aborted) {
      controller.abort();
    } else {
      upstreamSignal.addEventListener("abort", () => controller.abort(), {
        once: true,
      });
    }
  }

  const headers = new Headers(requestInit.headers);
  headers.set("x-internal-token", process.env.INTERNAL_API_TOKEN ?? "");

  try {
    const response = await fetch(`${baseURL}${path}`, {
      ...requestInit,
      headers,
      cache: "no-store",
      signal: controller.signal,
    });
    if (ttlMs > 0 && response.ok) {
      const body = await response.text();
      rememberResponse(cacheKey, ttlMs, {
        body,
        contentType: response.headers.get("content-type") ?? "application/json; charset=utf-8",
        expiresAt: Date.now() + ttlMs,
        status: response.status,
      });
      return cachedResponse(responseCache.get(cacheKey)!);
    }
    if (!response.ok) {
      console.warn("PreHub Go API returned a non-OK response", {
        path,
        status: response.status,
        baseHost: safeHost(baseURL),
      });
    }
    return response;
  } catch (error) {
    console.warn("PreHub Go API fetch failed", {
      path,
      baseHost: safeHost(baseURL),
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  } finally {
    clearTimeout(timeout);
  }
}

function resolveGoAPIBaseURL() {
  const explicit = process.env.GO_API_URL ?? process.env.API_URL;
  if (explicit) {
    return explicit.replace(/\/$/, "");
  }
  if (process.env.VERCEL_URL) {
    return `https://${process.env.VERCEL_URL}/api-go`;
  }
  return null;
}

function safeHost(rawURL: string) {
  try {
    return new URL(rawURL).host;
  } catch {
    return "invalid-url";
  }
}

function cacheTTLForPath(path: string) {
  if (path.startsWith("/v1/admin")) {
    return 0;
  }
  if (path.startsWith("/v1/radar/")) {
    return 45_000;
  }
  if (path.startsWith("/v1/daily-picks/")) {
    return 60_000;
  }
  if (path.startsWith("/v1/search")) {
    return 30_000;
  }
  if (path.startsWith("/v1/repositories/")) {
    return 120_000;
  }
  return 0;
}

function timeoutForPath(path: string, method: string) {
  if (method !== "GET") {
    return 25_000;
  }
  if (path.startsWith("/v1/search")) {
    return 15_000;
  }
  if (path.includes("/metrics")) {
    return 12_000;
  }
  return defaultAPITimeoutMs;
}

function rememberResponse(key: string, ttlMs: number, entry: CachedResponse) {
  if (responseCache.size > 100) {
    const oldestKey = responseCache.keys().next().value;
    if (oldestKey) {
      responseCache.delete(oldestKey);
    }
  }
  responseCache.set(key, {
    ...entry,
    expiresAt: Date.now() + ttlMs,
  });
}

function cachedResponse(entry: CachedResponse) {
  return new Response(entry.body, {
    status: entry.status,
    headers: {
      "content-type": entry.contentType,
      "x-prehub-cache": "memory",
    },
  });
}
