import type { Metadata } from "next";
import LegalPage from "../LegalPage";

export const metadata: Metadata = {
  title: "Privacy Policy (Draft) · Goodspot",
  robots: { index: false, follow: false },
};

export default function PrivacyPage() {
  return (
    <LegalPage title="Privacy Policy" updated="2026-10-03">
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">1. Data we collect</h2>
      <p>
        Account data: email address, username, password hash (bcrypt, never
        stored or logged in plain text), avatar, profile links, and role.
        Content you post: pin locations, captions, categories, photos,
        comments, favorites, follows, and collections (including their
        public/private setting). We also record which signed-in account
        viewed which pin (one entry per account and pin, used only to
        count unique views for trending); opening a pin while signed out
        is never recorded, and your own opens of your own pins are not
        counted. Uploaded photos are re-encoded and stripped
        of EXIF location metadata before storage.
      </p>
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">2. Technical data</h2>
      <p>
        A session cookie (`session_token`, httpOnly) keeps you signed in for
        30 days. Server logs record request paths, status codes, latency, IP
        addresses, and request IDs. Browser crash reports (message, stack,
        page URL) are sent to our error endpoint when something breaks. Search
        and map tiles go to MapTiler directly from your browser, which sees
        your IP address and search queries under their policy.
      </p>
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">3. How we use it</h2>
      <p>
        To operate the service (display pins, profiles, and collections),
        keep accounts secure (login, password reset, moderation), and fix
        bugs. We do not sell personal data and show no third-party ads.
      </p>
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">4. Sharing and retention</h2>
      <p>
        Public content is visible to other users by design. Password reset
        tokens expire after 30 minutes and are single-use; resetting signs out
        all other sessions. Deleted pins are hidden from everyone. Backups may
        retain copies until rotated. There is currently no self-serve account
        export or deletion; request either at [FILL IN: contact address].
      </p>
      <h2 className="font-semibold text-zinc-900 dark:text-zinc-100">5. Your rights and contact</h2>
      <p>
        Ask for access, correction, or deletion of your data at [FILL IN:
        contact address]. Data controller: [FILL IN: legal entity and address].
      </p>
    </LegalPage>
  );
}
