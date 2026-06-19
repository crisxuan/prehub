import { NextRequest, NextResponse } from "next/server";
import { timingSafeEqual } from "@/lib/secrets";

const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD;
const COOKIE_NAME = "prehub_admin_auth";
const MAX_AGE = 7 * 24 * 60 * 60;

const enc = new TextEncoder();

async function signHmac(secret: string, data: string): Promise<string> {
  const key = await crypto.subtle.importKey(
    "raw",
    enc.encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const sig = await crypto.subtle.sign("HMAC", key, enc.encode(data));
  return Array.from(new Uint8Array(sig))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

async function verifyCookie(cookieValue: string): Promise<boolean> {
  if (!ADMIN_PASSWORD) return false;

  const parts = cookieValue.split(".");
  if (parts.length !== 3) return false;

  const [, timestamp, signature] = parts;
  const unsigned = `authenticated.${timestamp}`;
  const expectedSignature = await signHmac(ADMIN_PASSWORD, unsigned);

  if (!timingSafeEqual(signature, expectedSignature)) return false;

  const timestampMs = Number.parseInt(timestamp, 10);
  if (!Number.isFinite(timestampMs)) return false;

  const age = Date.now() - timestampMs;
  return age >= 0 && age < MAX_AGE * 1000;
}

export async function proxy(request: NextRequest) {
  const pathname = request.nextUrl.pathname;

  if (pathname.startsWith("/admin") && pathname !== "/admin/login") {
    const cookie = request.cookies.get(COOKIE_NAME)?.value;

    if (!cookie || !(await verifyCookie(cookie))) {
      const loginUrl = new URL("/admin/login", request.url);
      loginUrl.searchParams.set("from", pathname);
      return NextResponse.redirect(loginUrl);
    }
  }

  if (pathname.startsWith("/api/admin") && pathname !== "/api/admin/auth") {
    const cookie = request.cookies.get(COOKIE_NAME)?.value;

    if (!cookie || !(await verifyCookie(cookie))) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/admin/:path*", "/api/admin/:path*"],
};
