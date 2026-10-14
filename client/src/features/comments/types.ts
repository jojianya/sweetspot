/**
 * Comments are only read and written by this feature, so the shape lives here
 * rather than in the shared wire types.
 */
export interface Comment {
  id: string;
  pin_id: string;
  user_id: string | null;
  body: string;
  is_hidden: boolean;
  created_at: string;
  username: string | null;
  avatar_url: string | null;
}
