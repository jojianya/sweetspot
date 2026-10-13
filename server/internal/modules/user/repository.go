package users

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/session"
)

type Repository interface {
	Create(ctx context.Context, email, passwordHash, username string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByUsername(ctx context.Context, username string) (User, error)
	GetByLogin(ctx context.Context, identifier string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
	// GetSessionState loads the session posture (live role + revocation
	// floor) in one indexed lookup for the auth middleware.
	GetSessionState(ctx context.Context, id string) (session.State, error)
	CountUsers(ctx context.Context) (int, error)
	CountOwners(ctx context.Context) (int, error)
	// ListUsers returns a page of users plus the total count. The count comes
	// from COUNT(*) OVER () in the same query (P1.4), with a fallback to the
	// separate CountUsers call when the page is empty and the total is unknown.
	ListUsers(ctx context.Context, limit, offset int) ([]User, int, error)
	// UpdateRole changes a user's role. The last-owner guard is atomic:
	// the owner count and the role write share one transaction that
	// locks every owner row first (SELECT ... FOR UPDATE), so two
	// concurrent demotions serialize — the second re-evaluates the lock
	// after the first commits and can no longer count an owner that was
	// just demoted.
	UpdateRole(ctx context.Context, id, role string) (User, error)
	UpdateProfile(ctx context.Context, id string, patch UpdateProfilePatch) (User, error)
	// SearchUsers returns a page of username matches plus the total number of
	// matches. The total comes from COUNT(*) OVER () in the same query, with a
	// fallback count when the page is empty and the window function has no row
	// to read.
	SearchUsers(ctx context.Context, query string, limit, offset int) ([]User, int, error)
}

// UpdateProfilePatch carries the fields to change on the caller's own profile.
// A nil field means "leave unchanged"; AvatarURL and Socials are only ever set
// (never cleared) through this endpoint.
type UpdateProfilePatch struct {
	Username  *string
	AvatarURL *string
	Socials   *map[string]any
}

// pgCodeUniqueViolation is the PostgreSQL SQLSTATE for unique_violation.
// A named constant because pgerrcode is not a dependency.
const pgCodeUniqueViolation = "23505"

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}
