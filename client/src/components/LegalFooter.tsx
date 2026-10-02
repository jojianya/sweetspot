import Link from "next/link";

/** Minimal footer linking the legal pages. Rendered on auth and legal pages. */
export default function LegalFooter() {
  return (
    <nav aria-label="Legal" className="flex items-center justify-center gap-4 py-4 text-xs text-zinc-500 dark:text-zinc-400">
      <Link href="/terms" className="hover:underline">
        Terms
      </Link>
      <span aria-hidden="true">·</span>
      <Link href="/privacy" className="hover:underline">
        Privacy
      </Link>
      <span aria-hidden="true">·</span>
      <Link href="/abuse" className="hover:underline">
        Report abuse
      </Link>
    </nav>
  );
}
