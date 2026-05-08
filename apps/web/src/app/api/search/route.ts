import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";

export async function GET(request: NextRequest) {
  const q = request.nextUrl.searchParams.get("q") ?? "";
  const goResponse = await fetchGoAPI(`/v1/search?q=${encodeURIComponent(q)}`);
  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; search requires the backend." },
    { status: 503 },
  );
}
