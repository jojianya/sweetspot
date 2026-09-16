package users

import (
	"context"
	"errors"
	"testing"
)

type mockRepository struct {
	users      map[string]User
	owners     int
	created    User
	createErr  error
	byEmailErr error
	updateErr  error
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
