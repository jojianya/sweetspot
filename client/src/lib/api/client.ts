import axios from "axios";

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8081";

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 20000,
});

api.interceptors.response.use(
  (res) => res,
  (error) => {
    const message: string =
      error?.response?.data?.error ?? error?.message ?? "something went wrong";
    return Promise.reject(new Error(message));
  }
);

export function bearer(token: string | null) {
  return token ? { Authorization: `Bearer ${token}` } : {};
}

export default api;