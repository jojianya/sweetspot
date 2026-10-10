"use client";

import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { reactToPin, unreactToPin } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import { useAuth } from "@/store/auth";

interface ReactionState {
  reacted: boolean;
  count: number;
}

/**
 * Optimistic "Good spot" toggle for a pin, modelled directly on
 * hooks/useOptimisticSave.ts: logged-out callers are sent to /login, logged-in
 * callers flip immediately and roll back with a visible reason on failure.
 *
 * The starting state comes from the pin's own `reacted_by_me` /
 * `good_spot_count`, which only GET /pins/:id supplies — so this hook is for the
 * detail panel, not for list rows or map markers, neither of which knows the
 * viewer.
 *
 * After a successful call the server's count is the truth: the optimistic number
 * is a guess made before the round trip, and the response is authoritative.
 */
export function useReaction(pin: {
  id: string;
  reactedByMe?: boolean;
  goodSpotCount: number;
}) {
  const router = useRouter();
  const { user } = useAuth();

  const [state, setState] = useState<ReactionState>({
    reacted: pin.reactedByMe ?? false,
    count: pin.goodSpotCount,
  });
  const [busy, setBusy] = useState(false);
  // Ref-based in-flight guard: `busy` only updates on the next render, so two
  // clicks in the same tick would both pass the state check and send duplicate
  // requests. The ref flips synchronously instead.
  const busyRef = useRef(false);
  const [error, setError] = useState<string | null>(null);

  const toggle = () => {
    if (!user) {
      router.push("/login");
      return;
    }
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    setError(null);

    const next = !state.reacted;
    const optimistic: ReactionState = {
      reacted: next,
      count: Math.max(0, state.count + (next ? 1 : -1)),
    };
    setState(optimistic);

    const op = next ? reactToPin(pin.id) : unreactToPin(pin.id);
    const done = () => {
      busyRef.current = false;
      setBusy(false);
    };

    op.then((result) => {
      // The server's count wins: the optimistic one was a guess, and the count
      // is a shared number that other accounts are moving at the same time.
      setState({ reacted: result.reacted, count: result.good_spot_count });
      done();
    }).catch((e: unknown) => {
      // Roll the badge back and say why: a silent revert left the user staring
      // at a reaction that did not land with no explanation.
      setState({ reacted: !next, count: state.count });
      setError(errorMessage(e));
      done();
    });
  };

  return { ...state, busy, error, toggle };
}
