import type { Metadata } from "next";
import { notFound } from "next/navigation";
import PinPageClient from "@/components/pins/PinPageClient";
import { getPinServer } from "@/lib/api/server";
import { buildPinMetadata } from "@/lib/metadata/pin";
import { getSiteUrl } from "@/lib/site";

interface PinPageParams {
  params: Promise<{ id: string }>;
}

export async function generateMetadata({ params }: PinPageParams): Promise<Metadata> {
  const { id } = await params;
  const pin = await getPinServer(id);
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
  const pin = await getPinServer(id);
  if (!pin) notFound();

  return <PinPageClient initialPin={pin} />;
}
