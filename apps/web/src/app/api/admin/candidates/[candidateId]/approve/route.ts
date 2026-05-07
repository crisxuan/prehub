import { fetchGoAPI } from "@/lib/go-api";

type CandidateActionContext = {
  params: Promise<{ candidateId: string }>;
};

export async function POST(_request: Request, context: CandidateActionContext) {
  const { candidateId } = await context.params;
  const goResponse = await fetchGoAPI(
    `/v1/admin/candidates/${encodeURIComponent(candidateId)}/approve`,
    {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({}),
    },
  );

  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; approval requires the database." },
    { status: 503 },
  );
}
