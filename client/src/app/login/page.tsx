"use client";

import { useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import Navbar from "@/components/layout/Navbar";
import { login } from "@/lib/api";
import { useAuth } from "@/store/auth";

export default function LoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const { setAuth } = useAuth();
  const router = useRouter();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!email.trim()) return setError("Email is required");
    if (!password) return setError("Password is required");

    setSubmitting(true);
    try {
      const { user, token } = await login(email.trim(), password);
      setAuth(user, token);
      router.push("/");
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <Navbar />
      <div className="flex flex-1 items-center justify-center overflow-hidden bg-gradient-to-br from-rose-50 via-white to-orange-50 px-4 py-12">
      <div className="w-full max-w-sm rounded-2xl border border-zinc-200/60 bg-white/80 p-8 shadow-xl shadow-zinc-900/5 backdrop-blur">
        <div className="mb-6 text-center">
          <span className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-rose-500 to-rose-700 text-white shadow-md shadow-rose-600/30">
            <svg className="h-6 w-6" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 2a7 7 0 0 0-7 7c0 5.25 7 13 7 13s7-7.75 7-13a7 7 0 0 0-7-7Zm0 9.5A2.5 2.5 0 1 1 12 6.5a2.5 2.5 0 0 1 0 5Z" />
            </svg>
          </span>
          <h1 className="text-2xl font-semibold tracking-tight text-zinc-900">
            Welcome back
          </h1>
          <p className="mt-1 text-sm text-zinc-500">Log in to Goodspot</p>
        </div>

        <form onSubmit={submit} className="space-y-4">
          <div>
            <label
              htmlFor="email"
              className="mb-1 block text-sm font-medium text-zinc-700"
            >
              Email
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full rounded-xl border border-zinc-300 px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-500/20 focus:outline-none"
              placeholder="you@example.com"
            />
          </div>

          <div>
            <label
              htmlFor="password"
              className="mb-1 block text-sm font-medium text-zinc-700"
            >
              Password
            </label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full rounded-xl border border-zinc-300 px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus:border-rose-500 focus:ring-2 focus:ring-rose-500/20 focus:outline-none"
            />
          </div>

          {error && (
            <p role="alert" className="text-sm font-medium text-rose-600">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="w-full rounded-xl bg-gradient-to-r from-rose-600 to-rose-500 px-4 py-2.5 font-medium text-white shadow-md shadow-rose-600/25 transition-all hover:shadow-lg hover:shadow-rose-600/30 active:scale-[0.98] disabled:opacity-60"
          >
            {submitting ? "Logging in…" : "Log in"}
          </button>
        </form>

        <p className="mt-6 text-center text-sm text-zinc-600">
          No account?{" "}
          <Link href="/register" className="font-semibold text-rose-600 hover:underline">
            Sign up
          </Link>
        </p>
      </div>
      </div>
    </>
  );
}