import api from "./client";
import { commentSchema } from "./schemas";
import type { Comment } from "@/lib/types";

export async function fetchComments(pinId: string): Promise<Comment[]> {
  const { data } = await api.get<{ comments: unknown }>(`/pins/${pinId}/comments`);
  return commentSchema.array().parse(data.comments);
}

export async function createComment(pinId: string, body: string): Promise<Comment> {
  const { data } = await api.post<{ comment: unknown }>(`/pins/${pinId}/comments`, { body });
  return commentSchema.parse(data.comment);
}

export async function deleteComment(id: string): Promise<void> {
  await api.delete(`/comments/${id}`);
}