import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";
import { buildSubmittedCandidate } from "@/lib/prehub-data";

export async function POST(request: NextRequest) {
  const body = await request.json();

  const goResponse = await fetchGoAPI("/v1/admin/repositories/submit", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });

  if (goResponse?.ok) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  const candidate = buildSubmittedCandidate(String(body.url ?? ""));
  if (!candidate) {
    return Response.json(
      { error: "expected github.com/{owner}/{repo}" },
      { status: 400 },
    );
  }

  return Response.json(
    {
      status: "queued_mock",
      candidate,
      message: "Go API is not reachable; returning a local candidate preview.",
    },
    { status: 202 },
  );
}
