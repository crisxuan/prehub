import { fetchGoAPI } from "@/lib/go-api";
import { adminOverview } from "@/lib/prehub-data";

export async function GET() {
  const goResponse = await fetchGoAPI("/v1/admin/overview");
  if (goResponse?.ok) {
    return Response.json(await goResponse.json());
  }

  return Response.json(adminOverview);
}
