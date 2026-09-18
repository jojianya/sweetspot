"use client";

import { useEffect, useRef, useState } from "react";
import { fetchPin } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import type { PinDetail } from "@/lib/types";

interface UsePinDetailOptions {
  onError?: (message: string) => void;
}

export function usePinDetail(id: string | null, options?: UsePinDetailOptions) {
  const [detail, setDetail] = useState<PinDetail | null>(null);
  const onErrorRef = useRef(options?.onError);

  useEffect(() => {
    onErrorRef.current = options?.onError;
  }, [options?.onError]);

  const [prevId, setPrevId] = useState(id);
  if (id !== prevId) {
    setPrevId(id);
    setDetail(null);
  }

  useEffect(() => {
    if (!id) return;

    const ctrl = new AbortController();
    fetchPin(id, ctrl.signal)
      .then((data) => {
        if (!ctrl.signal.aborted) setDetail(data);
      })
      .catch((e: unknown) => {
        if (!ctrl.signal.aborted) {
          onErrorRef.current?.(errorMessage(e));
        }
      });

    return () => ctrl.abort();
  }, [id]);

  return { detail };
}