import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";
import { candidates } from "@/lib/prehub-data";

export async function POST(request: NextRequest) {
  const body = await request.json();
  const goResponse = await fetchGoAPI("/v1/admin/recrawl", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });

  if (goResponse?.ok) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    {
      status: "queued_mock",
      query: body.query ?? "",
      candidates,
      message: "Go API is not reachable; returning seeded candidates.",
    },
    { status: 202 },
  );
}
