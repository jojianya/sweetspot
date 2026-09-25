import api from "./client";
import { reportEntrySchema, reportSchema } from "./schemas";
import type { Report, ReportEntry, ReportStatus } from "@/lib/types";

/** GET /reports — moderation queue (admin/owner only). */
export async function fetchReports(options: {
  status?: ReportStatus;
  limit?: number;
  offset?: number;
} = {}): Promise<ReportEntry[]> {
  const { data } = await api.get<{ reports: unknown }>("/reports", { params: options });
  return reportEntrySchema.array().parse(data.reports);
}

/** PATCH /reports/:id — approve (hides the pin) or dismiss (admin/owner only). */
export async function reviewReport(id: string, action: "approve" | "dismiss"): Promise<Report> {
  const { data } = await api.patch<{ report: unknown }>(`/reports/${id}`, { action });
  return reportSchema.parse(data.report);
}

/** POST /pins/:id/report — any logged-in user; one report per (pin, reporter). */
export async function createPinReport(pinId: string, reason: string): Promise<Report> {
  const { data } = await api.post<{ report: unknown }>(`/pins/${pinId}/report`, { reason });
  return reportSchema.parse(data.report);
}