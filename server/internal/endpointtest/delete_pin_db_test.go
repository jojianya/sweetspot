package endpointtest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/database"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

func isPinForbidden(err error) bool { return errors.Is(err, pins.ErrForbidden) }

func isPinNotFound(err error) bool { return errors.Is(err, pins.ErrNotFound) }

// requireEndpointDB connects to the disposable database named by
// ENDPOINT_TEST_DB and migrates it to the current schema.
//
// It refuses to run when ENDPOINT_TEST_DB equals DB_NAME (default
// "goodspotdb") so a misconfigured run cannot hide or lock real pins.
// It skips when credentials are absent, matching the requireDB convention,
// which keeps the default `go test ./...` (including CI without secrets)
// green while still running wherever the disposable DB is provisioned.
func requireEndpointDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	name := os.Getenv("ENDPOINT_TEST_DB")
	if name == "" {
		t.Skip("ENDPOINT_TEST_DB not set; set it to a disposable database to run")
	}
	if os.Getenv("DB_PASSWORD") == "" {
		t.Skip("DB_PASSWORD not set, skipping database test")
	}
	if name == endpointDBName() {
		t.Fatalf("ENDPOINT_TEST_DB %q equals DB_NAME; refusing to run against the development database", name)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		getEnv("DB_USER", "postgres"), os.Getenv("DB_PASSWORD"),
		getEnv("DB_HOST", "127.0.0.1"), getEnv("DB_PORT", "5432"), name)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("db ping: %v", err)
	}

	if err := database.RunMigrations(pool, endpointMigrationsDir(t)); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return pool
}

func endpointDBName() string {
	if v := os.Getenv("DB_NAME"); v != "" {
		return v
	}
	return "goodspotdb"
}

// endpointMigrationsDir resolves database.MigrationsDir from the module root.
// `go test` runs with the package directory as cwd, so walk up to go.mod.
func endpointMigrationsDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, database.MigrationsDir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find module root above package dir")
		}
		dir = parent
	}
}

func seedDBUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, role string) string {
	t.Helper()
	tag := fmt.Sprintf("%s-%d", strings.ReplaceAll(t.Name(), "/", "-"), time.Now().UnixNano())
	email := fmt.Sprintf("delpin-%s@example.com", tag)
	username := fmt.Sprintf("delpin-%s", tag)
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

func seedDBPin(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID string, categoryID int) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO pins (id, user_id, location, geohash, caption, category_id)
		VALUES (gen_random_uuid(), $1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, 'seed', 'seed', $4)
		RETURNING id
	`, userID, -122.43, 37.77, categoryID).Scan(&id)
	if err != nil {
		t.Fatalf("seed pin: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM pins WHERE id = $1`, id)
	})
	return id
}

func dbCategoryID(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var id int
	if err := pool.QueryRow(ctx, `SELECT id FROM categories ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("load category: %v", err)
	}
	return id
}

func dbIsHidden(t *testing.T, ctx context.Context, pool *pgxpool.Pool, pinID string) bool {
	t.Helper()
	var hidden bool
	if err := pool.QueryRow(ctx, `SELECT is_hidden FROM pins WHERE id = $1`, pinID).Scan(&hidden); err != nil {
		t.Fatalf("read is_hidden: %v", err)
	}
	return hidden
}

// TestDBDeletePinAuthorization exercises the real DeletePin SQL for the five
// cases that stub tests cannot prove: owner success, moderator success on
// another user's pin, non-owner refusal, missing pin, and delete-twice.
func TestDBDeletePinAuthorization(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	repo := pins.NewRepository(pool)
	categoryID := dbCategoryID(t, ctx, pool)

	t.Run("OwnerDeletesOwnPin", func(t *testing.T) {
		owner := seedDBUser(t, ctx, pool, "user")
		pinID := seedDBPin(t, ctx, pool, owner, categoryID)
		if err := repo.DeletePin(ctx, pinID, owner, false); err != nil {
			t.Fatalf("owner delete: %v", err)
		}
		if !dbIsHidden(t, ctx, pool, pinID) {
			t.Fatal("expected pin to be hidden after owner delete")
		}
	})

	t.Run("ModeratorDeletesOthersPin", func(t *testing.T) {
		owner := seedDBUser(t, ctx, pool, "user")
		moderator := seedDBUser(t, ctx, pool, "admin")
		pinID := seedDBPin(t, ctx, pool, owner, categoryID)
		if err := repo.DeletePin(ctx, pinID, moderator, true); err != nil {
			t.Fatalf("moderator delete: %v", err)
		}
		if !dbIsHidden(t, ctx, pool, pinID) {
			t.Fatal("expected pin to be hidden after moderator delete")
		}
	})

	t.Run("NonOwnerNonModeratorIsForbidden", func(t *testing.T) {
		owner := seedDBUser(t, ctx, pool, "user")
		stranger := seedDBUser(t, ctx, pool, "user")
		pinID := seedDBPin(t, ctx, pool, owner, categoryID)
		if err := repo.DeletePin(ctx, pinID, stranger, false); err == nil {
			t.Fatal("expected forbidden error, got nil")
		} else if !isPinForbidden(err) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
		if dbIsHidden(t, ctx, pool, pinID) {
			t.Fatal("refused delete must not hide the pin")
		}
	})

	t.Run("MissingPinIsNotFound", func(t *testing.T) {
		owner := seedDBUser(t, ctx, pool, "user")
		if err := repo.DeletePin(ctx, "00000000-0000-4000-8000-000000000000", owner, false); err == nil {
			t.Fatal("expected not-found error, got nil")
		} else if !isPinNotFound(err) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("DeleteTwiceIsNotFound", func(t *testing.T) {
		owner := seedDBUser(t, ctx, pool, "user")
		pinID := seedDBPin(t, ctx, pool, owner, categoryID)
		if err := repo.DeletePin(ctx, pinID, owner, false); err != nil {
			t.Fatalf("first delete: %v", err)
		}
		if err := repo.DeletePin(ctx, pinID, owner, false); err == nil {
			t.Fatal("expected not-found on second delete, got nil")
		} else if !isPinNotFound(err) {
			t.Fatalf("expected ErrNotFound on second delete, got %v", err)
		}
	})
}

// TestDBDeletePinHandlerModerator proves the handler wiring against live
// roles: an admin caller deleting another user's pin through HTTP gets 204.
func TestDBDeletePinHandlerModerator(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()
	categoryID := dbCategoryID(t, ctx, pool)
	owner := seedDBUser(t, ctx, pool, "user")
	moderator := seedDBUser(t, ctx, pool, "admin")
	pinID := seedDBPin(t, ctx, pool, owner, categoryID)

	gin.SetMode(gin.TestMode)
	userSvc := users.NewService(users.NewRepository(pool))
	h := pins.NewHandler(pins.NewRepository(pool), storage.NewLocal(t.TempDir(), "http://test.local"), nil, userSvc)
	r := gin.New()
	r.DELETE("/pins/:id", func(c *gin.Context) {
		c.Set("user_id", moderator)
		h.DeletePin(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/pins/"+pinID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (%s)", w.Code, w.Body.String())
	}
	if !dbIsHidden(t, ctx, pool, pinID) {
		t.Fatal("expected pin to be hidden after handler moderator delete")
	}
}
