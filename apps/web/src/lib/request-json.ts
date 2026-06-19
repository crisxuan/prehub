type ReadJSONResult =
  | { ok: true; value: unknown }
  | { ok: false; response: Response };

export async function readRequiredJSON(request: Request): Promise<ReadJSONResult> {
  const text = await request.text();
  if (text.trim() === "") {
    return invalidJSONResponse();
  }

  try {
    return { ok: true, value: JSON.parse(text) };
  } catch {
    return invalidJSONResponse();
  }
}

export async function readOptionalJSON(request: Request): Promise<ReadJSONResult> {
  const text = await request.text();
  if (text.trim() === "") {
    return { ok: true, value: {} };
  }

  try {
    return { ok: true, value: JSON.parse(text) };
  } catch {
    return invalidJSONResponse();
  }
}

function invalidJSONResponse(): ReadJSONResult {
  return {
    ok: false,
    response: Response.json({ error: "Invalid JSON" }, { status: 400 }),
  };
}
