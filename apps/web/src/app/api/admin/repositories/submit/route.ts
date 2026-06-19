import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";
import { readRequiredJSON } from "@/lib/request-json";

export async function POST(request: NextRequest) {
  const input = await readRequiredJSON(request);
  if (!input.ok) {
    return input.response;
  }

  const goResponse = await fetchGoAPI("/v1/admin/repositories/submit", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(input.value),
  });

  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; repository submit requires the backend." },
    { status: 503 },
  );
}
