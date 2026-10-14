"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import Navbar from "@/shared/layout/Navbar";
import LegalFooter from "@/shared/ui/LegalFooter";
import { AuthError, AuthField, AuthHero, AuthShell, AuthSubmit } from "@/components/auth/AuthChrome";
import { errorMessage } from "@/shared/utils";
import { login } from "@/lib/api/auth";
import { useAuth } from "@/store/auth";

export default function LoginPage() {
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const { setUser } = useAuth();
  const router = useRouter();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!identifier.trim()) return setError("Enter your email or username");
    if (!password) return setError("Password is required");

    setSubmitting(true);
    try {
      const { user } = await login(identifier.trim(), password);
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
        <AuthHero title="Welcome back" subtitle="Log in to Goodspot" />

        <form onSubmit={submit} className="space-y-4">
          <AuthField
            id="identifier"
            label="Email or username"
            type="text"
            autoComplete="username"
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
            placeholder="you@example.com or yourname"
          />

          <AuthField
            id="password"
            label="Password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />

          <AuthError error={error} />

          <AuthSubmit submitting={submitting} pending="Logging in…">
            Log in
          </AuthSubmit>
        </form>

        <p className="mt-4 text-center text-sm">
          <Link href="/forgot-password" className="font-medium text-zinc-500 hover:underline dark:text-zinc-400">
            Forgot password?
          </Link>
        </p>

        <p className="mt-6 text-center text-sm text-zinc-600 dark:text-zinc-400">
          No account?{" "}
          <Link href="/register" className="font-semibold text-rose-600 hover:underline">
            Sign up
          </Link>
        </p>
        <LegalFooter />
      </AuthShell>
    </>
  );
}