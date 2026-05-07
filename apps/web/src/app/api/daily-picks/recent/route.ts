import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";
import { buildRecentDailyPickHistory, normalizeCategory } from "@/lib/prehub-data";

export async function GET(request: NextRequest) {
  const rawDays = request.nextUrl.searchParams.get("days") ?? "7";
  const days = Number.parseInt(rawDays, 10);
  const safeDays = Number.isFinite(days) && days > 0 ? Math.min(days, 31) : 7;
  const category = normalizeCategory(
    request.nextUrl.searchParams.get("category") ?? undefined,
  );

  const goResponse = await fetchGoAPI(
    `/v1/daily-picks/recent?days=${encodeURIComponent(safeDays)}&category=${encodeURIComponent(category)}`,
  );
  if (goResponse?.ok) {
    return Response.json(await goResponse.json());
  }

  return Response.json({ ...buildRecentDailyPickHistory(safeDays), category });
}
