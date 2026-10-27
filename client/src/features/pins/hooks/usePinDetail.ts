"use client";

import { fetchPin } from "../api/pins";
import { useAsyncData } from "@/shared/hooks/useAsyncData";
import { useClearOnChange } from "@/shared/hooks/useClearOnChange";
import type { PinDetail } from "@/shared/types/pin";

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
  useClearOnChange(id, () => setData(null));

  return { detail: data };
}