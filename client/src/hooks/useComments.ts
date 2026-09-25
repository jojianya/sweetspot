"use client";

import { useCallback } from "react";
import { createComment, deleteComment, fetchComments } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { Comment } from "@/lib/types";

/** Comment list state for a pin: load/retry, add, and remove. */
export function useComments(pinId: string) {
  const { data, loading, error, retry, setData } = useAsyncData<Comment[]>(
    () => fetchComments(pinId),
    [pinId]
  );

  const add = useCallback(
    async (body: string): Promise<Comment> => {
      const comment = await createComment(pinId, body);
      setData((prev) => [...(prev ?? []), comment]);
      return comment;
    },
    [pinId, setData]
  );

  const remove = useCallback(
    async (id: string) => {
      await deleteComment(id);
      setData((prev) => (prev ?? []).filter((c) => c.id !== id));
    },
    [setData]
  );

  return { comments: data ?? [], loading, error, retry, add, remove };
}