import { fetchGoAPI } from "@/lib/go-api";
import { bearerTokenMatches } from "@/lib/secrets";

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
  if (!bearerTokenMatches(authHeader, cronSecret)) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  const goResponse = await fetchGoAPI("/v1/admin/daily-picks/generate", {
    method: "POST",
    headers: { "content-type": "application/json" },
  });

  if (!goResponse) {
    return Response.json(
      { error: "Go API is not reachable; Daily pick generation requires the backend." },
      { status: 503 },
    );
  }

  return Response.json(await goResponse.json(), { status: goResponse.status });
}
