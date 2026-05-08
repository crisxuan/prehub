import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";

export async function POST(request: NextRequest) {
  const body = await request.json();
  const goResponse = await fetchGoAPI("/v1/admin/radar/watchlist", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });

  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; Radar watchlist requires the database." },
    { status: 503 },
  );
}
