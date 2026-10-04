import type { Metadata } from "next";
import LegalPage from "../LegalPage";

export const metadata: Metadata = {
  title: "Report abuse (Draft) · Goodspot",
  robots: { index: false, follow: false },
};

export default function AbusePage() {
  return (
    <LegalPage title="Report abuse and takedowns" updated="2026-10-03">
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">1. How to report</h2>
      <p>
        Use the Report button on any pin, or write to [FILL IN: contact
        address] with a link to the content and why it violates the rules.
        Reports are visible only to moderators.
      </p>
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">2. Photo and location policy</h2>
      <p>
        Photos must be yours to share or lawfully shared. Do not post
        other people&apos;s private locations, exact home addresses, license
        plates, or images of people who have not consented where consent is
        required. Illegal content, harassment, and spam are removed.
      </p>
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">3. How reports are handled</h2>
      <p>
        Moderators review pending reports in the order received. Approved
        reports hide the pin from everyone immediately; dismissed reports leave
        it up. Repeat or severe violations lead to account suspension. We aim
        to act within [FILL IN: response time, e.g. 7 days]; illegal content
        is prioritized.
      </p>
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">4. Appeals</h2>
      <p>
        If your content was removed and you believe that was a mistake, contact
        [FILL IN: contact address] with a link to the pin and we will take a
        second look.
      </p>
    </LegalPage>
  );
}
