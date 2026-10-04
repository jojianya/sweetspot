package endpointtest

import (
	"context"
	"testing"

	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
)

// TestLegacyUsernameStillLogsInAndIsFound pins the migration-safety half of the
// username character rule: the rule guards where a username is *chosen*, never
// where one is *looked up*, so an account that already carries a username the
// rule would reject keeps working. The rows are written with raw SQL, bypassing
// validation, which is exactly what a pre-existing account looks like.
//
// Everything below the handler runs for real — the pg repository, the users
// service and the auth service — so this exercises the actual login path rather
// than a stub's idea of it.
func TestLegacyUsernameStillLogsInAndIsFound(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()

	legacy := []struct {
		username string
		email    string
	}{
		{"legacy user", "legacy-space@example.com"},   // space
		{"<script>alice", "legacy-angle@example.com"}, // angle brackets
		{"аlice", "legacy-cyrillic@example.com"},      // Cyrillic "а"
	}

	hash, err := password.Hash("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	var ids []string
	for _, l := range legacy {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO users (id, email, username, password_hash, role)
			VALUES (gen_random_uuid(), $1, $2, $3, 'user')
			RETURNING id
		`, l.email, l.username, hash).Scan(&id); err != nil {
			t.Fatalf("insert legacy username %q: %v", l.username, err)
		}
		ids = append(ids, id)
	}
	defer func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1)`, ids); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}()

	userRepo := users.NewRepository(pool)
	userSvc := users.NewService(userRepo)
	authSvc := auth.NewService(userSvc, testSecret)

	for _, l := range legacy {
		// Lookup by the exact legacy username still resolves.
		got, err := userRepo.GetByUsername(ctx, l.username)
		if err != nil {
			t.Errorf("GetByUsername(%q) = %v, want the account", l.username, err)
			continue
		}
		if got.Username != l.username {
			t.Errorf("GetByUsername(%q) returned username %q", l.username, got.Username)
		}

		// And a login by that username still authenticates: Login resolves the
		// identifier without running the character rule.
		if _, _, err := authSvc.Login(ctx, auth.LoginRequest{
			Identifier: l.username,
			Password:   "password123",
		}); err != nil {
			t.Errorf("Login(%q) = %v, want success", l.username, err)
		}
	}
}
