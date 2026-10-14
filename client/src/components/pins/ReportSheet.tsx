"use client";

import { useState } from "react";
import PanelSheet from "@/shared/ui/PanelSheet";
import { SheetHeader, SheetSignIn } from "@/components/pins/SheetChrome";
import { createPinReport } from "@/lib/api/reports";
import { errorMessage } from "@/shared/utils";
import { useAuth } from "@/store/auth";

interface ReportSheetProps {
  pinId: string;
  onClose: () => void;
}

const QUICK_REASONS = ["Inappropriate content", "Spam", "Misleading location"];

/** Report a pin: shown from the pin detail panel / pin page to logged-in users. */
export default function ReportSheet({ pinId, onClose }: ReportSheetProps) {
  const { user } = useAuth();
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  if (!user) {
    return (
      <PanelSheet role="dialog" aria-modal="true" aria-label="Report pin" onClose={onClose}>
        <SheetHeader title="Report this pin" onClose={onClose} />
        <SheetSignIn action="report this pin" />
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
      <SheetHeader title="Report this pin" onClose={onClose} />

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