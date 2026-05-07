export async function fetchGoAPI(path: string, init?: RequestInit) {
  const baseURL = process.env.GO_API_URL;
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
