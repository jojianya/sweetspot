"use client";

/** Posting-mode crosshair overlay. Pure static markup, no props. */
export function PostingCrosshair() {
  return (
    <div
      className="pointer-events-none absolute left-1/2 top-1/2 z-10 flex -translate-x-1/2 -translate-y-1/2 items-center justify-center"
      aria-hidden
    >
      <div className="absolute h-0.5 w-16 bg-rose-600 shadow-[0_0_4px_rgba(255,255,255,0.9)]" />
      <div className="absolute h-16 w-0.5 bg-rose-600 shadow-[0_0_4px_rgba(255,255,255,0.9)]" />
    </div>
  );
}
