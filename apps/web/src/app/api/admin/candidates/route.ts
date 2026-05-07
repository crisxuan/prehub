import { fetchGoAPI } from "@/lib/go-api";
import { candidates } from "@/lib/prehub-data";

export async function GET() {
  const goResponse = await fetchGoAPI("/v1/admin/candidates");
  if (goResponse?.ok) {
    return Response.json(await goResponse.json());
  }

  return Response.json({ candidates });
}
