import { NextRequest, NextResponse } from "next/server";
import { createHmac, timingSafeEqual } from "node:crypto";

const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD;
const COOKIE_NAME = "prehub_admin_auth";
const MAX_AGE = 7 * 24 * 60 * 60; // 7 days

function signCookie(value: string): string {
  if (!ADMIN_PASSWORD) {
    throw new Error("ADMIN_PASSWORD is not configured");
  }
  const timestamp = Date.now();
  const unsigned = `${value}.${timestamp}`;
  const signature = createHmac("sha256", ADMIN_PASSWORD).update(unsigned).digest("hex");
  return `${unsigned}.${signature}`;
}

function safeEqual(a: string, b: string): boolean {
  const left = Buffer.from(a);
  const right = Buffer.from(b);
  return left.length === right.length && timingSafeEqual(left, right);
}

export async function POST(request: NextRequest) {
  if (!ADMIN_PASSWORD) {
    return Response.json({ error: "Admin authentication not configured" }, { status: 500 });
  }

  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return Response.json({ error: "Invalid JSON" }, { status: 400 });
  }

  const password =
    typeof body === "object" &&
    body !== null &&
    typeof (body as Record<string, unknown>).password === "string"
      ? (body as Record<string, string>).password
      : "";

  if (!safeEqual(password, ADMIN_PASSWORD)) {
    return Response.json({ error: "Invalid password" }, { status: 401 });
  }

  const signedCookie = signCookie("authenticated");

  const response = NextResponse.json({ ok: true });
  response.cookies.set(COOKIE_NAME, signedCookie, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    maxAge: MAX_AGE,
    path: "/",
  });

  return response;
}

export async function DELETE() {
  const response = NextResponse.json({ ok: true });
  response.cookies.delete(COOKIE_NAME);
  return response;
}
