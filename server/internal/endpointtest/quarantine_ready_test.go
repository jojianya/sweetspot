package endpointtest

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/jojianya/sweetspot247-backend/internal/config"
	"github.com/jojianya/sweetspot247-backend/internal/di"
	srvhttp "github.com/jojianya/sweetspot247-backend/internal/http"
	"github.com/jojianya/sweetspot247-backend/internal/modules/collections"
	"github.com/jojianya/sweetspot247-backend/internal/modules/comments"
	"github.com/jojianya/sweetspot247-backend/internal/modules/favorites"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/modules/realtime"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
	"github.com/jojianya/sweetspot247-backend/internal/modules/social"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

// readyBody builds the full production router against the endpoint-test
// database and reads GET /ready with the given boot-time quarantine status.
func readyBody(t *testing.T, quarantineStatus string) (int, map[string]any) {
	t.Helper()
	pool := requireEndpointDB(t)
	usersSvc := newUsersSvc()
	store := storage.NewLocal(t.TempDir(), "http://test.local")
	c := &di.Container{
		UserService:    usersSvc,
		PinRepo:        pins.NewRepository(pool),
		ReportRepo:     reports.NewRepository(pool),
		FavoriteRepo:   favorites.NewRepository(pool),
		CommentRepo:    comments.NewRepository(pool),
		SocialRepo:     social.NewRepository(pool),
		CollectionRepo: collections.NewRepository(pool),
		Store:          store,
		Events:         realtime.NewBrokerWithClient(redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})),
	}
	cfg := &config.Config{JWTSecret: testSecret}
	r := srvhttp.NewRouter(cfg, pool, c, slog.Default(), nil, quarantineStatus)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ready", nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /ready: %v (body=%q)", err, w.Body.String())
	}
	return w.Code, body
}

// TestReadyReportsQuarantineStatus proves the boot-time probe result rides
// along in /ready without changing its HTTP status: quarantine health must
// be loud but must never take traffic down. /health is untouched.
func TestReadyReportsQuarantineStatus(t *testing.T) {
	for _, tc := range []struct{ status string }{{"ok"}, {"unwritable"}} {
		code, body := readyBody(t, tc.status)
		if code != http.StatusOK {
			t.Fatalf("status %q: /ready = %d, want 200", tc.status, code)
		}
		if body["quarantine"] != tc.status {
			t.Errorf("status %q: quarantine field = %v, want %q", tc.status, body["quarantine"], tc.status)
		}
	}
}
