import type { ReactNode } from "react";
import Navbar from "@/components/layout/Navbar";
import LegalFooter from "@/components/LegalFooter";

export function DraftBanner() {
  return (
    <p role="note" className="rounded-xl border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200">
      DRAFT, pending legal review. This text is a starting point, not legal
      advice, and must be reviewed before launch.
    </p>
  );
}

export default function LegalPage({
  title,
  updated,
  children,
}: {
  title: string;
  updated: string;
  children: ReactNode;
}) {
  return (
    <>
      <Navbar backHref="/" backLabel="Back to map" />
      <div className="mx-auto w-full max-w-2xl flex-1 overflow-y-auto px-4 pb-8 pt-20">
        <h1 className="text-2xl font-semibold tracking-tight text-zinc-900 dark:text-zinc-100">
          {title}
        </h1>
        <p className="mt-1 text-xs text-zinc-500 dark:text-zinc-400">Draft updated {updated}</p>
        <div className="mt-4">
          <DraftBanner />
        </div>
        <div className="mt-6 space-y-4 text-sm leading-relaxed text-zinc-700 dark:text-zinc-300">
          {children}
        </div>
        <LegalFooter />
      </div>
    </>
  );
}
