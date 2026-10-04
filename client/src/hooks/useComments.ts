"use client";

import { useCallback } from "react";
import { createComment, deleteComment, fetchComments } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { Comment } from "@/lib/types";

/** Matches the server's maximum page size for a pin's comments. */
const COMMENT_PAGE_SIZE = 200;

/** Comment list state for a pin: load/retry, add, and remove. */
export function useComments(pinId: string) {
  // The endpoint is paginated now, so ask for a page large enough to cover
  // the thread and unwrap the {comments, total} envelope.
  const { data, loading, error, retry, setData } = useAsyncData<Comment[]>(
    async () => (await fetchComments(pinId, { limit: COMMENT_PAGE_SIZE })).comments,
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