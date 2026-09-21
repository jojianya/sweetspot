import { notFound } from "next/navigation";
import type { Metadata } from "next";
import PinPageClient from "@/components/pins/PinPageClient";
import { fetchPinServer } from "@/lib/api/server";

interface PinPageParams {
  params: Promise<{ id: string }>;
}

export async function generateMetadata({ params }: PinPageParams): Promise<Metadata> {
  const { id } = await params;
  const pin = await fetchPinServer(id);
  if (!pin) return { title: "Pin not found · Goodspot" };

  const caption = pin.caption?.trim() || "A spot on Goodspot";
  const hero = pin.photos[0]?.photo_url;

  return {
    title: `${caption} · Goodspot`,
    description: `Check out "${caption}"${
      pin.username ? ` by @${pin.username}` : ""
    } on Goodspot — including photos and location.`,
    openGraph: {
      title: `${caption} · Goodspot`,
      description: `"${caption}"${
        pin.username ? ` by @${pin.username}` : ""
      } — shared on Goodspot.`,
      type: "website",
      ...(hero ? { images: [{ url: hero, width: 1200, height: 630 }] } : {}),
    },
    twitter: {
      card: hero ? "summary_large_image" : "summary",
      title: `${caption} · Goodspot`,
      ...(hero ? { images: [hero] } : {}),
    },
    alternates: { canonical: `/pin/${pin.id}` },
  };
}

export default async function PinPage({ params }: PinPageParams) {
  const { id } = await params;
  const pin = await fetchPinServer(id);
  if (!pin) notFound();

  return <PinPageClient initialPin={pin} />;
}