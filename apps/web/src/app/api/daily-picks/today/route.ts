import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";
import { normalizeCategory, todayPick } from "@/lib/prehub-data";

export async function GET(request: NextRequest) {
  const category = normalizeCategory(
    request.nextUrl.searchParams.get("category") ?? undefined,
  );
  const goResponse = await fetchGoAPI(
    `/v1/daily-picks/today?category=${encodeURIComponent(category)}`,
  );
  if (goResponse?.ok) {
    return Response.json(await goResponse.json());
  }

  return Response.json({ ...todayPick, category });
}
