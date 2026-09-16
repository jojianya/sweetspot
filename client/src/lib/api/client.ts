import axios from "axios";
import { useAuth } from "@/store/auth";

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8081";

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 20000,
});

api.interceptors.request.use((config) => {
  const token = useAuth.getState().token;
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

api.interceptors.response.use(
  (res) => res,
  (error) => {
    if (
      error?.response?.status === 401 &&
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