import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";

type RadarMetricsRouteContext = {
  params: Promise<{ owner: string; repo: string }>;
};

export async function GET(
  request: NextRequest,
  context: RadarMetricsRouteContext,
) {
  const { owner, repo } = await context.params;
  const window = request.nextUrl.searchParams.get("window") ?? "24h";
  const goResponse = await fetchGoAPI(
    `/v1/radar/repositories/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/metrics?window=${encodeURIComponent(window)}`,
  );
  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; Radar metrics require the backend." },
    { status: 503 },
  );
}
