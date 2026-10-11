"use client";

import { useReaction } from "@/hooks/useReaction";
import { GoodSpotIcon } from "@/components/icons";

interface GoodSpotButtonProps {
  /** The pin this button acts on. `reactedByMe` is optional because the same
   *  component is used where the viewer is unknown. */
  pin: { id: string; reactedByMe?: boolean; goodSpotCount: number };
  /** True when the viewer is the pin's author. The author cannot react to their
   *  own pin, so the control is replaced by plain text rather than a disabled
   *  button — a disabled button implies an action exists. */
  isOwnPin: boolean;
}

/**
 * "Good spot" toggle for the pin detail panel.
 *
 * Accessibility mirrors the Save button: a real <button>, aria-pressed for the
 * on/off state, an aria-label that carries the count so a screen reader hears
 * "Good spot, 12" rather than an icon's worth of meaning, and disabled while a
 * call is in flight so a double tap cannot race itself.
 *
 * The failure message is rendered by the caller's panel (role="alert"), matching
 * how Save reports its own errors.
 */
export default function GoodSpotButton({ pin, isOwnPin }: GoodSpotButtonProps) {
  const { reacted, count, busy, error, toggle } = useReaction(pin);

  if (isOwnPin) {
    // Plain text, not a disabled button: the action does not exist for the
    // author, so offering a control that cannot work is worse than not offering
    // one. The count is still shown, because the author can see it everywhere
    // else.
    return (
      <span className="flex flex-col items-center gap-1.5">
        <span className="flex h-11 items-center justify-center text-sm font-medium text-zinc-700 dark:text-zinc-300">
          {count} {count === 1 ? "good spot" : "good spots"}
        </span>
      </span>
    );
  }

  return (
    <div className="flex flex-col items-center gap-1.5">
      <button
        type="button"
        onClick={toggle}
        disabled={busy}
        className="flex h-11 w-11 items-center justify-center rounded-full bg-amber-500/10 text-amber-600 transition-colors hover:bg-amber-500/20 disabled:opacity-40 dark:bg-amber-500/20 dark:text-amber-400"
        aria-label={`Good spot, ${count}`}
        aria-pressed={reacted}
      >
        <GoodSpotIcon filled={reacted} />
      </button>
      <span className="text-xs font-medium text-zinc-700 dark:text-zinc-300">
        {reacted ? "Good spot" : count === 1 ? "1 good spot" : `${count} good spots`}
      </span>
      {error && (
        <span role="alert" className="max-w-20 text-center text-xs text-rose-600 dark:text-rose-400">
          {error}
        </span>
      )}
    </div>
  );
}
