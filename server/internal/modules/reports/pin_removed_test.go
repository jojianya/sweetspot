package reports

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func init() { gin.SetMode(gin.TestMode) }

type stubReviewService struct {
	Service
	report    Report
	location  *string
	reviewErr error
}

func (s *stubReviewService) ReviewReport(context.Context, string, string, string) (Report, *string, error) {
	return s.report, s.location, s.reviewErr
}

type recordingPublisher struct {
	ids       []string
	locations []string
}

func (r *recordingPublisher) PublishRemoval(_ context.Context, id string, location string) {
	r.ids = append(r.ids, id)
	r.locations = append(r.locations, location)
}

func reviewRequest(t *testing.T, h *Handler, reportID, action string) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.PATCH("/reports/:id", func(c *gin.Context) {
		c.Set("user_id", "admin-1")
		h.Review(c)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/reports/"+reportID, strings.NewReader(`{"action":"`+action+`"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func uuidPtr(s string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		panic(err)
	}
	return u
}

// TestReviewApprovePublishesRemoval proves report approve publishes a removal
// with the reported pin's id and committed location via the reports-local
// Publisher (no pins import involved).
func TestReviewApprovePublishesRemoval(t *testing.T) {
	loc := "POINT(25 15)"
	svc := &stubReviewService{
		report:   Report{PinID: uuidPtr("11111111-1111-1111-1111-111111111111")},
		location: &loc,
	}
	pub := &recordingPublisher{}
	h := NewHandler(svc, pub)

	w := reviewRequest(t, h, "rep-1", "approve")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	if len(pub.ids) != 1 {
		t.Fatalf("expected 1 removal publish, got %d", len(pub.ids))
	}
	if pub.ids[0] != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("expected pin id to be published, got %q", pub.ids[0])
	}
	if pub.locations[0] != loc {
		t.Errorf("expected location %q, got %q", loc, pub.locations[0])
	}
}

// TestReviewDismissPublishesNothing proves a non-approve resolution never
// publishes a removal.
func TestReviewDismissPublishesNothing(t *testing.T) {
	svc := &stubReviewService{report: Report{}, location: nil}
	pub := &recordingPublisher{}
	h := NewHandler(svc, pub)

	w := reviewRequest(t, h, "rep-1", "dismiss")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	if len(pub.ids) != 0 {
		t.Errorf("expected no removal publish on dismiss, got %d", len(pub.ids))
	}
}

// TestReviewErrorPublishesNothing proves a failed review publishes nothing:
// the report was not resolved, so there is no committed removal to broadcast.
func TestReviewErrorPublishesNothing(t *testing.T) {
	svc := &stubReviewService{reviewErr: ErrReportNotFound}
	pub := &recordingPublisher{}
	h := NewHandler(svc, pub)

	w := reviewRequest(t, h, "missing", "approve")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}
	if len(pub.ids) != 0 {
		t.Errorf("expected no removal publish on service error, got %d", len(pub.ids))
	}
}
