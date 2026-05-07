import { fetchGoAPI } from "@/lib/go-api";

export async function GET() {
  const goResponse = await fetchGoAPI("/v1/health");
  const goHealth = goResponse?.ok ? await goResponse.json() : null;

  return Response.json({
    service: "prehub-web",
    status: "ok",
    go: goHealth ?? { status: "unreachable" },
  });
}
