import { fetchGoAPI } from "@/lib/go-api";

export const dynamic = "force-dynamic";
export const maxDuration = 300;

export async function GET(request: Request) {
  const cronSecret = process.env.CRON_SECRET;
  if (!cronSecret) {
    return Response.json(
      { error: "CRON_SECRET is not configured" },
      { status: 500 },
    );
  }

  const authHeader = request.headers.get("authorization");
  if (authHeader !== `Bearer ${cronSecret}`) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  const goResponse = await fetchGoAPI("/v1/admin/radar/backfill", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      category: "all",
      windows: ["1h", "24h", "7d", "30d"],
      batchSize: 250,
    }),
  });

  if (!goResponse) {
    return Response.json(
      { error: "Go API is not reachable; Radar backfill requires the backend." },
      { status: 503 },
    );
  }

  return Response.json(await goResponse.json(), { status: goResponse.status });
}
