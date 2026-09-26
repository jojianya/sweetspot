import type { Metadata } from "next";
import { notFound } from "next/navigation";
import PinPageClient from "@/components/pins/PinPageClient";
import { getPinServer } from "@/lib/api/server";
import { buildPinMetadata } from "@/lib/metadata/pin";
import { getSiteUrl } from "@/lib/site";
import { ApiError } from "@/lib/api/client";

interface PinPageParams {
  params: Promise<{ id: string }>;
}

/**
 * Resolves a pin for the page, mapping a 404 to notFound() and letting every
 * other error propagate to error.tsx.
 *
 * getPinServer throws an ApiError with status 404 for a genuine not-found
 * response. Timeouts, 500s, and network failures also throw ApiError (with
 * status 0 for network-level failures) — those propagate to the error
 * boundary rather than crashing the route unhandled.
 */
async function resolvePin(id: string) {
  try {
    return await getPinServer(id);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) {
      return null;
    }
    throw e;
  }
}

export async function generateMetadata({ params }: PinPageParams): Promise<Metadata> {
  const { id } = await params;
  const pin = await resolvePin(id);
  if (!pin) {
    return {
      title: "Pin not found · Goodspot",
      robots: { index: false, follow: false },
    };
  }

  return buildPinMetadata(pin, getSiteUrl(), process.env.NEXT_PUBLIC_API_URL);
}

export default async function PinPage({ params }: PinPageParams) {
  const { id } = await params;
  const pin = await resolvePin(id);
  if (!pin) notFound();

  return <PinPageClient initialPin={pin} />;
}
