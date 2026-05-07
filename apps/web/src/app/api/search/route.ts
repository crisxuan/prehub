import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";
import { searchRepositories } from "@/lib/prehub-data";

export async function GET(request: NextRequest) {
  const q = request.nextUrl.searchParams.get("q") ?? "";
  const goResponse = await fetchGoAPI(`/v1/search?q=${encodeURIComponent(q)}`);
  if (goResponse?.ok) {
    return Response.json(await goResponse.json());
  }

  return Response.json({
    query: q,
    intent: ["repository-discovery"],
    results: searchRepositories(q),
  });
}
