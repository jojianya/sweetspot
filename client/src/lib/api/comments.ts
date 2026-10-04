import api from "./client";
import { commentSchema } from "./schemas";
import type { Comment } from "@/lib/types";

/**
 * GET /pins/:id/comments — one page of comments plus the total number of
 * visible comments, so a caller can tell a full list from a truncated one.
 */
export async function fetchComments(
  pinId: string,
  options: { limit?: number; offset?: number } = {}
): Promise<{ comments: Comment[]; total: number }> {
  const { data } = await api.get<{ comments: unknown; total?: unknown }>(`/pins/${pinId}/comments`, {
    params: options,
  });
  return {
    comments: commentSchema.array().parse(data.comments),
    total: typeof data.total === "number" ? data.total : 0,
  };
}

export async function createComment(pinId: string, body: string): Promise<Comment> {
  const { data } = await api.post<{ comment: unknown }>(`/pins/${pinId}/comments`, { body });
  return commentSchema.parse(data.comment);
}

export async function deleteComment(id: string): Promise<void> {
  await api.delete(`/comments/${id}`);
}