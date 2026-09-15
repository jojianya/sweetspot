package reports

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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
	ID         pgtype.UUID `json:"id"`
	PinID      pgtype.UUID `json:"pin_id"`
	ReporterID pgtype.UUID `json:"reporter_id"`
	Reason     string      `json:"reason"`
	Status     string      `json:"status"`
	ResolvedBy pgtype.UUID `json:"resolved_by"`
	ResolvedAt *time.Time  `json:"resolved_at"`
	CreatedAt  time.Time   `json:"created_at"`
}

type ReportListEntry struct {
	Report
	ReporterUsername *string `json:"reporter_username"`
	PinCaption       *string `json:"pin_caption"`
}
