import { NextRequest } from "next/server";
import { fetchGoAPI } from "@/lib/go-api";
import { readOptionalJSON } from "@/lib/request-json";

type CandidateActionContext = {
  params: Promise<{ candidateId: string }>;
};

export async function POST(
  request: NextRequest,
  context: CandidateActionContext,
) {
  const { candidateId } = await context.params;
  const input = await readOptionalJSON(request);
  if (!input.ok) {
    return input.response;
  }

  const goResponse = await fetchGoAPI(
    `/v1/admin/candidates/${encodeURIComponent(candidateId)}/publish`,
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(input.value),
    },
  );

  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; publishing requires the database." },
    { status: 503 },
  );
}
