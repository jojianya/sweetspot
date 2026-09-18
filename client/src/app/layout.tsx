import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import Script from "next/script";
import ErrorBoundary from "@/components/ErrorBoundary";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Goodspot — Pins on a map",
  description: "Drop pins, share spots, explore the map.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <head>
        <Script id="theme-init" strategy="beforeInteractive">
          {`(function () {
  try {
    var saved = localStorage.getItem("goodspot-theme");
    var theme = "light";
    if (saved) {
      var parsed = JSON.parse(saved);
      if (parsed && parsed.state && parsed.state.theme === "dark") theme = "dark";
    } else if (window.matchMedia("(prefers-color-scheme: dark)").matches) {
      theme = "dark";
    }
    if (theme === "dark") document.documentElement.classList.add("dark");
    document.documentElement.style.colorScheme = theme;
  } catch (e) {}
})();`}
        </Script>
      </head>
      <body className="flex h-screen flex-col overflow-hidden">
        <ErrorBoundary>
          <main className="relative flex flex-1 flex-col overflow-hidden">{children}</main>
        </ErrorBoundary>
      </body>
    </html>
  );
}
