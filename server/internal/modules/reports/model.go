package reports

import (
	"errors"
	"time"
)

var (
	ErrPinNotFound     = errors.New("pin not found")
	ErrReportNotFound  = errors.New("report not found")
	ErrAlreadyReported = errors.New("pin already reported by this user")
	ErrAlreadyResolved = errors.New("report already resolved")
)

const (
	StatusPending  = "pending"
	StatusReviewed = "reviewed"
	StatusActioned = "actioned"
)

type Report struct {
	ID         string     `json:"id"`
	PinID      string     `json:"pin_id"`
	ReporterID string     `json:"reporter_id"`
	Reason     string     `json:"reason"`
	Status     string     `json:"status"`
	ResolvedBy *string    `json:"resolved_by"`
	ResolvedAt *time.Time `json:"resolved_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

type ReportListEntry struct {
	Report
	ReporterUsername *string `json:"reporter_username"`
	PinCaption       *string `json:"pin_caption"`
}
