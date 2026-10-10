import { z } from "zod";
import api from "./client";

/**
 * Shape of the answer to one toggle. `reacted` is the caller's state after the
 * write and `good_spot_count` is the pin's authoritative total, so a client
 * never has to re-read the pin to learn the new count.
 */
export const reactionResultSchema = z.object({
  reacted: z.boolean(),
  good_spot_count: z.number(),
});

export type ReactionResult = z.infer<typeof reactionResultSchema>;

/**
 * Marks a pin as a "Good spot". Idempotent server-side: a second call for the
 * same account is a successful no-op that reports the same count, so a double
 * tap cannot double count.
 */
export async function reactToPin(pinId: string): Promise<ReactionResult> {
  const { data } = await api.put(`/pins/${pinId}/good-spot`);
  return reactionResultSchema.parse(data);
}

/**
 * Removes the caller's reaction. Removing one that was never there is still a
 * success with reacted:false, so an undo race never surfaces as an error.
 */
export async function unreactToPin(pinId: string): Promise<ReactionResult> {
  const { data } = await api.delete(`/pins/${pinId}/good-spot`);
  return reactionResultSchema.parse(data);
}
