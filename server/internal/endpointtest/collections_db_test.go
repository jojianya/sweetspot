package endpointtest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/collections"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

func seedPrivacyUsers(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (owner, other string) {
	t.Helper()
	tag := fmt.Sprintf("%d", time.Now().UnixNano())
	owner = seedDBUserWithRole(t, ctx, pool, "priv-owner-"+tag+"@example.com", "user")
	other = seedDBUserWithRole(t, ctx, pool, "priv-other-"+tag+"@example.com", "user")
	return owner, other
}

func seedDBUserWithRole(t *testing.T, ctx context.Context, pool *pgxpool.Pool, email, role string) string {
	t.Helper()
	username := "priv-" + strings.SplitN(email, "@", 2)[0]
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (id, email, username, password_hash, role)
		VALUES (gen_random_uuid(), $1, $2, 'hash', $3)
		RETURNING id
	`, email, username, role).Scan(&id)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

func seedCollection(t *testing.T, ctx context.Context, repo collections.Repository, userID, name string, isPrivate bool) collections.Collection {
	t.Helper()
	desc := "desc"
	c, err := repo.Create(ctx, userID, name, &desc, isPrivate)
	if err != nil {
		t.Fatalf("seed collection: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Delete(context.Background(), c.ID.String())
	})
	return c
}

// privacyRouter wires the real collections handler with stubbed auth: a nil
// userID simulates a logged-out visitor.
func privacyRouter(h *collections.Handler, userID *string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	stub := func(c *gin.Context) {
		if userID != nil {
			c.Set("user_id", *userID)
		}
		c.Next()
	}
	g := r
	g.GET("/users/:id/collections", stub, h.ListByUser)
	g.GET("/collections/:id", stub, h.Get)
	return r
}

func getJSON(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

// TestDBCollectionPrivacy proves the visibility contract on a real database:
// owners see everything, anyone else gets 404 on private collections and a
// public-only list, whether logged in or out.
func TestDBCollectionPrivacy(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	repo := collections.NewRepository(pool)
	userSvc := users.NewService(users.NewRepository(pool))
	h := collections.NewHandler(repo, userSvc)
	owner, other := seedPrivacyUsers(t, ctx, pool)

	public := seedCollection(t, ctx, repo, owner, "public", false)
	private := seedCollection(t, ctx, repo, owner, "private", true)

	t.Run("OwnerSeesPrivate", func(t *testing.T) {
		r := privacyRouter(h, &owner)
		w := getJSON(t, r, "/collections/"+private.ID.String())
		if w.Code != http.StatusOK {
			t.Fatalf("owner get private: got %d (%s)", w.Code, w.Body.String())
		}
		w = getJSON(t, r, "/users/"+owner+"/collections")
		if w.Code != http.StatusOK {
			t.Fatalf("owner list: got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, public.ID.String()) || !strings.Contains(body, private.ID.String()) {
			t.Fatalf("owner list missing collections: %s", body)
		}
	})

	t.Run("OtherUserGets404AndPublicOnly", func(t *testing.T) {
		r := privacyRouter(h, &other)
		w := getJSON(t, r, "/collections/"+private.ID.String())
		if w.Code != http.StatusNotFound {
			t.Fatalf("other get private: got %d (%s), want 404", w.Code, w.Body.String())
		}
		w = getJSON(t, r, "/collections/"+public.ID.String())
		if w.Code != http.StatusOK {
			t.Fatalf("other get public: got %d", w.Code)
		}
		w = getJSON(t, r, "/users/"+owner+"/collections")
		body := w.Body.String()
		if !strings.Contains(body, public.ID.String()) {
			t.Fatalf("other list missing public collection: %s", body)
		}
		if strings.Contains(body, private.ID.String()) {
			t.Fatalf("other list leaks private collection: %s", body)
		}
	})

	t.Run("LoggedOutGets404AndPublicOnly", func(t *testing.T) {
		r := privacyRouter(h, nil)
		w := getJSON(t, r, "/collections/"+private.ID.String())
		if w.Code != http.StatusNotFound {
			t.Fatalf("logged-out get private: got %d (%s), want 404", w.Code, w.Body.String())
		}
		w = getJSON(t, r, "/users/"+owner+"/collections")
		body := w.Body.String()
		if !strings.Contains(body, public.ID.String()) {
			t.Fatalf("logged-out list missing public collection: %s", body)
		}
		if strings.Contains(body, private.ID.String()) {
			t.Fatalf("logged-out list leaks private collection: %s", body)
		}
	})
}

// TestDBCollectionPrivacyToggle proves owners can flip visibility and that an
// absent flag preserves the current value.
func TestDBCollectionPrivacyToggle(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	repo := collections.NewRepository(pool)
	userSvc := users.NewService(users.NewRepository(pool))
	h := collections.NewHandler(repo, userSvc)
	owner, other := seedPrivacyUsers(t, ctx, pool)

	c := seedCollection(t, ctx, repo, owner, "toggle", false)

	patch := func(userID *string, body string) *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.PATCH("/collections/:id", func(g *gin.Context) {
			if userID != nil {
				g.Set("user_id", *userID)
			}
			g.Next()
		}, h.Update)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/collections/"+c.ID.String(), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}

	// Make private: the other user loses access.
	if w := patch(&owner, `{"name":"toggle","is_private":true}`); w.Code != http.StatusNoContent {
		t.Fatalf("make private: got %d (%s)", w.Code, w.Body.String())
	}
	r := privacyRouter(h, &other)
	if w := getJSON(t, r, "/collections/"+c.ID.String()); w.Code != http.StatusNotFound {
		t.Fatalf("other after privatize: got %d, want 404", w.Code)
	}

	// Absent flag preserves privacy.
	if w := patch(&owner, `{"name":"toggle"}`); w.Code != http.StatusNoContent {
		t.Fatalf("name-only update: got %d (%s)", w.Code, w.Body.String())
	}
	if w := getJSON(t, r, "/collections/"+c.ID.String()); w.Code != http.StatusNotFound {
		t.Fatalf("other after name-only update: got %d, want 404", w.Code)
	}

	// Make public again: visible once more.
	if w := patch(&owner, `{"name":"toggle","is_private":false}`); w.Code != http.StatusNoContent {
		t.Fatalf("make public: got %d (%s)", w.Code, w.Body.String())
	}
	if w := getJSON(t, r, "/collections/"+c.ID.String()); w.Code != http.StatusOK {
		t.Fatalf("other after publicize: got %d (%s), want 200", w.Code, w.Body.String())
	}
}
