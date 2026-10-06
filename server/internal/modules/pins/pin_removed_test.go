package pins

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

type recordingEvents struct {
	created []Event
	removed []PinRemoved
}

func (r *recordingEvents) PinCreated(_ context.Context, ev Event) { r.created = append(r.created, ev) }
func (r *recordingEvents) PinRemoved(_ context.Context, ev PinRemoved) {
	r.removed = append(r.removed, ev)
}

// TestDeletePinPublishesRemoval proves the delete handler publishes
// pin_removed with the deleted pin's id and location after a successful
// delete (publish-after-commit at the handler layer: the repository delete
// already returned nil).
func TestDeletePinPublishesRemoval(t *testing.T) {
	dir := t.TempDir()
	repo := &stubRepo{detail: PinDetail{Photos: []PinPhoto{}, Pin: Pin{Location: "POINT(25 15)"}}}
	events := &recordingEvents{}
	h := &Handler{repo: repo, store: storage.NewLocal(dir, "http://api.test"), events: events}

	r := gin.New()
	r.DELETE("/pins/:id", func(c *gin.Context) {
		c.Set("user_id", "owner-1")
		h.DeletePin(c)
	})

	w := deletePin(r, "pin-1")
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", w.Code, w.Body.String())
	}
	if len(events.removed) != 1 {
		t.Fatalf("expected 1 pin_removed event, got %d", len(events.removed))
	}
	if events.removed[0].ID != "pin-1" {
		t.Errorf("expected removed id pin-1, got %q", events.removed[0].ID)
	}
	if events.removed[0].Location != "POINT(25 15)" {
		t.Errorf("expected removed location POINT(25 15), got %q", events.removed[0].Location)
	}
	if len(events.created) != 0 {
		t.Errorf("delete must not publish pin_created, got %d", len(events.created))
	}
}

// TestDeletePinPublishesNothingOnNotFound proves no removal event fires when
// the delete itself fails.
func TestDeletePinPublishesNothingOnNotFound(t *testing.T) {
	dir := t.TempDir()
	repo := &stubRepo{delErr: ErrNotFound}
	events := &recordingEvents{}
	h := &Handler{repo: repo, store: storage.NewLocal(dir, "http://api.test"), events: events}

	r := gin.New()
	r.DELETE("/pins/:id", func(c *gin.Context) {
		c.Set("user_id", "owner-1")
		h.DeletePin(c)
	})

	w := deletePin(r, "missing")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}
	if len(events.removed) != 0 {
		t.Errorf("expected no pin_removed on failed delete, got %d", len(events.removed))
	}
}
