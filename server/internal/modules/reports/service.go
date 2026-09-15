package reports

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

type Service interface {
	PinExists(ctx context.Context, pinID string) (bool, error)
	CreateReport(ctx context.Context, pinID, reporterID, reason string) (Report, error)
	ReviewReport(ctx context.Context, reportID, action, resolvedBy string) (Report, error)
	ListReports(ctx context.Context, status *string, limit, offset int) ([]ReportListEntry, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) PinExists(ctx context.Context, pinID string) (bool, error) {
	return s.repo.PinExists(ctx, pinID)
}

func (s *service) CreateReport(ctx context.Context, pinID, reporterID, reason string) (Report, error) {
	rep, err := s.repo.CreateReport(ctx, pinID, reporterID, reason)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Report{}, ErrAlreadyReported
		}
		return Report{}, err
	}
	return rep, nil
}

func (s *service) ReviewReport(ctx context.Context, reportID, action, resolvedBy string) (Report, error) {
	return s.repo.ReviewReport(ctx, reportID, action, resolvedBy)
}

func (s *service) ListReports(ctx context.Context, status *string, limit, offset int) ([]ReportListEntry, error) {
	return s.repo.ListReports(ctx, status, limit, offset)
}
