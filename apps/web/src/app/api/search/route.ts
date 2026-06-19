import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";

export async function GET(request: NextRequest) {
  const q = request.nextUrl.searchParams.get("q") ?? "";
  const page = request.nextUrl.searchParams.get("page");
  const limit = request.nextUrl.searchParams.get("limit");

  let url = `/v1/search?q=${encodeURIComponent(q)}`;
  if (page) url += `&page=${encodeURIComponent(page)}`;
  if (limit) url += `&limit=${encodeURIComponent(limit)}`;

  const goResponse = await fetchGoAPI(url);
  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; search requires the backend." },
    { status: 503 },
  );
}
