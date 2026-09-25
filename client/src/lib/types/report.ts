export type ReportStatus = "pending" | "reviewed" | "actioned";

/** A moderation report on a pin (POST /pins/:id/report, PATCH /reports/:id). */
export interface Report {
  id: string;
  pin_id: string;
  reporter_id: string;
  reason: string;
  status: ReportStatus;
  resolved_by: string | null;
  resolved_at: string | null;
  created_at: string;
}

/** Report as returned by GET /reports — includes joined user/pin display fields. */
export interface ReportEntry extends Report {
  reporter_username: string | null;
  pin_caption: string | null;
}