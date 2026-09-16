import { z } from "zod";

export const pinBaseSchema = z.object({
  id: z.string(),
  user_id: z.string(),
  location: z.string(),
  geohash: z.string(),
  caption: z.string().nullable(),
  category_id: z.number(),
  is_hidden: z.boolean(),
  created_at: z.string(),
});

export const pinListEntrySchema = pinBaseSchema.extend({
  cover_url: z.string(),
  username: z.string().nullable(),
});

export const pinPhotoSchema = z.object({
  id: z.string(),
  pin_id: z.string(),
  photo_url: z.string(),
  thumbnail_url: z.string(),
  position: z.number(),
  created_at: z.string(),
});

export const pinDetailSchema = pinBaseSchema.extend({
  category: z.string().nullable(),
  username: z.string().nullable(),
  avatar_url: z.string().nullable(),
  photos: z.array(pinPhotoSchema),
});

export const createdPinSchema = pinBaseSchema;

export const newPinPhotoSchema = z.object({
  photo_url: z.string(),
  thumbnail_url: z.string(),
  position: z.number(),
});