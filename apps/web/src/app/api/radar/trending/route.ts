import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";
import { normalizeCategory } from "@/lib/prehub-data";

export async function GET(request: NextRequest) {
  const category = normalizeCategory(
    request.nextUrl.searchParams.get("category") ?? undefined,
  );
  const window = request.nextUrl.searchParams.get("window") ?? "24h";
  const limit = request.nextUrl.searchParams.get("limit") ?? "50";
  const potential = request.nextUrl.searchParams.get("potential");
  const goResponse = await fetchGoAPI(
    `/v1/radar/trending?category=${encodeURIComponent(category)}&window=${encodeURIComponent(window)}&limit=${encodeURIComponent(limit)}${potential ? `&potential=${encodeURIComponent(potential)}` : ""}`,
  );
  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; Radar data requires the backend." },
    { status: 503 },
  );
}
