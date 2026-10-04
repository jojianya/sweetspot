// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import ResetForm from "./ResetForm";

const apiMocks = vi.hoisted(() => ({
  confirmPasswordReset: vi.fn(),
}));

vi.mock("@/lib/api", () => apiMocks);

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(window.location.search),
}));

vi.mock("@/components/layout/Navbar", () => ({ default: () => null }));

const TOKEN = "secret-reset-token-xyz";

function setInput(el: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")!.set!;
  setter.call(el, value);
  el.dispatchEvent(new Event("input", { bubbles: true }));
}

describe("ResetForm token hygiene", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    const actEnvironment = globalThis as typeof globalThis & {
      IS_REACT_ACT_ENVIRONMENT: boolean;
    };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    apiMocks.confirmPasswordReset.mockReset();
    window.history.replaceState(null, "", `/reset-password?token=${TOKEN}`);
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    container.remove();
    window.history.replaceState(null, "", "/");
  });

  async function renderForm() {
    await act(async () => {
      root.render(<ResetForm />);
    });
  }

  async function submitValidPasswords() {
    const pw = container.querySelector("#password");
    const cf = container.querySelector("#confirm");
    if (!(pw instanceof HTMLInputElement) || !(cf instanceof HTMLInputElement)) {
      throw new Error("password inputs missing");
    }
    await act(async () => {
      setInput(pw, "brandnewpassword123");
      setInput(cf, "brandnewpassword123");
    });
    const form = container.querySelector("form");
    if (!form) throw new Error("form missing");
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
  }

  it("strips the token from the address bar on mount", async () => {
    expect(window.location.search).toContain(TOKEN);
    await renderForm();
    expect(window.location.search).toBe("");
  });

  it("sends the token only in the POST body and shows a generic failure", async () => {
    apiMocks.confirmPasswordReset.mockRejectedValue(new Error("invalid or expired reset link"));
    await renderForm();
    await submitValidPasswords();
    expect(apiMocks.confirmPasswordReset).toHaveBeenCalledWith(TOKEN, "brandnewpassword123");
    const alert = container.querySelector('[role="alert"]');
    expect(alert).not.toBeNull();
    expect(alert!.textContent).toContain("invalid or has expired");
    // The token appears nowhere in the rendered page or the URL afterwards.
    expect(document.body.innerHTML).not.toContain(TOKEN);
    expect(window.location.href).not.toContain(TOKEN);
  });

  it("shows a missing-link message when no token is present", async () => {
    window.history.replaceState(null, "", "/reset-password");
    await renderForm();
    await submitValidPasswords();
    expect(apiMocks.confirmPasswordReset).not.toHaveBeenCalled();
    expect(container.querySelector('[role="alert"]')).not.toBeNull();
  });
});
