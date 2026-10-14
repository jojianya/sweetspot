/**
 * The auth feature's public surface.
 *
 * Everything other features and app routes need from auth comes through
 * here; the store, API module and hooks stay private to the feature.
 */
export { default as ForgotForm } from "./components/ForgotForm";
export { default as LoginForm } from "./components/LoginForm";
export { default as RegisterForm } from "./components/RegisterForm";
export { default as ResetForm } from "./components/ResetForm";
export { default as SessionSync } from "./components/SessionSync";
export { useLogout } from "./hooks/useLogout";
export { useSessionRefresh } from "./hooks/useSessionRefresh";
export { fetchSession } from "./api/auth";
export { useAuth, type PersistedUser } from "./store/auth";
