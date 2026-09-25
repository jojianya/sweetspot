"use client";

import { useState } from "react";
import Link from "next/link";
import PanelSheet from "@/components/PanelSheet";
import { CloseIcon } from "@/components/icons";
import { createPinReport } from "@/lib/api";
import { errorMessage } from "@/lib/utils";
import { useAuth } from "@/store/auth";

interface ReportSheetProps {
  pinId: string;
  onClose: () => void;
}

const QUICK_REASONS = ["Inappropriate content", "Spam", "Misleading location"];

/** Report a pin: shown from the pin detail panel / pin page to logged-in users. */
export default function ReportSheet({ pinId, onClose }: ReportSheetProps) {
  const { token } = useAuth();
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  if (!token) {
    return (
      <PanelSheet role="dialog" aria-modal="true" aria-label="Report pin" onClose={onClose}>
        <header className="flex shrink-0 items-center justify-between border-b border-zinc-100 px-5 py-4 dark:border-zinc-800">
          <h2 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            Report this pin
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="rounded-full p-1.5 text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"
          >
            <CloseIcon />
          </button>
        </header>
        <div className="flex-1 overflow-y-auto p-5 text-sm text-zinc-600 dark:text-zinc-400">
          <p className="mb-3">Sign in to report this pin.</p>
          <Link
            href="/login"
            className="inline-block rounded-lg bg-rose-600 px-4 py-2 font-medium text-white hover:bg-rose-700"
          >
            Log in
          </Link>
        </div>
      </PanelSheet>
    );
  }

  const reasonValid = reason.trim().length >= 3 && reason.length <= 1000;

  const handleSubmit = async () => {
    if (!reasonValid || busy) return;
    setBusy(true);
    setError(null);
    try {
      await createPinReport(pinId, reason.trim());
      setDone(true);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <PanelSheet role="dialog" aria-modal="true" aria-label="Report pin" onClose={onClose}>
      <header className="flex shrink-0 items-center justify-between border-b border-zinc-100 px-5 py-4 dark:border-zinc-800">
        <h2 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
          Report this pin
        </h2>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close"
          className="rounded-full p-1.5 text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"
        >
          <CloseIcon />
        </button>
      </header>

      <div className="flex-1 overflow-y-auto p-5">
        {done ? (
          <div className="text-center">
            <p className="text-sm font-medium text-zinc-900 dark:text-zinc-100">
              Thanks — your report was submitted.
            </p>
            <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
              Our moderators will review it shortly.
            </p>
            <button
              type="button"
              onClick={onClose}
              className="mt-4 rounded-lg bg-rose-600 px-4 py-2 text-sm font-medium text-white hover:bg-rose-700"
            >
              Close
            </button>
          </div>
        ) : (
          <>
            <label
              htmlFor="report-reason"
              className="mb-1.5 block text-sm font-medium text-zinc-700 dark:text-zinc-300"
            >
              Why are you reporting this pin?
            </label>
            <textarea
              id="report-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              maxLength={1000}
              rows={4}
              placeholder="Tell us what&apos;s wrong (at least 3 characters)…"
              className="w-full rounded-lg border border-zinc-300 px-3 py-2 text-sm text-zinc-900 placeholder:text-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
            />

            <div className="mt-2 flex flex-wrap gap-1.5">
              {QUICK_REASONS.map((r) => (
                <button
                  key={r}
                  type="button"
                  onClick={() => setReason(r)}
                  className="rounded-full border border-zinc-300 px-3 py-1 text-xs font-medium text-zinc-600 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                >
                  {r}
                </button>
              ))}
            </div>

            {error && (
              <p className="mt-3 text-sm text-rose-600 dark:text-rose-400" role="alert">
                {error}
              </p>
            )}

            <div className="mt-4 flex gap-2">
              <button
                type="button"
                onClick={onClose}
                className="rounded-lg border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleSubmit}
                disabled={!reasonValid || busy}
                className="flex-1 rounded-lg bg-rose-600 px-4 py-2 text-sm font-medium text-white hover:bg-rose-700 disabled:opacity-50"
              >
                {busy ? "Submitting…" : "Submit report"}
              </button>
            </div>
          </>
        )}
      </div>
    </PanelSheet>
  );
}