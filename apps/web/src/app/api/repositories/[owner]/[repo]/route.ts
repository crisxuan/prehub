import { fetchGoAPI } from "@/lib/go-api";

type RepositoryRouteContext = {
  params: Promise<{ owner: string; repo: string }>;
};

export async function GET(_request: Request, context: RepositoryRouteContext) {
  const { owner, repo } = await context.params;
  const goResponse = await fetchGoAPI(`/v1/repositories/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}`);
  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; repository details require the backend." },
    { status: 503 },
  );
}
