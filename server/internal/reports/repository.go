package reports

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrPinNotFound     = errors.New("pin not found")
	ErrReportNotFound  = errors.New("report not found")
	ErrAlreadyReported = errors.New("pin already reported by this user")
	ErrAlreadyResolved = errors.New("report already resolved")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) PinExists(ctx context.Context, pinID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pins WHERE id = $1)`, pinID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *Repository) CreateReport(ctx context.Context, pinID, reporterID, reason string) (Report, error) {
	var rep Report
	err := r.pool.QueryRow(ctx, `
		INSERT INTO reports (pin_id, reporter_id, reason)
		VALUES ($1, $2, $3)
		RETURNING id, pin_id, reporter_id, reason, status, resolved_by, resolved_at, created_at
	`, pinID, reporterID, reason).Scan(
		&rep.ID, &rep.PinID, &rep.ReporterID, &rep.Reason, &rep.Status, &rep.ResolvedBy, &rep.ResolvedAt, &rep.CreatedAt,
	)
	return rep, err
}

func (r *Repository) ReviewReport(ctx context.Context, reportID, action, resolvedBy string) (Report, error) {
	var rep Report

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return rep, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		SELECT status FROM reports WHERE id = $1 FOR UPDATE
	`, reportID).Scan(&rep.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rep, ErrReportNotFound
		}
		return rep, err
	}
	if rep.Status != StatusPending {
		return rep, ErrAlreadyResolved
	}

	status := StatusReviewed
	if action == "approve" {
		status = StatusActioned
		var pinID pgtype.UUID
		if err := tx.QueryRow(ctx, `SELECT pin_id FROM reports WHERE id = $1`, reportID).Scan(&pinID); err != nil {
			return rep, err
		}
		if _, err := tx.Exec(ctx, `UPDATE pins SET is_hidden = true WHERE id = $1`, pinID); err != nil {
			return rep, err
		}
	}

	err = tx.QueryRow(ctx, `
		UPDATE reports
		SET status = $2, resolved_by = $3, resolved_at = now()
		WHERE id = $1
		RETURNING id, pin_id, reporter_id, reason, status, resolved_by, resolved_at, created_at
	`, reportID, status, resolvedBy).Scan(
		&rep.ID, &rep.PinID, &rep.ReporterID, &rep.Reason, &rep.Status, &rep.ResolvedBy, &rep.ResolvedAt, &rep.CreatedAt,
	)
	if err != nil {
		return rep, err
	}

	if err := tx.Commit(ctx); err != nil {
		return rep, err
	}
	return rep, nil
}
