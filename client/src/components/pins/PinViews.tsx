import { formatViews } from "@/lib/utils";

/**
 * Plain-text view count shared by the map detail panel and the permalink
 * page so the two surfaces cannot drift apart. Text only, no icon.
 */
export default function PinViews({ views }: { views: number }) {
  return <span>{formatViews(views)}</span>;
}
