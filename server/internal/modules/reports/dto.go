package reports

type CreateReportRequest struct {
	Reason string `json:"reason" binding:"required,min=3"`
}
