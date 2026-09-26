// Shown during route transitions. Without this, every navigation is a blank
// flash while the new segment loads.
export default function Loading() {
  return (
    <div className="flex flex-1 items-center justify-center bg-gradient-to-br from-rose-50 via-white to-orange-50 dark:from-rose-950/30 dark:via-zinc-950 dark:to-orange-950">
      <div className="flex flex-col items-center gap-3">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-zinc-300 border-t-rose-600 dark:border-zinc-700 dark:border-t-rose-500" />
        <p className="text-sm text-zinc-500 dark:text-zinc-400">Loading…</p>
      </div>
    </div>
  );
}
