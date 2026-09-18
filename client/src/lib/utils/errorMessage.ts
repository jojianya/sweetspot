const FALLBACK_MESSAGE = "something went wrong";

/** Extracts a human-readable message from an unknown thrown value. */
export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : FALLBACK_MESSAGE;
}
