import type { Metadata } from "next";
import LegalPage from "../LegalPage";

export const metadata: Metadata = {
  title: "Terms of Service (Draft) · Goodspot",
  robots: { index: false, follow: false },
};

export default function TermsPage() {
  return (
    <LegalPage title="Terms of Service" updated="2026-10-03">
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">1. What Goodspot is</h2>
      <p>
        Goodspot lets you drop pins on a shared map with photos and captions,
        and browse pins other people shared. Pins, collections you leave
        public, comments, and profiles are visible to other users; private
        collections are visible only to you.
      </p>
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">2. Your content</h2>
      <p>
        You keep ownership of the photos and text you post. By posting you
        grant Goodspot a worldwide, non-exclusive license to store, display,
        and distribute that content as part of the service. You can delete your
        pins at any time from the pin editor, which hides them from everyone.
      </p>
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">3. Acceptable use</h2>
      <p>
        Do not post illegal content, harassment, spam, or other people&apos;s
        private information such as exact home addresses or license plates.
        Location pins are public by design: never pin a location you need to
        keep private. Reported content is reviewed under the{" "}
        <a href="/abuse" className="underline">
          abuse and takedown process
        </a>
        .
      </p>
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">4. Accounts</h2>
      <p>
        You are responsible for activity under your account. Use a strong,
        unique password; a forgotten password can be reset by email. We may
        suspend accounts that abuse the service.
      </p>
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">5. Service and liability</h2>
      <p>
        The service is provided as-is, without warranties. Map data, search
        results, and user pins may be inaccurate. To the extent permitted by
        law, Goodspot is not liable for indirect or consequential damages.
      </p>
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">6. Contact</h2>
      <p>Questions about these terms: [FILL IN: contact address].</p>
    </LegalPage>
  );
}
