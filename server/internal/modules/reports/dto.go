package reports

type CreateReportRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=1000"`
}

type ReviewReportRequest struct {
	Action string `json:"action" binding:"required,oneof=approve dismiss"`
}
