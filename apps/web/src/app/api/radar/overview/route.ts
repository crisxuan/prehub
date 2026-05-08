import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";
import { normalizeCategory } from "@/lib/prehub-data";

export async function GET(request: NextRequest) {
  const category = normalizeCategory(
    request.nextUrl.searchParams.get("category") ?? undefined,
  );
  const window = request.nextUrl.searchParams.get("window") ?? "24h";
  const goResponse = await fetchGoAPI(
    `/v1/radar/overview?category=${encodeURIComponent(category)}&window=${encodeURIComponent(window)}`,
  );
  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; Radar data requires the backend." },
    { status: 503 },
  );
}
