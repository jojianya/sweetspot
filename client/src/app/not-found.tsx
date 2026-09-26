import Link from "next/link";
import Navbar from "@/components/layout/Navbar";

// Reached both by notFound() calls in route segments and by unmatched URLs.
// The root app/not-found.tsx handles both cases (Next.js v13.3+).
export default function NotFound() {
  return (
    <div className="flex flex-1 flex-col bg-gradient-to-br from-rose-50 via-white to-orange-50 dark:from-rose-950/30 dark:via-zinc-950 dark:to-orange-950">
      <Navbar />
      <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
        <p className="text-4xl font-bold text-zinc-900 dark:text-zinc-100">404</p>
        <p className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
          Page not found
        </p>
        <p className="text-sm text-zinc-500 dark:text-zinc-400">
          The page you are looking for does not exist.
        </p>
        <Link
          href="/"
          className="rounded-full bg-gradient-to-r from-rose-600 to-rose-500 px-4 py-2 text-sm font-medium text-white shadow-md shadow-rose-600/25 transition-transform hover:shadow-lg hover:shadow-rose-600/30 active:scale-95"
        >
          Back to map
        </Link>
      </div>
    </div>
  );
}
