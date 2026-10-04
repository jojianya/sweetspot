import type { Metadata } from "next";
import { notFound } from "next/navigation";
import PinPageClient from "@/components/pins/PinPageClient";
import { getPinServer } from "@/lib/api/server";
import { buildPinMetadata } from "@/lib/metadata/pin";
import { getSiteUrl } from "@/lib/site";

interface PinPageParams {
  params: Promise<{ id: string }>;
}

/**
 * Resolves a pin for the page: null means the API answered 404, and anything
 * else propagates to error.tsx.
 *
 * getPinServer already maps a 404 response to null (see lib/api/server.ts), so
 * there is no ApiError to inspect here: a missing or hidden pin arrives as null,
 * and timeouts, 500s and network failures arrive as thrown errors that belong
 * in the error boundary. The previous ApiError 404 branch was unreachable —
 * getPinServer never threw ApiError — which meant a genuine 404 could only be
 * handled by accident.
 */
async function resolvePin(id: string) {
  return getPinServer(id);
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
