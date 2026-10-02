import type { Metadata } from "next";
import { Suspense } from "react";
import ResetForm from "./ResetForm";

export const metadata: Metadata = {
  title: "Reset password · Goodspot",
  robots: { index: false, follow: false },
  // The token arrives in the query string; never let it leave as a Referer.
  referrer: "no-referrer",
};

export default function ResetPasswordPage() {
  return (
    <Suspense>
      <ResetForm />
    </Suspense>
  );
}
