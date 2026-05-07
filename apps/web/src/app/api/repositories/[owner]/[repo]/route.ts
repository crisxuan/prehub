import { fetchGoAPI } from "@/lib/go-api";
import { findRepository } from "@/lib/prehub-data";

type RepositoryRouteContext = {
  params: Promise<{ owner: string; repo: string }>;
};

export async function GET(_request: Request, context: RepositoryRouteContext) {
  const { owner, repo } = await context.params;
  const goResponse = await fetchGoAPI(`/v1/repositories/${owner}/${repo}`);
  if (goResponse?.ok) {
    return Response.json(await goResponse.json());
  }

  const repository = findRepository(owner, repo);
  if (!repository) {
    return Response.json({ error: "repository not found" }, { status: 404 });
  }

  return Response.json(repository);
}
