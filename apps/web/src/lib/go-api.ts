export async function fetchGoAPI(path: string, init?: RequestInit) {
  const baseURL = resolveGoAPIBaseURL();
  if (!baseURL) {
    return null;
  }

  try {
    return await fetch(`${baseURL}${path}`, {
      ...init,
      headers: {
        "x-internal-token": process.env.INTERNAL_API_TOKEN ?? "",
        ...(init?.headers ?? {}),
      },
      cache: "no-store",
    });
  } catch {
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
