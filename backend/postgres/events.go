package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/core"
	"strings"
)

func insertPendingEvents(ctx context.Context, tx *sql.Tx, instance *core.WorkflowInstance, newEvents []*history.Event) error {
	return insertEvents(ctx, tx, "pending_events", instance, newEvents)
}

func insertHistoryEvents(ctx context.Context, tx *sql.Tx, instance *core.WorkflowInstance, historyEvents []*history.Event) error {
	return insertEvents(ctx, tx, "history", instance, historyEvents)
}

func insertEvents(ctx context.Context, tx *sql.Tx, tableName string, instance *core.WorkflowInstance, events []*history.Event) error {
	const batchSize = 20
	for batchStart := 0; batchStart < len(events); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(events) {
			batchEnd = len(events)
		}
		batchEvents := events[batchStart:batchEnd]

		aquery := fmt.Sprintf("INSERT INTO attributes (event_id, instance_id, execution_id, data) VALUES ($1, $2, $3, $4) %s ON CONFLICT ON CONSTRAINT idx_attributes_instance_id_execution_id_event_id DO NOTHING;",
			strings.Repeat(", ($1, $2, $3, $4)", len(batchEvents)-1))
		aargs := make([]interface{}, 0, len(batchEvents)*4)

		query := fmt.Sprintf("INSERT INTO %s (event_id, sequence_id, instance_id, execution_id, event_type, timestamp, schedule_event_id, visible_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) %s",
			tableName,
			strings.Repeat(", ($1, $2, $3, $4, $5, $6, $7, $8)", len(batchEvents)-1),
		)

		args := make([]interface{}, 0, len(batchEvents)*8)

		for _, newEvent := range batchEvents {
			a, err := history.SerializeAttributes(newEvent.Attributes)
			if err != nil {
				return err
			}

			aargs = append(aargs, newEvent.ID, instance.InstanceID, instance.ExecutionID, a)

			args = append(
				args,
				newEvent.ID, newEvent.SequenceID, instance.InstanceID, instance.ExecutionID, newEvent.Type, newEvent.Timestamp, newEvent.ScheduleEventID, newEvent.VisibleAt)
		}

		if _, err := tx.ExecContext(
			ctx,
			aquery,
			aargs...,
		); err != nil {
			return fmt.Errorf("inserting attributes: %w", err)
		}

		_, err := tx.ExecContext(
			ctx,
			query,
			args...,
		)
		if err != nil {
			return fmt.Errorf("inserting events: %w", err)
		}
	}

	return nil
}

func removeFutureEvent(ctx context.Context, tx *sql.Tx, instance *core.WorkflowInstance, scheduleEventID int64) error {
	_, err := tx.ExecContext(
		ctx,
		`DELETE FROM attributes WHERE event_id IN (SELECT event_id FROM pending_events WHERE instance_id = $1 AND execution_id = $2 AND schedule_event_id = $3 AND visible_at IS NOT NULL);
				DELETE FROM pending_events WHERE instance_id = $1 AND execution_id = $2 AND schedule_event_id = $3 AND visible_at IS NOT NULL;`,
		instance.InstanceID,
		instance.ExecutionID,
		scheduleEventID,
	)

	return err
}
