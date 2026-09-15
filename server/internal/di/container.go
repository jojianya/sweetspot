package di

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/config"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

type Container struct {
	Config      *config.Config
	UserService users.Service
	PinRepo     pins.Repository
	ReportRepo  reports.Repository
	Store       *storage.Local
	Blacklist   *cache.Blacklist
}

func Build(cfg *config.Config, pool *pgxpool.Pool) *Container {
	return &Container{
		Config:      cfg,
		UserService: users.NewService(users.NewRepository(pool)),
		PinRepo:     pins.NewRepository(pool),
		ReportRepo:  reports.NewRepository(pool),
		Store:       storage.NewLocal("./uploads", cfg.StorageBase),
		Blacklist:   cache.New(cfg.RedisAddr),
	}
}
