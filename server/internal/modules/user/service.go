package users

import "context"

type Service interface {
	Create(ctx context.Context, email, passwordHash, username string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByUsername(ctx context.Context, username string) (User, error)
	GetByLogin(ctx context.Context, identifier string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
	CountUsers(ctx context.Context) (int, error)
	ListUsers(ctx context.Context, limit, offset int) ([]User, error)
	UpdateRole(ctx context.Context, actorID, userID, role string) (User, error)
	UpdateProfile(ctx context.Context, id string, patch UpdateProfilePatch) (User, error)
	SearchUsers(ctx context.Context, query string, limit int) ([]User, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, email, passwordHash, username string) (User, error) {
	return s.repo.Create(ctx, email, passwordHash, username)
}

func (s *service) GetByEmail(ctx context.Context, email string) (User, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *service) GetByUsername(ctx context.Context, username string) (User, error) {
	return s.repo.GetByUsername(ctx, username)
}

func (s *service) GetByLogin(ctx context.Context, identifier string) (User, error) {
	return s.repo.GetByLogin(ctx, identifier)
}

func (s *service) GetByID(ctx context.Context, id string) (User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) SearchUsers(ctx context.Context, query string, limit int) ([]User, error) {
	return s.repo.SearchUsers(ctx, query, limit)
}

func (s *service) CountUsers(ctx context.Context) (int, error) {
	return s.repo.CountUsers(ctx)
}

func (s *service) ListUsers(ctx context.Context, limit, offset int) ([]User, error) {
	return s.repo.ListUsers(ctx, limit, offset)
}

func (s *service) UpdateRole(ctx context.Context, actorID, userID, role string) (User, error) {
	if actorID != "" && actorID == userID {
		return User{}, ErrCannotChangeOwnRole
	}

	target, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return User{}, err
	}

	if target.Role == RoleOwner && role != RoleOwner {
		owners, err := s.repo.CountOwners(ctx)
		if err != nil {
			return User{}, err
		}
		if owners <= 1 {
			return User{}, ErrCannotDemoteLastOwner
		}
	}

	return s.repo.UpdateRole(ctx, userID, role)
}

func (s *service) UpdateProfile(ctx context.Context, id string, patch UpdateProfilePatch) (User, error) {
	return s.repo.UpdateProfile(ctx, id, patch)
}
