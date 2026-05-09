import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";
import { normalizeCategory } from "@/lib/prehub-data";

const allowedWindows = new Set(["1h", "24h", "7d", "30d"]);

export const dynamic = "force-dynamic";
export const maxDuration = 300;

export async function POST(request: NextRequest) {
  const secret = process.env.SCHEDULER_SECRET ?? process.env.CRON_SECRET;
  if (!secret) {
    return Response.json(
      { error: "SCHEDULER_SECRET or CRON_SECRET is not configured" },
      { status: 500 },
    );
  }

  const authHeader = request.headers.get("authorization");
  if (authHeader !== `Bearer ${secret}`) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  const input = await readJSON(request);
  if (!input.ok) {
    return Response.json({ error: input.error }, { status: 400 });
  }

  const payload = normalizeSchedulerPayload(input.value);
  if (!payload.ok) {
    return Response.json({ error: payload.error }, { status: 400 });
  }

  const goResponse = await fetchGoAPI("/v1/admin/radar/backfill", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(payload.value),
  });

  if (!goResponse) {
    return Response.json(
      { error: "Go API is not reachable; Radar backfill requires the backend." },
      { status: 503 },
    );
  }

  return Response.json(await goResponse.json(), { status: goResponse.status });
}

async function readJSON(request: NextRequest) {
  try {
    return { ok: true as const, value: await request.json() };
  } catch {
    return { ok: false as const, error: "Request body must be valid JSON" };
  }
}

function normalizeSchedulerPayload(raw: unknown) {
  if (!raw || typeof raw !== "object") {
    return { ok: false as const, error: "Request body must be an object" };
  }
  const input = raw as Record<string, unknown>;
  const windows = normalizeWindows(input.window, input.windows);
  if (windows.length === 0) {
    return { ok: false as const, error: "window/windows must contain one of 1h, 24h, 7d, 30d" };
  }

  const shards = safeInteger(input.shards, 1);
  const shard = safeInteger(input.shard, 0);
  if (shards < 1 || shards > 128) {
    return { ok: false as const, error: "shards must be between 1 and 128" };
  }
  if (shard < 0 || shard >= shards) {
    return { ok: false as const, error: "shard must be >= 0 and < shards" };
  }

  return {
    ok: true as const,
    value: {
      category: normalizeCategory(asString(input.category) || "ai"),
      windows,
      shard,
      shards,
      batchSize: clampInteger(input.batchSize, 100, 1, 250),
      limit: clampInteger(input.limit, 0, 0, 500),
    },
  };
}

function normalizeWindows(window: unknown, windows: unknown) {
  const rawWindows = [
    asString(window),
    ...(Array.isArray(windows) ? windows.map(asString) : []),
  ];
  const seen = new Set<string>();
  const result: string[] = [];
  for (const item of rawWindows) {
    const normalized = item.toLowerCase().trim();
    if (!allowedWindows.has(normalized) || seen.has(normalized)) {
      continue;
    }
    seen.add(normalized);
    result.push(normalized);
  }
  return result;
}

function asString(value: unknown) {
  return typeof value === "string" ? value : "";
}

function safeInteger(value: unknown, fallback: number) {
  const parsed =
    typeof value === "number"
      ? value
      : typeof value === "string"
        ? Number.parseInt(value, 10)
        : Number.NaN;
  return Number.isFinite(parsed) ? Math.trunc(parsed) : fallback;
}

function clampInteger(
  value: unknown,
  fallback: number,
  minimum: number,
  maximum: number,
) {
  return Math.min(Math.max(safeInteger(value, fallback), minimum), maximum);
}
