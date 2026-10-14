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
