"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import Navbar from "@/components/layout/Navbar";
import { errorMessage } from "@/lib/utils";
import { checkPassword } from "@/lib/password";
import { register } from "@/lib/api";
import { useAuth } from "@/store/auth";

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function RegisterPage() {
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const { setAuth } = useAuth();
  const router = useRouter();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!EMAIL_RE.test(email.trim())) return setError("Enter a valid email");
    if (username.trim().length < 3)
      return setError("Username must be at least 3 characters");
    // Mirrors the server's rules, including the byte-based maximum. A
    // character-count check would let a long non-ASCII password through and
    // then fail server-side with a 400.
    const passwordCheck = checkPassword(password);
    if (!passwordCheck.ok) return setError(passwordCheck.message);

    setSubmitting(true);
    try {
      const { user, token } = await register(
        email.trim(),
        username.trim(),
        password
      );
      setAuth(user, token);
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
      <div className="flex flex-1 items-center justify-center overflow-hidden bg-gradient-to-br from-rose-50 via-white to-orange-50 px-4 py-12 dark:from-rose-950/30 dark:via-zinc-950 dark:to-orange-950/30">
      <div className="w-full max-w-sm rounded-2xl border border-zinc-200/60 bg-white/80 p-8 shadow-xl shadow-zinc-900/5 backdrop-blur dark:border-zinc-700/60 dark:bg-zinc-900/80 dark:shadow-zinc-950/40">
        <div className="mb-6 text-center">
          <span className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-rose-500 to-rose-700 text-white shadow-md shadow-rose-600/30">
            <svg className="h-6 w-6" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 2a7 7 0 0 0-7 7c0 5.25 7 13 7 13s7-7.75 7-13a7 7 0 0 0-7-7Zm0 9.5A2.5 2.5 0 1 1 12 6.5a2.5 2.5 0 0 1 0 5Z" />
            </svg>
          </span>
          <h1 className="text-2xl font-semibold tracking-tight text-zinc-900 dark:text-zinc-100">
            Create your account
          </h1>
          <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">Join the map and share spots</p>
        </div>

        <form onSubmit={submit} className="space-y-4">
          <div>
            <label
              htmlFor="email"
              className="mb-1 block text-sm font-medium text-zinc-700 dark:text-zinc-300"
            >
              Email
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full rounded-xl border border-zinc-300 px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-500/20 focus:outline-none dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100"
              placeholder="you@example.com"
            />
          </div>

          <div>
            <label
              htmlFor="username"
              className="mb-1 block text-sm font-medium text-zinc-700 dark:text-zinc-300"
            >
              Username
            </label>
            <input
              id="username"
              type="text"
              autoComplete="nickname"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full rounded-xl border border-zinc-300 px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-500/20 focus:outline-none dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100"
              placeholder="myname"
            />
          </div>

          <div>
            <label
              htmlFor="password"
              className="mb-1 block text-sm font-medium text-zinc-700 dark:text-zinc-300"
            >
              Password
            </label>
            <input
              id="password"
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full rounded-xl border border-zinc-300 px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-500/20 focus:outline-none dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100"
            />
            <p className="mt-1 text-xs text-zinc-500 dark:text-zinc-400">At least 8 characters</p>
          </div>

          {error && (
            <p role="alert" className="text-sm font-medium text-rose-600 dark:text-rose-400">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="w-full rounded-xl bg-gradient-to-r from-rose-600 to-rose-500 px-4 py-2.5 font-medium text-white shadow-md shadow-rose-600/25 transition-all hover:shadow-lg hover:shadow-rose-600/30 active:scale-[0.98] disabled:opacity-60"
          >
            {submitting ? "Creating account…" : "Sign up"}
          </button>
        </form>

        <p className="mt-6 text-center text-sm text-zinc-600 dark:text-zinc-400">
          Already have an account?{" "}
          <Link href="/login" className="font-semibold text-rose-600 hover:underline">
            Log in
          </Link>
        </p>
      </div>
      </div>
    </>
  );
}