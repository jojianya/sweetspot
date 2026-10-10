package reactions

// Service-level tests with a stub repository. The database rules (trigger,
// cascade, idempotency) are covered by
// endpointtest/good_spot_trigger_db_test.go; these pin the service's decisions
// so a rule change is visible without a database.

import (
	"context"
	"errors"
	"testing"
)

type stubRepo struct {
	exists  bool
	owner   *string
	rows    int
	reacted bool

	reactCalls   int
	unreactCalls int

	reactErr   error
	unreactErr error
	countErr   error
	existsErr  error
	ownerErr   error
}

func (r *stubRepo) React(context.Context, string, string) error {
	r.reactCalls++
	return r.reactErr
}

func (r *stubRepo) Unreact(context.Context, string, string) error {
	r.unreactCalls++
	return r.unreactErr
}

func (r *stubRepo) ReactedByMe(context.Context, string, string) (bool, error) {
	return r.reacted, nil
}

func (r *stubRepo) Count(context.Context, string) (int, error) {
	return r.rows, r.countErr
}

func (r *stubRepo) PinExistsVisible(context.Context, string) (bool, error) {
	return r.exists, r.existsErr
}

func (r *stubRepo) PinOwner(context.Context, string) (*string, error) {
	return r.owner, r.ownerErr
}

func strPtr(s string) *string { return &s }

const (
	testUser = "user-1"
	testPin  = "pin-1"
)

func TestReactRules(t *testing.T) {
	owner := strPtr(testUser)

	cases := []struct {
		name    string
		repo    *stubRepo
		wantErr error
		// wantWrites reports whether the insert was allowed to run.
		wantWrites bool
	}{
		{
			name:       "react to someone else's pin",
			repo:       &stubRepo{exists: true, owner: strPtr("user-2"), rows: 1, reacted: true},
			wantWrites: true,
		},
		{
			// The rule the product asks for: an author cannot inflate their own
			// pin's count. The UI hides the button; this is the 403 backstop.
			name:       "own pin is refused",
			repo:       &stubRepo{exists: true, owner: owner},
			wantErr:    ErrOwnPin,
			wantWrites: false,
		},
		{
			name:       "hidden or missing pin is a 404",
			repo:       &stubRepo{exists: false, owner: owner},
			wantErr:    ErrNotFound,
			wantWrites: false,
		},
		{
			// An orphan pin (author deleted, user_id SET NULL) has no owner, so
			// it can never be the caller's own pin.
			name:       "orphan pin with a nil owner",
			repo:       &stubRepo{exists: true, owner: nil, rows: 1, reacted: true},
			wantWrites: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewService(tc.repo).React(context.Background(), testUser, testPin)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				if tc.repo.reactCalls != 0 {
					t.Error("a rejected reaction still wrote to the database")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tc.wantWrites && tc.repo.reactCalls != 1 {
				t.Errorf("react calls = %d, want 1", tc.repo.reactCalls)
			}
			if got.GoodSpotCount != tc.repo.rows {
				t.Errorf("count = %d, want %d", got.GoodSpotCount, tc.repo.rows)
			}
			if got.Reacted != tc.repo.reacted {
				t.Errorf("reacted = %v, want %v", got.Reacted, tc.repo.reacted)
			}
		})
	}
}

// TestReactIsIdempotent covers the double-tap case at the service level. The
// repository absorbs the duplicate with ON CONFLICT DO NOTHING, so the second
// call is a successful no-op that reports the same state.
func TestReactIsIdempotent(t *testing.T) {
	repo := &stubRepo{exists: true, owner: strPtr("user-2"), rows: 1, reacted: true}
	svc := NewService(repo)

	first, err := svc.React(context.Background(), testUser, testPin)
	if err != nil {
		t.Fatalf("first react: %v", err)
	}
	second, err := svc.React(context.Background(), testUser, testPin)
	if err != nil {
		t.Fatalf("second react: %v", err)
	}

	if repo.reactCalls != 2 {
		t.Errorf("react calls = %d, want 2 (both attempts are legitimate)", repo.reactCalls)
	}
	if first != second {
		t.Errorf("state changed between taps: %+v vs %+v", first, second)
	}
	if first.GoodSpotCount != 1 {
		t.Errorf("count = %d, want 1 — a double tap must not double count", first.GoodSpotCount)
	}
}

func TestUnreactRules(t *testing.T) {
	owner := strPtr(testUser)

	t.Run("unreact a reaction that exists", func(t *testing.T) {
		repo := &stubRepo{exists: true, owner: strPtr("user-2"), rows: 0, reacted: false}
		got, err := NewService(repo).Unreact(context.Background(), testUser, testPin)
		if err != nil {
			t.Fatalf("unreact: %v", err)
		}
		if repo.unreactCalls != 1 {
			t.Errorf("unreact calls = %d, want 1", repo.unreactCalls)
		}
		if got.Reacted {
			t.Error("reacted = true after an undo, want false")
		}
	})

	// Undoing a reaction that was never there, or one another tab already
	// removed, is a race — not a failure. The end state is the one asked for.
	t.Run("unreact an absent reaction is not an error", func(t *testing.T) {
		repo := &stubRepo{exists: true, owner: strPtr("user-2"), rows: 0, reacted: false}
		got, err := NewService(repo).Unreact(context.Background(), testUser, testPin)
		if err != nil {
			t.Fatalf("err = %v, want nil — an absent reaction is a valid end state", err)
		}
		if got.Reacted {
			t.Error("reacted = true, want false")
		}
	})

	t.Run("hidden or missing pin is a 404", func(t *testing.T) {
		repo := &stubRepo{exists: false, owner: owner}
		_, err := NewService(repo).Unreact(context.Background(), testUser, testPin)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want %v", err, ErrNotFound)
		}
		if repo.unreactCalls != 0 {
			t.Error("a refused unreact still wrote to the database")
		}
	})

	// React and Unreact must agree about what exists: if React refuses an own
	// pin, Unreact must too, or a caller could clear a count they cannot set.
	t.Run("own pin is refused here too", func(t *testing.T) {
		repo := &stubRepo{exists: true, owner: owner}
		_, err := NewService(repo).Unreact(context.Background(), testUser, testPin)
		if !errors.Is(err, ErrOwnPin) {
			t.Fatalf("err = %v, want %v", err, ErrOwnPin)
		}
	})
}

// TestRepositoryErrorsPropagate keeps a repository failure from being mistaken
// for a business rule: an error must never be reported as a 404 or a 403.
func TestRepositoryErrorsPropagate(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name string
		repo *stubRepo
	}{
		{"exists lookup fails", &stubRepo{existsErr: boom}},
		{"owner lookup fails", &stubRepo{exists: true, ownerErr: boom}},
		{"insert fails", &stubRepo{exists: true, owner: strPtr("user-2"), reactErr: boom}},
		{"count read fails", &stubRepo{exists: true, owner: strPtr("user-2"), countErr: boom}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewService(tc.repo).React(context.Background(), testUser, testPin)
			if err == nil {
				t.Fatal("err = nil, want the repository's error")
			}
			if errors.Is(err, ErrNotFound) || errors.Is(err, ErrOwnPin) {
				t.Fatalf("repository error %q was reported as a business rule", err)
			}
		})
	}
}
