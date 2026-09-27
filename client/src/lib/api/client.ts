import axios from "axios";
import { useAuth } from "@/store/auth";
import { API_BASE_URL, reportError } from "@/lib/monitoring";
import { isSessionEnded } from "./session";

export { API_BASE_URL };

/**
 * ApiError carries the HTTP status code alongside the message, so callers can
 * branch on `error.status` instead of parsing the message string.
 *
 * The status is 0 when the request never reached the server (network error,
 * timeout, CORS preflight failure) — there is no HTTP response to read.
 */
export class ApiError extends Error {
  status: number;
  code: string | undefined;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 20000,
  // The session token is an httpOnly cookie, so the browser sends it
  // automatically on same-origin requests. No Authorization header is needed.
  withCredentials: true,
});

api.interceptors.response.use(
  (res) => res,
  (error) => {
    const status = error?.response?.status ?? 0;
    const code = error?.code;

    if (status >= 500) {
      reportError(error, {
        kind: "api",
        method: error?.config?.method,
        url: error?.config?.url,
        status,
      });
    }

    // A 401 from a session-protected endpoint means the httpOnly cookie can no
    // longer authenticate anything, so the cached user is stale and goes. The
    // credential endpoints are excluded: a failed login is a 401 too, but the
    // caller has no session to lose. Boot-time reconciliation lives in
    // useSessionSync — this is the backstop for a session that dies mid-visit.
    if (status === 401 && isSessionEnded(error?.config?.url)) {
      useAuth.getState().clearAuth();
    }

    const serverMessage = error?.response?.data?.error;
    const message: string =
      typeof serverMessage === "string" && serverMessage.length > 0
        ? serverMessage
        : error?.message ?? "something went wrong";

    return Promise.reject(new ApiError(message, status, code));
  }
);

export default api;
