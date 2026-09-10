package reports

type ReviewReportRequest struct {
	Action string `json:"action" binding:"required,oneof=approve dismiss"`
}
