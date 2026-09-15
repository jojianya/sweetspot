package reports

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type mockReportRepository struct {
	exists    bool
	existsErr error
	created   Report
	createErr error
	reviewed  Report
	reviewErr error
	list      []ReportListEntry
	listErr   error
}

func (m *mockReportRepository) PinExists(context.Context, string) (bool, error) {
	return m.exists, m.existsErr
}

func (m *mockReportRepository) CreateReport(context.Context, string, string, string) (Report, error) {
	return m.created, m.createErr
}

func (m *mockReportRepository) ReviewReport(context.Context, string, string, string) (Report, error) {
	return m.reviewed, m.reviewErr
}

func (m *mockReportRepository) ListReports(context.Context, *string, int, int) ([]ReportListEntry, error) {
	return m.list, m.listErr
}

func TestCreateReportDuplicate(t *testing.T) {
	svc := NewService(&mockReportRepository{createErr: &pgconn.PgError{Code: "23505"}})
	_, err := svc.CreateReport(context.Background(), "pin1", "u1", "spam")
	if !errors.Is(err, ErrAlreadyReported) {
		t.Fatalf("expected ErrAlreadyReported, got %v", err)
	}
}

func TestReviewReportPropagatesNotFound(t *testing.T) {
	svc := NewService(&mockReportRepository{reviewErr: ErrReportNotFound})
	_, err := svc.ReviewReport(context.Background(), "r1", "dismiss", "u1")
	if !errors.Is(err, ErrReportNotFound) {
		t.Fatalf("expected ErrReportNotFound, got %v", err)
	}
}

func TestPinExists(t *testing.T) {
	svc := NewService(&mockReportRepository{exists: true})
	exists, err := svc.PinExists(context.Background(), "pin1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected pin to exist")
	}
}

func TestListReports(t *testing.T) {
	svc := NewService(&mockReportRepository{list: []ReportListEntry{}})
	entries, err := svc.ListReports(context.Background(), nil, 50, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries == nil {
		t.Fatal("expected empty slice, got nil")
	}
}
