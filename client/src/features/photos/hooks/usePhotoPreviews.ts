"use client";

import { useEffect, useMemo } from "react";

/**
 * Object-URL previews for picked files, revoked when the selection changes
 * or the picker unmounts. Shared by the create and edit photo pickers so a
 * dropped selection never leaks blob URLs.
 */
export function usePhotoPreviews(files: File[]): string[] {
  const previews = useMemo(() => files.map((f) => URL.createObjectURL(f)), [files]);

  useEffect(() => {
    return () => {
      previews.forEach((u) => URL.revokeObjectURL(u));
    };
  }, [previews]);

  return previews;
}
