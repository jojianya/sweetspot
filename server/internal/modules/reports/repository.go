package reports

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

// pgCodeUniqueViolation is the PostgreSQL SQLSTATE for unique_violation.
// A named constant because pgerrcode is not a dependency.
const pgCodeUniqueViolation = "23505"

type Repository interface {
	PinExists(ctx context.Context, pinID string) (bool, error)
	CreateReport(ctx context.Context, pinID, reporterID, reason string) (Report, error)
	ReviewReport(ctx context.Context, reportID, action, resolvedBy string) (Report, *string, error)
	ListReports(ctx context.Context, status *string, limit, offset int) ([]ReportListEntry, error)
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) PinExists(ctx context.Context, pinID string) (bool, error) {
	return pins.VisiblePinExists(ctx, r.pool, pinID)
}

func (r *postgresRepository) CreateReport(ctx context.Context, pinID, reporterID, reason string) (Report, error) {
	var rep Report
	err := r.pool.QueryRow(ctx, `
		INSERT INTO reports (pin_id, reporter_id, reason)
		VALUES ($1, $2, $3)
		RETURNING id, pin_id, reporter_id, reason, status, resolved_by, resolved_at, created_at
	`, pinID, reporterID, reason).Scan(
		&rep.ID, &rep.PinID, &rep.ReporterID, &rep.Reason, &rep.Status, &rep.ResolvedBy, &rep.ResolvedAt, &rep.CreatedAt,
	)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgCodeUniqueViolation {
		return Report{}, ErrAlreadyReported
	}
	return rep, err
}

func (r *postgresRepository) ReviewReport(ctx context.Context, reportID, action, resolvedBy string) (Report, *string, error) {
	var rep Report

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return rep, nil, err
	}
	defer tx.Rollback(ctx)

	var scanErr error
	scanErr = tx.QueryRow(ctx, `
		SELECT status FROM reports WHERE id = $1 FOR UPDATE
	`, reportID).Scan(&rep.Status)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return rep, nil, ErrReportNotFound
		}
		return rep, nil, scanErr
	}
	if rep.Status != StatusPending {
		return rep, nil, ErrAlreadyResolved
	}

	status := StatusReviewed
	var pinLocation *string
	if action == "approve" {
		status = StatusActioned
		var pinID pgtype.UUID
		if err := tx.QueryRow(ctx, `SELECT pin_id FROM reports WHERE id = $1`, reportID).Scan(&pinID); err != nil {
			return rep, nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE pins SET is_hidden = true WHERE id = $1`, pinID); err != nil {
			return rep, nil, err
		}
		// Get the pin location for the pin_removed event. Must be WKT like
		// the pins repo's ST_AsText, or matchesRemoved in realtime/handler.go
		// drops it for bbox-filtered clients.
		var location string
		err := tx.QueryRow(ctx, `SELECT ST_AsText(location) FROM pins WHERE id = $1`, pinID).Scan(&location)
		if err != nil {
			return rep, nil, err
		}
		pinLocation = &location
	}

	scanErr = tx.QueryRow(ctx, `
		UPDATE reports
		SET status = $2, resolved_by = $3, resolved_at = now()
		WHERE id = $1
		RETURNING id, pin_id, reporter_id, reason, status, resolved_by, resolved_at, created_at
	`, reportID, status, resolvedBy).Scan(
		&rep.ID, &rep.PinID, &rep.ReporterID, &rep.Reason, &rep.Status, &rep.ResolvedBy, &rep.ResolvedAt, &rep.CreatedAt,
	)
	if scanErr != nil {
		return rep, nil, scanErr
	}

	if err := tx.Commit(ctx); err != nil {
		return rep, nil, err
	}
	return rep, pinLocation, nil
}

func (r *postgresRepository) ListReports(ctx context.Context, status *string, limit, offset int) ([]ReportListEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.id, r.pin_id, r.reporter_id, r.reason, r.status, r.resolved_by, r.resolved_at, r.created_at,
		       u.username, p.caption
		FROM reports r
		LEFT JOIN users u ON u.id = r.reporter_id
		LEFT JOIN pins p ON p.id = r.pin_id
		WHERE ($1::text IS NULL OR r.status = $1)
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3
	`, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []ReportListEntry{}
	for rows.Next() {
		var e ReportListEntry
		if err := rows.Scan(
			&e.ID, &e.PinID, &e.ReporterID, &e.Reason, &e.Status, &e.ResolvedBy, &e.ResolvedAt, &e.CreatedAt,
			&e.ReporterUsername, &e.PinCaption,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	return entries, nil
}
