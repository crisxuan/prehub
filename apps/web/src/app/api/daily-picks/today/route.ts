import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";
import { normalizeCategory } from "@/lib/prehub-data";

export async function GET(request: NextRequest) {
  const category = normalizeCategory(
    request.nextUrl.searchParams.get("category") ?? undefined,
  );
  const goResponse = await fetchGoAPI(
    `/v1/daily-picks/today?category=${encodeURIComponent(category)}`,
  );
  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; daily pick requires the backend." },
    { status: 503 },
  );
}
