package users

import (
	"context"
	"errors"
	"testing"
)

type mockRepository struct {
	users             map[string]User
	owners            int
	created           User
	createErr         error
	byEmailErr        error
	updateErr         error
	updateProfile     User
	updateProfileErr  error
	search            []User
	searchErr         error
	list              []User
	count             int
}

func (m *mockRepository) Create(ctx context.Context, email, passwordHash, username string) (User, error) {
	if m.createErr != nil {
		return User{}, m.createErr
	}
	return m.created, nil
}

func (m *mockRepository) GetByEmail(ctx context.Context, email string) (User, error) {
	if m.byEmailErr != nil {
		return User{}, m.byEmailErr
	}
	return m.created, nil
}

func (m *mockRepository) GetByUsername(ctx context.Context, username string) (User, error) {
	if m.byEmailErr != nil {
		return User{}, m.byEmailErr
	}
	return m.created, nil
}

func (m *mockRepository) GetByLogin(ctx context.Context, identifier string) (User, error) {
	if m.byEmailErr != nil {
		return User{}, m.byEmailErr
	}
	return m.created, nil
}

func (m *mockRepository) GetByID(_ context.Context, id string) (User, error) {
	u, ok := m.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (m *mockRepository) CountOwners(_ context.Context) (int, error) {
	return m.owners, nil
}

func (m *mockRepository) UpdateRole(_ context.Context, id, role string) (User, error) {
	if m.updateErr != nil {
		return User{}, m.updateErr
	}
	u := m.users[id]
	u.Role = role
	return u, nil
}

func (m *mockRepository) SearchUsers(_ context.Context, _ string, _ int) ([]User, error) {
	return m.search, m.searchErr
}

func (m *mockRepository) ListUsers(_ context.Context, _ int, _ int) ([]User, error) {
	return m.list, nil
}

func (m *mockRepository) CountUsers(_ context.Context) (int, error) {
	return m.count, nil
}

func (m *mockRepository) UpdateProfile(_ context.Context, _ string, _ UpdateProfilePatch) (User, error) {
	if m.updateProfileErr != nil {
		return User{}, m.updateProfileErr
	}
	return m.updateProfile, nil
}

func newTestService(repo Repository) Service {
	return NewService(repo)
}

func TestUpdateRoleCannotChangeOwnRole(t *testing.T) {
	svc := newTestService(&mockRepository{})
	_, err := svc.UpdateRole(context.Background(), "u1", "u1", RoleAdmin)
	if !errors.Is(err, ErrCannotChangeOwnRole) {
		t.Fatalf("expected ErrCannotChangeOwnRole, got %v", err)
	}
}

func TestUpdateRoleTargetNotFound(t *testing.T) {
	svc := newTestService(&mockRepository{users: map[string]User{}})
	_, err := svc.UpdateRole(context.Background(), "u2", "u1", RoleAdmin)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateRoleCannotDemoteLastOwner(t *testing.T) {
	svc := newTestService(&mockRepository{
		users: map[string]User{
			"u1": {ID: "u1", Role: RoleOwner},
		},
		owners: 1,
	})
	_, err := svc.UpdateRole(context.Background(), "u2", "u1", RoleUser)
	if !errors.Is(err, ErrCannotDemoteLastOwner) {
		t.Fatalf("expected ErrCannotDemoteLastOwner, got %v", err)
	}
}

func TestUpdateRoleDemoteOwnerWhenOthersExist(t *testing.T) {
	svc := newTestService(&mockRepository{
		users: map[string]User{
			"u1": {ID: "u1", Role: RoleOwner},
			"u3": {ID: "u3", Role: RoleOwner},
		},
		owners: 2,
	})
	updated, err := svc.UpdateRole(context.Background(), "u2", "u1", RoleUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Role != RoleUser {
		t.Fatalf("expected role %q, got %q", RoleUser, updated.Role)
	}
}

func TestUpdateRolePromoteOwnersUnrestricted(t *testing.T) {
	svc := newTestService(&mockRepository{
		users: map[string]User{
			"u1": {ID: "u1", Role: RoleUser},
		},
		owners: 0,
	})
	updated, err := svc.UpdateRole(context.Background(), "u2", "u1", RoleOwner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Role != RoleOwner {
		t.Fatalf("expected role %q, got %q", RoleOwner, updated.Role)
	}
}

func TestSearchUsersDelegatesToRepository(t *testing.T) {
	svc := newTestService(&mockRepository{
		search: []User{
			{ID: "u1", Username: "alice", Role: RoleUser},
			{ID: "u2", Username: "alicia", Role: RoleAdmin},
		},
	})
	got, err := svc.SearchUsers(context.Background(), "ali", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].Username != "alice" || got[1].Role != RoleAdmin {
		t.Fatalf("unexpected results: %+v", got)
	}
}

func TestListUsersDelegatesToRepository(t *testing.T) {
	svc := newTestService(&mockRepository{
		list: []User{
			{ID: "u1", Username: "alice", Role: RoleUser},
			{ID: "u2", Username: "bob", Role: RoleAdmin},
		},
		count: 2,
	})
	got, err := svc.ListUsers(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].Username != "alice" || got[1].Role != RoleAdmin {
		t.Fatalf("unexpected results: %+v", got)
	}
	n, err := svc.CountUsers(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("expected count 2, got %d (err %v)", n, err)
	}
}

func TestUpdateProfileDelegatesToRepository(t *testing.T) {
	username := "newname"
	avatar := "http://local/uploads/a.webp"
	socials := map[string]any{"instagram": "@newname"}
	svc := newTestService(&mockRepository{
		updateProfile: User{ID: "u1", Username: username, AvatarURL: &avatar, Socials: socials},
	})
	got, err := svc.UpdateProfile(context.Background(), "u1", UpdateProfilePatch{
		Username:  &username,
		AvatarURL: &avatar,
		Socials:   &socials,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Username != username || got.AvatarURL == nil || *got.AvatarURL != avatar {
		t.Fatalf("unexpected user: %+v", got)
	}
	if got.Socials["instagram"] != "@newname" {
		t.Fatalf("unexpected socials: %+v", got.Socials)
	}
}

func TestUpdateProfilePropagatesUsernameTaken(t *testing.T) {
	svc := newTestService(&mockRepository{updateProfileErr: ErrUsernameTaken})
	name := "taken"
	_, err := svc.UpdateProfile(context.Background(), "u1", UpdateProfilePatch{Username: &name})
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}
}
