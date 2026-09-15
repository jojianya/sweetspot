package reports

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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
