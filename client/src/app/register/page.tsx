"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import Navbar from "@/shared/layout/Navbar";
import { AuthError, AuthField, AuthHero, AuthShell, AuthSubmit } from "@/components/auth/AuthChrome";
import { errorMessage } from "@/shared/utils";
import { checkPassword } from "@/lib/password";
import { checkUsername } from "@/features/profile/lib/username";
import { register } from "@/lib/api/auth";
import { useAuth } from "@/store/auth";

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function RegisterPage() {
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const { setUser } = useAuth();
  const router = useRouter();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!EMAIL_RE.test(email.trim())) return setError("Enter a valid email");
    // Mirrors the server's rules (see lib/username.ts) so the problem is named
    // before the request rather than after it.
    const usernameCheck = checkUsername(username);
    if (!usernameCheck.ok) return setError(usernameCheck.message);
    // Mirrors the server's rules, including the byte-based maximum. A
    // character-count check would let a long non-ASCII password through and
    // then fail server-side with a 400.
    const passwordCheck = checkPassword(password);
    if (!passwordCheck.ok) return setError(passwordCheck.message);

    setSubmitting(true);
    try {
      const { user } = await register(email.trim(), username.trim(), password);
      setUser(user);
      router.push("/");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <Navbar backHref="/" />
      <AuthShell>
        <AuthHero title="Create your account" subtitle="Join the map and share spots" />

        <form onSubmit={submit} className="space-y-4">
          <AuthField
            id="email"
            label="Email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.com"
          />

          <AuthField
            id="username"
            label="Username"
            type="text"
            autoComplete="nickname"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="myname"
          />

          <AuthField
            id="password"
            label="Password"
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            hint="At least 8 characters"
          />

          <AuthError error={error} />

          <AuthSubmit submitting={submitting} pending="Creating account…">
            Sign up
          </AuthSubmit>
        </form>

        <p className="mt-6 text-center text-sm text-zinc-600 dark:text-zinc-400">
          Already have an account?{" "}
          <Link href="/login" className="font-semibold text-rose-600 hover:underline">
            Log in
          </Link>
        </p>
      </AuthShell>
    </>
  );
}