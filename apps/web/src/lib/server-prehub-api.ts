import {
  adminOverview,
  buildRecentDailyPickHistory,
  candidates,
  defaultCategory,
  findRepository,
  normalizeCategory,
  searchRepositories,
  todayPick,
  type AdminOverview,
  type Candidate,
  type DailyPick,
  type DailyPickHistory,
  type Repository,
} from "@/lib/prehub-data";

async function fetchInternalJSON<T>(path: string): Promise<T | null> {
  const baseURL = process.env.GO_API_URL;
  if (!baseURL) {
    return null;
  }

  try {
    const response = await fetch(`${baseURL}${path}`, {
      headers: {
        "x-internal-token": process.env.INTERNAL_API_TOKEN ?? "",
      },
      cache: "no-store",
    });
    if (!response.ok) {
      return null;
    }
    return (await response.json()) as T;
  } catch {
    return null;
  }
}

export async function getTodayPick(category = defaultCategory): Promise<DailyPick> {
  const normalized = normalizeCategory(category);
  return (
    (await fetchInternalJSON<DailyPick>(
      `/v1/daily-picks/today?category=${encodeURIComponent(normalized)}`,
    )) ?? { ...todayPick, category: normalized }
  );
}

export async function getRecentDailyPicks(
  days = 7,
  category = defaultCategory,
): Promise<DailyPickHistory> {
  const normalized = normalizeCategory(category);
  return (
    (await fetchInternalJSON<DailyPickHistory>(
      `/v1/daily-picks/recent?days=${encodeURIComponent(days)}&category=${encodeURIComponent(normalized)}`,
    )) ?? { ...buildRecentDailyPickHistory(days), category: normalized }
  );
}

export async function getSearchResults(query: string) {
  return (
    (await fetchInternalJSON<{
      query: string;
      intent: string[];
      results: Repository[];
    }>(`/v1/search?q=${encodeURIComponent(query)}`)) ?? {
      query,
      intent: ["repository-discovery"],
      results: searchRepositories(query),
    }
  );
}

export async function getRepository(owner: string, repo: string) {
  return (
    (await fetchInternalJSON<Repository>(
      `/v1/repositories/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}`,
    )) ?? findRepository(owner, repo)
  );
}

export async function getAdminOverview(): Promise<AdminOverview> {
  return (
    (await fetchInternalJSON<AdminOverview>("/v1/admin/overview")) ??
    adminOverview
  );
}

export async function getCandidates(): Promise<Candidate[]> {
  const payload = await fetchInternalJSON<{ candidates: Candidate[] }>(
    "/v1/admin/candidates",
  );
  return payload?.candidates ?? candidates;
}
