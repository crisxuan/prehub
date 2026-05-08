export async function fetchGoAPI(path: string, init?: RequestInit) {
  const baseURL = resolveGoAPIBaseURL();
  if (!baseURL) {
    console.warn("PreHub Go API base URL is not configured", { path });
    return null;
  }

  try {
    const response = await fetch(`${baseURL}${path}`, {
      ...init,
      headers: {
        "x-internal-token": process.env.INTERNAL_API_TOKEN ?? "",
        ...(init?.headers ?? {}),
      },
      cache: "no-store",
    });
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
