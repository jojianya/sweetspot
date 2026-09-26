import axios from "axios";
import { useAuth } from "@/store/auth";
import { API_BASE_URL, reportError } from "@/lib/monitoring";

export { API_BASE_URL };

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
    const status = error?.response?.status;
    if (status >= 500) {
      reportError(error, {
        kind: "api",
        method: error?.config?.method,
        url: error?.config?.url,
        status,
      });
    }

    if (
      status === 401 &&
      !error?.config?.url?.includes("/auth/login")
    ) {
      useAuth.getState().clearAuth();
    }

    const serverMessage = error?.response?.data?.error;
    const message: string =
      typeof serverMessage === "string" && serverMessage.length > 0
        ? serverMessage
        : error?.message ?? "something went wrong";
    return Promise.reject(new Error(message));
  }
);

export default api;