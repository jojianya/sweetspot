"use client";

import { useState } from "react";
import Link from "next/link";
import Avatar from "@/components/Avatar";
import { useComments } from "@/hooks/useComments";
import { errorMessage, relativeTime } from "@/lib/utils";
import { useAuth } from "@/store/auth";
import type { Comment } from "@/lib/types";

interface CommentsSectionProps {
  pinId: string;
  /** Compact styling for the map detail sheet (no nested page chrome). */
  compact?: boolean;
}

export default function CommentsSection({ pinId, compact = false }: CommentsSectionProps) {
  const { comments, loading, error, retry, add, remove } = useComments(pinId);
  const { token, user } = useAuth();
  const [body, setBody] = useState("");
  const [posting, setPosting] = useState(false);
  const [postError, setPostError] = useState<string | null>(null);

  const canModerate =
    user?.role === "admin" || user?.role === "owner";

  const canDelete = (c: Comment) =>
    (user !== null && user.id === c.user_id) || canModerate;

  const submit = async () => {
    const text = body.trim();
    if (!text || posting) return;
    setPosting(true);
    setPostError(null);
    try {
      await add(text);
      setBody("");
    } catch (e) {
      setPostError(errorMessage(e));
    } finally {
      setPosting(false);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await remove(id);
    } catch {
      // removal errors surface through a re-fetch next mount; keep it quiet
    }
  };

  return (
    <section aria-label="Comments" className="flex flex-col">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
          Comments
          {!loading && comments.length > 0 && (
            <span className="ml-1.5 text-xs font-medium text-zinc-400">
              {comments.length}
            </span>
          )}
        </h3>
        {!token && (
          <Link
            href="/login"
            className="text-xs font-medium text-sky-600 hover:underline dark:text-sky-400"
          >
            Log in to comment
          </Link>
        )}
      </div>

      {loading && (
        <p className="py-3 text-center text-sm text-zinc-400 dark:text-zinc-500">
          Loading comments…
        </p>
      )}

      {!loading && error && (
        <div className="flex items-center gap-2 py-3 text-sm text-rose-600 dark:text-rose-400">
          <span>{error}</span>
          <button
            type="button"
            onClick={retry}
            className="font-semibold underline"
          >
            Retry
          </button>
        </div>
      )}

      {!loading && !error && comments.length === 0 && (
        <p className="py-3 text-sm text-zinc-400 dark:text-zinc-500">
          No comments yet.
        </p>
      )}

      <ul className={`space-y-3 ${compact ? "max-h-52 overflow-y-auto pr-1" : ""}`}>
        {comments.map((c) => (
          <li key={c.id} className="flex items-start gap-2.5">
            <Avatar
              src={c.avatar_url}
              username={c.username ?? "?"}
              className="h-8 w-8"
              fallbackClassName="bg-zinc-100 text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400"
            />
            <div className="min-w-0 flex-1">
              <div className="flex items-baseline gap-2">
                <Link
                  href={c.user_id ? `/users/${c.user_id}` : "#"}
                  className="truncate text-sm font-semibold text-zinc-900 dark:text-zinc-100"
                >
                  {c.username ?? "deleted user"}
                </Link>
                <span className="shrink-0 text-xs text-zinc-400 dark:text-zinc-500">
                  {relativeTime(c.created_at)}
                </span>
              </div>
              <p className="mt-0.5 whitespace-pre-wrap break-words text-sm leading-relaxed text-zinc-700 dark:text-zinc-300">
                {c.body}
              </p>
            </div>
            {canDelete(c) && (
              <button
                type="button"
                onClick={() => handleDelete(c.id)}
                aria-label="Delete comment"
                className="shrink-0 rounded-full p-1 text-zinc-300 transition-colors hover:bg-zinc-100 hover:text-rose-500 dark:text-zinc-600 dark:hover:bg-zinc-800 dark:hover:text-rose-400"
              >
                <svg
                  className="h-3.5 w-3.5"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth={2}
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden
                >
                  <path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6M10 11v6M14 11v6" />
                </svg>
              </button>
            )}
          </li>
        ))}
      </ul>

      {token && (
        <div className="mt-3 border-t border-zinc-100 pt-3 dark:border-zinc-800">
          <div className="flex items-end gap-2">
            <textarea
              value={body}
              onChange={(e) => setBody(e.target.value)}
              rows={compact ? 1 : 2}
              maxLength={500}
              placeholder="Add a comment…"
              className="min-w-0 flex-1 resize-none rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-sky-500 focus:outline-none dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
            />
            <button
              type="button"
              onClick={submit}
              disabled={!body.trim() || posting}
              className="shrink-0 rounded-lg bg-sky-600 px-3.5 py-2 text-sm font-medium text-white transition-colors hover:bg-sky-700 disabled:opacity-50"
            >
              {posting ? "…" : "Post"}
            </button>
          </div>
          {postError && (
            <p className="mt-2 text-xs text-rose-600 dark:text-rose-400" role="alert">
              {postError}
            </p>
          )}
        </div>
      )}
    </section>
  );
}