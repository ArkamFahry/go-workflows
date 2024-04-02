package postgres

import (
	"github.com/cschleiden/go-workflows/backend"
	"github.com/jackc/pgx/v5/pgxpool"
)

type options struct {
	backend.Options

	PostgresOptions func(db *pgxpool.Pool)

	// ApplyMigrations automatically applies database migrations on startup.
	ApplyMigrations bool
}

type option func(*options)

// WithApplyMigrations automatically applies database migrations on startup.
func WithApplyMigrations(applyMigrations bool) option {
	return func(o *options) {
		o.ApplyMigrations = applyMigrations
	}
}

// WithPostgresOptions allows to pass a custom postgres options
func WithPostgresOptions(f func(db *pgxpool.Pool)) option {
	return func(o *options) {
		o.PostgresOptions = f
	}
}

// WithBackendOptions allows to pass generic backend options.
func WithBackendOptions(opts ...backend.BackendOption) option {
	return func(o *options) {
		for _, opt := range opts {
			opt(&o.Options)
		}
	}
}
