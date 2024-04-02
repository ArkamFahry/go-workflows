package postgres

import (
	"context"
	"fmt"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/core"
	"github.com/jackc/pgx/v5"
	"time"
)

func (b *postgresBackend) GetStats(ctx context.Context) (*backend.Stats, error) {
	s := &backend.Stats{}

	tx, err := b.db.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get active instances
	row, err := tx.Query(
		ctx,
		"SELECT COUNT(*) FROM instances i WHERE i.completed_at IS NULL",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query active instances: %w", err)
	}

	defer row.Close()

	if err := row.Err(); err != nil {
		return nil, fmt.Errorf("failed to query active instances: %w", err)
	}

	var activeInstances int64
	if row.Next() {
		if err := row.Scan(&activeInstances); err != nil {
			return nil, fmt.Errorf("failed to scan active instances: %w", err)
		}
	}

	s.ActiveWorkflowInstances = activeInstances

	// Get workflow instances ready to be picked up
	now := time.Now()
	row, err = tx.Query(
		ctx,
		`SELECT COUNT(*)
			FROM instances i
			INNER JOIN pending_events pe ON i.instance_id = pe.instance_id
			WHERE
				state = $1 AND i.completed_at IS NULL
				AND (pe.visible_at IS NULL OR pe.visible_at <= $2)
				AND (i.locked_until IS NULL OR i.locked_until < $3)
			LIMIT 1
			FOR UPDATE OF i SKIP LOCKED`,
		core.WorkflowInstanceStateActive,
		now, // event.visible_at
		now, // locked_until
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query active instances: %w", err)
	}

	defer row.Close()

	if err := row.Err(); err != nil {
		return nil, fmt.Errorf("failed to query active instances: %w", err)
	}

	var pendingInstances int64
	if row.Next() {
		if err := row.Scan(&pendingInstances); err != nil {
			return nil, fmt.Errorf("failed to scan active instances: %w", err)
		}
	}

	s.PendingWorkflowTasks = pendingInstances

	// Get pending activities
	row, err = tx.Query(
		ctx,
		"SELECT COUNT(*) FROM activities")
	if err != nil {
		return nil, fmt.Errorf("failed to query active activities: %w", err)
	}

	defer row.Close()

	if err := row.Err(); err != nil {
		return nil, fmt.Errorf("failed to query active activities: %w", err)
	}

	var pendingActivities int64
	if row.Next() {
		if err := row.Scan(&pendingActivities); err != nil {
			return nil, fmt.Errorf("failed to scan active activities: %w", err)
		}
	}

	s.PendingActivities = pendingActivities

	return s, nil
}
