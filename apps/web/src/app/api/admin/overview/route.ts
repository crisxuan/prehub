import { fetchGoAPI } from "@/lib/go-api";

export async function GET() {
  const goResponse = await fetchGoAPI("/v1/admin/overview");
  if (goResponse) {
    return Response.json(await goResponse.json(), { status: goResponse.status });
  }

  return Response.json(
    { error: "Go API is not reachable; admin overview requires the backend." },
    { status: 503 },
  );
}
