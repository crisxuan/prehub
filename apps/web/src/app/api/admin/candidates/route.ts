import { fetchGoAPI } from "@/lib/go-api";

export async function GET() {
  const goResponse = await fetchGoAPI("/v1/admin/candidates");
  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; candidates require the backend." },
    { status: 503 },
  );
}
