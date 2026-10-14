// Leaf import, not the feature barrel: this is a React Server Component, and
// a barrel re-export would pull the feature's hook modules into the RSC graph,
// which Next.js rejects. See the boundary rule in eslint.config.mjs.
import MapApp from "@/features/map/components/MapApp";

export default async function Home({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  const raw = (await searchParams).category;
  const initialCategory = typeof raw === "string" ? raw : null;

  return <MapApp initialCategory={initialCategory} />;
}
