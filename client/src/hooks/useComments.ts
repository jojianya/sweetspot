"use client";

import { useCallback, useEffect, useState } from "react";
import { createComment, deleteComment, fetchComments } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import type { Comment } from "@/lib/types";

/** Comment list state for a pin: load/retry, add, and remove. */
export function useComments(pinId: string) {
  const [comments, setComments] = useState<Comment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    fetchComments(pinId)
      .then((data) => {
        if (!cancelled) setComments(data);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorMessage(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [pinId, attempt]);

  const retry = useCallback(() => {
    setError(null);
    setLoading(true);
    setAttempt((n) => n + 1);
  }, []);

  const add = useCallback(
    async (body: string): Promise<Comment> => {
      const comment = await createComment(pinId, body);
      setComments((prev) => [...prev, comment]);
      return comment;
    },
    [pinId]
  );

  const remove = useCallback(async (id: string) => {
    await deleteComment(id);
    setComments((prev) => prev.filter((c) => c.id !== id));
  }, []);

  return { comments, loading, error, retry, add, remove };
}