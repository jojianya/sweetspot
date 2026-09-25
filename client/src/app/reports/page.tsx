"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import Navbar from "@/components/layout/Navbar";
import { fetchReports, reviewReport } from "@/lib/api";
import { errorMessage, relativeTime } from "@/lib/utils";
import { useAuth } from "@/store/auth";
import type { ReportEntry, ReportStatus } from "@/lib/types";

const STATUS_OPTIONS: Array<{ value: ReportStatus | "all"; label: string }> = [
  { value: "pending", label: "Pending" },
  { value: "reviewed", label: "Reviewed" },
  { value: "actioned", label: "Actioned" },
  { value: "all", label: "All" },
];

const STATUS_STYLES: Record<ReportStatus, string> = {
  pending:
    "bg-amber-100 text-amber-700 dark:bg-amber-950/60 dark:text-amber-300",
  reviewed: "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400",
  actioned:
    "bg-rose-100 text-rose-700 dark:bg-rose-950/60 dark:text-rose-300",
};

export default function ReportsPage() {
  const { token, user } = useAuth();
  const isModerator = user?.role === "admin" || user?.role === "owner";

  const [status, setStatus] = useState<ReportStatus | "all">("pending");
  const [reports, setReports] = useState<ReportEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    fetchReports(status === "all" ? {} : { status })
      .then((data) => {
        if (!cancelled) setReports(data);
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
  }, [status, attempt]);

  const selectStatus = (value: ReportStatus | "all") => {
    setStatus(value);
    setLoading(true);
    setError(null);
  };

  const retry = useCallback(() => {
    setError(null);
    setLoading(true);
    setAttempt((n) => n + 1);
  }, []);

  const handleReview = async (report: ReportEntry, action: "approve" | "dismiss") => {
    if (busyId) return;
    setBusyId(report.id);
    setError(null);
    try {
      await reviewReport(report.id, action);
      // Refilter: resolved reports leave the pending tab, others refresh.
      setReports((prev) => prev.filter((r) => r.id !== report.id));
    } catch (e) {
      const msg = errorMessage(e);
      if (/already resolved/i.test(msg)) {
        retry();
      } else {
        setError(msg);
      }
    } finally {
      setBusyId(null);
    }
  };

  if (!token || !user) {
    return (
      <>
        <Navbar backHref="/" backLabel="Back to map" />
        <div className="flex flex-1 flex-col items-center justify-center gap-4 px-4 text-center">
          <h1 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            Sign in to moderate
          </h1>
          <Link
            href="/login"
            className="rounded-full bg-rose-600 px-5 py-2 text-sm font-medium text-white hover:bg-rose-700"
          >
            Log in
          </Link>
        </div>
      </>
    );
  }

  if (!isModerator) {
    return (
      <>
        <Navbar backHref="/" backLabel="Back to map" />
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-4 text-center">
          <h1 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            Admins only
          </h1>
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            You don&apos;t have permission to review reports.
          </p>
          <Link
            href="/"
            className="rounded-full bg-rose-600 px-5 py-2 text-sm font-medium text-white hover:bg-rose-700"
          >
            Back to map
          </Link>
        </div>
      </>
    );
  }

  return (
    <>
      <Navbar backHref="/" backLabel="Back to map" />

      <div className="mx-auto w-full max-w-3xl flex-1 overflow-y-auto px-4 pb-16 pt-20">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="text-xl font-bold text-zinc-900 dark:text-zinc-100">Reports</h1>
          {user.role === "owner" && (
            <Link
              href="/roles"
              className="rounded-full border border-zinc-300 px-4 py-1.5 text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Manage roles
            </Link>
          )}
        </div>

        {/* Status filter */}
        <div className="mt-4 flex flex-wrap gap-1.5" role="tablist" aria-label="Report status">
          {STATUS_OPTIONS.map((opt) => (
            <button
              key={opt.value}
              type="button"
              role="tab"
              aria-selected={status === opt.value}
              onClick={() => selectStatus(opt.value)}
              className={`rounded-full px-3.5 py-1.5 text-sm font-medium transition-colors ${
                status === opt.value
                  ? "bg-zinc-900 text-white dark:bg-zinc-100 dark:text-zinc-900"
                  : "border border-zinc-300 text-zinc-600 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              }`}
            >
              {opt.label}
            </button>
          ))}
        </div>

        {loading && (
          <p className="py-10 text-center text-sm text-zinc-400 dark:text-zinc-500">
            Loading reports…
          </p>
        )}

        {!loading && error && (
          <div className="flex flex-col items-center gap-3 py-10 text-center">
            <p className="text-sm text-rose-600 dark:text-rose-400">{error}</p>
            <button
              type="button"
              onClick={retry}
              className="rounded-full border border-zinc-300 px-5 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
            >
              Retry
            </button>
          </div>
        )}

        {!loading && !error && reports.length === 0 && (
          <div className="rounded-xl border border-dashed border-zinc-300 py-12 px-6 text-center dark:border-zinc-700">
            <p className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
              No {status === "all" ? "" : `${status} `}reports
            </p>
            <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
              {status === "pending"
                ? "You're all caught up."
                : "Try another filter."}
            </p>
          </div>
        )}

        {!loading && !error && reports.length > 0 && (
          <ul className="mt-4 space-y-2.5">
            {reports.map((report) => (
              <li
                key={report.id}
                className="rounded-xl border border-zinc-200/70 p-4 dark:border-zinc-800"
              >
                <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-zinc-500 dark:text-zinc-400">
                  <span className="font-medium text-zinc-700 dark:text-zinc-300">
                    {report.reporter_username ? `@${report.reporter_username}` : "deleted user"}
                  </span>
                  <span className="text-zinc-300 dark:text-zinc-600">reported</span>
                  <Link
                    href={`/pin/${report.pin_id}`}
                    className="max-w-full truncate font-medium text-sky-600 hover:underline dark:text-sky-400"
                  >
                    {report.pin_caption?.trim() || "this pin"}
                  </Link>
                  <span className="text-zinc-300 dark:text-zinc-600">·</span>
                  <time dateTime={report.created_at}>{relativeTime(report.created_at)}</time>
                  <span
                    className={`ml-auto rounded-full px-2 py-0.5 text-xs font-semibold ${STATUS_STYLES[report.status]}`}
                  >
                    {report.status}
                  </span>
                </div>

                <p className="mt-2 whitespace-pre-wrap text-sm text-zinc-700 dark:text-zinc-300">
                  {report.reason}
                </p>

                {report.status === "pending" && (
                  <div className="mt-3 flex gap-2">
                    <button
                      type="button"
                      onClick={() => void handleReview(report, "dismiss")}
                      disabled={busyId !== null}
                      className="rounded-full border border-zinc-300 px-3.5 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                    >
                      Dismiss
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleReview(report, "approve")}
                      disabled={busyId !== null}
                      className="rounded-full bg-rose-600 px-3.5 py-1.5 text-xs font-medium text-white hover:bg-rose-700 disabled:opacity-50"
                    >
                      {busyId === report.id ? "Working…" : "Hide pin"}
                    </button>
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </>
  );
}