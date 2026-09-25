"use client";

import { useState } from "react";
import { fetchPin } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { PinDetail } from "@/lib/types";

interface UsePinDetailOptions {
  onError?: (message: string) => void;
}

export function usePinDetail(id: string | null, options?: UsePinDetailOptions) {
  const { data, setData } = useAsyncData<PinDetail>(
    (signal) => fetchPin(id as string, signal), // enabled guarantees id is non-null
    [id],
    { enabled: !!id, onError: options?.onError }
  );

  // Clear the previous pin's detail as soon as the selection changes so the
  // panel never flashes stale content while the new pin loads.
  const [prevId, setPrevId] = useState(id);
  if (id !== prevId) {
    setPrevId(id);
    setData(null);
  }

  return { detail: data };
}