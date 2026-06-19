import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";

const RATE_LIMIT_WINDOW_MS = 60_000;
const RATE_LIMIT_MAX = 5;
const RATE_LIMIT_MAP_MAX = 1_000;
const hits = new Map<string, number[]>();

function isRateLimited(ip: string): boolean {
  const now = Date.now();
  const recent = (hits.get(ip) ?? []).filter(
    (t) => now - t < RATE_LIMIT_WINDOW_MS,
  );
  if (recent.length >= RATE_LIMIT_MAX) return true;
  recent.push(now);
  if (hits.size >= RATE_LIMIT_MAP_MAX && !hits.has(ip)) {
    const oldest = hits.keys().next().value;
    if (oldest !== undefined) hits.delete(oldest);
  }
  hits.set(ip, recent);
  return false;
}

const VALID_ACTIONS = new Set(["like", "dislike"]);

export async function POST(request: NextRequest) {
  const ip =
    request.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ?? "unknown";
  if (isRateLimited(ip)) {
    return Response.json({ error: "Too many requests" }, { status: 429 });
  }

  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return Response.json({ error: "Invalid JSON" }, { status: 400 });
  }

  if (typeof body !== "object" || body === null) {
    return Response.json(
      { error: "action and repositoryFullName are required" },
      { status: 400 },
    );
  }

  const input = body as Record<string, unknown>;
  const action = typeof input.action === "string" ? input.action : "";
  const repositoryFullName =
    typeof input.repositoryFullName === "string"
      ? input.repositoryFullName.trim()
      : typeof input.repositoryId === "string"
        ? input.repositoryId.trim()
        : "";
  const context = typeof input.context === "string" ? input.context : undefined;

  if (
    !VALID_ACTIONS.has(action) ||
    !repositoryFullName
  ) {
    return Response.json(
      { error: "action and repositoryFullName are required" },
      { status: 400 },
    );
  }

  const goResponse = await fetchGoAPI("/v1/feedback", {
    method: "POST",
    body: JSON.stringify({ action, repositoryFullName, context }),
  });

  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable" },
    { status: 503 },
  );
}
