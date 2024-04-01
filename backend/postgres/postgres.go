package postgres

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/converter"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/metadata"
	"github.com/cschleiden/go-workflows/backend/metrics"
	"github.com/cschleiden/go-workflows/core"
	"github.com/cschleiden/go-workflows/internal/metrickeys"
	"github.com/cschleiden/go-workflows/internal/workflowerrors"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"time"
)

//go:embed db/migrations/*.sql
var migrationsFS embed.FS

func NewPostgresBackend(host string, port int, user, password, database string, opts ...option) *postgresBackend {
	options := &options{
		Options:         backend.ApplyOptions(),
		ApplyMigrations: true,
	}

	for _, opt := range opts {
		opt(options)
	}

	dsn := fmt.Sprintf("postgres://%s:%d@%s:%s/%s", host, port, user, password, database)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}

	if options.PostgresOptions != nil {
		options.PostgresOptions(db)
	}

	b := &postgresBackend{
		dsn:        dsn,
		db:         db,
		workerName: fmt.Sprintf("worker-%v", uuid.NewString()),
		options:    options,
	}

	if options.ApplyMigrations {
		if err := b.Migrate(); err != nil {
			panic(err)
		}
	}

	return b
}

type postgresBackend struct {
	dsn        string
	db         *sql.DB
	workerName string
	options    *options
}

func (b *postgresBackend) Close() error {
	return b.db.Close()
}

func (b *postgresBackend) Migrate() error {
	schemaDsn := b.dsn
	db, err := sql.Open("postgres", schemaDsn)
	if err != nil {
		return fmt.Errorf("opening schema database: %w", err)
	}

	dbi, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("creating migration instance: %w", err)
	}

	migrations, err := iofs.New(migrationsFS, "db/migrations")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", migrations, "postgres", dbi)
	if err != nil {
		return fmt.Errorf("creating migration: %w", err)
	}

	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("running migrations: %w", err)
		}
	}

	if err := db.Close(); err != nil {
		return fmt.Errorf("closing schema database: %w", err)
	}

	return nil
}

func (b *postgresBackend) Logger() *slog.Logger {
	return b.options.Logger
}

func (b *postgresBackend) Tracer() trace.Tracer {
	return b.options.TracerProvider.Tracer(backend.TracerName)
}

func (b *postgresBackend) Metrics() metrics.Client {
	return b.options.Metrics.WithTags(metrics.Tags{metrickeys.Backend: "postgres"})
}

func (b *postgresBackend) Converter() converter.Converter {
	return b.options.Converter
}

func (b *postgresBackend) ContextPropagators() []workflow.ContextPropagator {
	return b.options.ContextPropagators
}

func (b *postgresBackend) CreateWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Create workflow instance
	if err := createInstance(ctx, tx, instance, event.Attributes.(*history.ExecutionStartedAttributes).Metadata); err != nil {
		return err
	}

	// Initial history is empty, store only new events
	if err := insertPendingEvents(ctx, tx, instance, []*history.Event{event}); err != nil {
		return fmt.Errorf("inserting new event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("creating workflow instance: %w", err)
	}

	return nil
}

func (b *postgresBackend) RemoveWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `SELECT state FROM instances WHERE instance_id = $1 AND execution_id = $2 LIMIT 1`, instance.InstanceID, instance.ExecutionID)
	var state core.WorkflowInstanceState
	if err := row.Scan(&state); err != nil {
		if err == sql.ErrNoRows {
			return backend.ErrInstanceNotFound
		}
	}

	if state == core.WorkflowInstanceStateActive {
		return backend.ErrInstanceNotFinished
	}

	// Delete from instances and history tables
	if _, err := tx.ExecContext(ctx, `DELETE FROM instances WHERE instance_id = $1 AND execution_id = $2`, instance.InstanceID, instance.ExecutionID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM history WHERE instance_id = $1 AND execution_id = $2`, instance.InstanceID, instance.ExecutionID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM attributes WHERE instance_id = $1 AND execution_id = $2`, instance.InstanceID, instance.ExecutionID); err != nil {
		return err
	}

	return tx.Commit()
}

func (b *postgresBackend) CancelWorkflowInstance(ctx context.Context, instance *workflow.Instance, event *history.Event) error {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Cancel workflow instance
	// TODO: Combine this with the event insertion
	res := tx.QueryRowContext(ctx, `SELECT 1 FROM instances WHERE instance_id = $1 AND execution_id = $2 LIMIT 1`, instance.InstanceID, instance.ExecutionID)
	if err := res.Scan(new(int)); err != nil {
		if err == sql.ErrNoRows {
			return backend.ErrInstanceNotFound
		}

		return err
	}

	if err := insertPendingEvents(ctx, tx, instance, []*history.Event{event}); err != nil {
		return fmt.Errorf("inserting cancellation event: %w", err)
	}

	return tx.Commit()
}

func (b *postgresBackend) GetWorkflowInstanceHistory(ctx context.Context, instance *workflow.Instance, lastSequenceID *int64) ([]*history.Event, error) {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var historyEvents *sql.Rows
	if lastSequenceID != nil {
		historyEvents, err = tx.QueryContext(
			ctx,
			`SELECT h.event_id, h.sequence_id, h.event_type, h.timestamp, h.schedule_event_id, a.data, h.visible_at FROM history h JOIN attributes a ON h.event_id = a.event_id AND a.instance_id = h.instance_id AND a.execution_id = h.execution_id WHERE h.instance_id = $1 AND h.execution_id = $2 AND h.sequence_id > $3 ORDER BY h.sequence_id`,
			instance.InstanceID,
			instance.ExecutionID,
			*lastSequenceID,
		)
	} else {
		historyEvents, err = tx.QueryContext(
			ctx,
			`SELECT h.event_id, h.sequence_id, h.event_type, h.timestamp, h.schedule_event_id, a.data, h.visible_at FROM history h JOIN attributes a ON h.event_id = a.event_id AND a.instance_id = h.instance_id AND a.execution_id = h.execution_id WHERE h.instance_id = $1 AND h.execution_id = $2 ORDER BY h.sequence_id`,
			instance.InstanceID,
			instance.ExecutionID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("getting history: %w", err)
	}

	defer historyEvents.Close()

	h := make([]*history.Event, 0)

	for historyEvents.Next() {
		var attributes []byte

		historyEvent := &history.Event{}

		if err := historyEvents.Scan(
			&historyEvent.ID,
			&historyEvent.SequenceID,
			&historyEvent.Type,
			&historyEvent.Timestamp,
			&historyEvent.ScheduleEventID,
			&attributes,
			&historyEvent.VisibleAt,
		); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}

		a, err := history.DeserializeAttributes(historyEvent.Type, attributes)
		if err != nil {
			return nil, fmt.Errorf("deserializing attributes: %w", err)
		}

		historyEvent.Attributes = a

		h = append(h, historyEvent)
	}

	return h, nil
}

func (b *postgresBackend) GetWorkflowInstanceState(ctx context.Context, instance *workflow.Instance) (core.WorkflowInstanceState, error) {
	row := b.db.QueryRowContext(
		ctx,
		`SELECT state FROM instances WHERE instance_id = $1 AND execution_id = $2`,
		instance.InstanceID,
		instance.ExecutionID,
	)

	var state core.WorkflowInstanceState
	if err := row.Scan(&state); err != nil {
		if err == sql.ErrNoRows {
			return core.WorkflowInstanceStateActive, backend.ErrInstanceNotFound
		}
	}

	return state, nil
}

func createInstance(ctx context.Context, tx *sql.Tx, wfi *workflow.Instance, metadata *workflow.Metadata) error {
	// Check for existing instance
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM instances WHERE instance_id = $1 AND state = $2 LIMIT 1`, wfi.InstanceID, core.WorkflowInstanceStateActive).
		Scan(new(int)); err != sql.ErrNoRows {
		return backend.ErrInstanceAlreadyExists
	}

	var parentInstanceID, parentExecutionID *string
	var parentEventID *int64
	if wfi.SubWorkflow() {
		parentInstanceID = &wfi.Parent.InstanceID
		parentExecutionID = &wfi.Parent.ExecutionID
		parentEventID = &wfi.ParentEventID
	}

	metadataJson, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO instances (instance_id, execution_id, parent_instance_id, parent_execution_id, parent_schedule_event_id, metadata, state) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		wfi.InstanceID,
		wfi.ExecutionID,
		parentInstanceID,
		parentExecutionID,
		parentEventID,
		string(metadataJson),
		core.WorkflowInstanceStateActive,
	)
	if err != nil {
		return fmt.Errorf("inserting workflow instance: %w", err)
	}

	return nil
}

func (b *postgresBackend) SignalWorkflow(ctx context.Context, instanceID string, event *history.Event) error {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// TODO: Combine this with the event insertion
	res := tx.QueryRowContext(ctx, `SELECT execution_id FROM instances WHERE instance_id = $1 AND state = $2 LIMIT 1`, instanceID, core.WorkflowInstanceStateActive)
	var executionID string
	if err := res.Scan(&executionID); err == sql.ErrNoRows {
		return backend.ErrInstanceNotFound
	}

	instance := core.NewWorkflowInstance(instanceID, executionID)

	if err := insertPendingEvents(ctx, tx, instance, []*history.Event{event}); err != nil {
		return fmt.Errorf("inserting signal event: %w", err)
	}

	return tx.Commit()
}

func (b *postgresBackend) GetWorkflowTask(ctx context.Context) (*backend.WorkflowTask, error) {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock next workflow task by finding an unlocked instance with new events to process.
	now := time.Now()
	row := tx.QueryRowContext(
		ctx,
		`SELECT i.id, i.instance_id, i.execution_id, i.parent_instance_id, i.parent_execution_id, i.parent_schedule_event_id, i.metadata, i.sticky_until
			FROM instances i
			INNER JOIN pending_events pe ON i.instance_id = pe.instance_id
			WHERE
				state = $1 AND i.completed_at IS NULL
				AND (pe.visible_at IS NULL OR pe.visible_at <= $2)
				AND (i.locked_until IS NULL OR i.locked_until < $3)
				AND (i.sticky_until IS NULL OR i.sticky_until < $4 OR i.worker = $5)
			LIMIT 1
			FOR UPDATE OF i SKIP LOCKED`,
		core.WorkflowInstanceStateActive, // state
		now,                              // event.visible_at
		now,                              // locked_until
		now,                              // sticky_until
		b.workerName,                     // worker
	)

	var id int
	var instanceID, executionID string
	var parentInstanceID, parentExecutionID *string
	var parentEventID *int64
	var metadataJson sql.NullString
	var stickyUntil *time.Time
	if err := row.Scan(&id, &instanceID, &executionID, &parentInstanceID, &parentExecutionID, &parentEventID, &metadataJson, &stickyUntil); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, fmt.Errorf("scanning workflow instance: %w", err)
	}

	res, err := tx.ExecContext(
		ctx,
		`UPDATE instances i
			SET locked_until = $1, worker = $2
			WHERE id = $3`,
		now.Add(b.options.WorkflowLockTimeout),
		b.workerName,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("locking workflow instance: %w", err)
	}

	if affectedRows, err := res.RowsAffected(); err != nil {
		return nil, fmt.Errorf("locking workflow instance: %w", err)
	} else if affectedRows == 0 {
		// No instance locked?
		return nil, nil
	}

	var wfi *workflow.Instance
	if parentInstanceID != nil {
		wfi = core.NewSubWorkflowInstance(instanceID, executionID, core.NewWorkflowInstance(*parentInstanceID, *parentExecutionID), *parentEventID)
	} else {
		wfi = core.NewWorkflowInstance(instanceID, executionID)
	}

	var metadata *metadata.WorkflowMetadata
	if metadataJson.Valid {
		if err := json.Unmarshal([]byte(metadataJson.String), &metadata); err != nil {
			return nil, fmt.Errorf("parsing workflow metadata: %w", err)
		}
	}

	t := &backend.WorkflowTask{
		ID:                    wfi.InstanceID,
		WorkflowInstance:      wfi,
		WorkflowInstanceState: core.WorkflowInstanceStateActive,
		Metadata:              metadata,
		NewEvents:             []*history.Event{},
	}

	// Get new events
	events, err := tx.QueryContext(
		ctx,
		`SELECT pe.event_id, pe.sequence_id, pe.event_type, pe.timestamp, pe.schedule_event_id, a.data, pe.visible_at FROM pending_events pe LEFT JOIN attributes a ON pe.instance_id = a.instance_id AND pe.execution_id = a.execution_id AND pe.event_id = a.event_id WHERE pe.instance_id = $1 AND pe.execution_id = $2 AND (pe.visible_at IS NULL OR pe.visible_at <= $3) ORDER BY pe.id`,
		instanceID,
		executionID,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("getting new events: %w", err)
	}

	defer events.Close()

	for events.Next() {
		var attributes []byte

		historyEvent := &history.Event{}

		if err := events.Scan(
			&historyEvent.ID,
			&historyEvent.SequenceID,
			&historyEvent.Type,
			&historyEvent.Timestamp,
			&historyEvent.ScheduleEventID,
			&attributes,
			&historyEvent.VisibleAt,
		); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}

		a, err := history.DeserializeAttributes(historyEvent.Type, attributes)
		if err != nil {
			return nil, fmt.Errorf("deserializing attributes: %w", err)
		}

		historyEvent.Attributes = a

		t.NewEvents = append(t.NewEvents, historyEvent)
	}

	// Return if there aren't any new events
	if len(t.NewEvents) == 0 {
		return nil, nil
	}

	// Get most recent sequence id
	var lastSequenceID sql.NullInt64
	row = tx.QueryRowContext(ctx, `SELECT MAX(sequence_id) FROM history WHERE instance_id = $1 AND execution_id = $2`, instanceID, executionID)
	if err := row.Scan(
		&lastSequenceID,
	); err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("getting most recent sequence id: %w", err)
		}
	}

	if lastSequenceID.Valid {
		t.LastSequenceID = lastSequenceID.Int64
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return t, nil
}

func (b *postgresBackend) CompleteWorkflowTask(
	ctx context.Context,
	task *backend.WorkflowTask,
	instance *workflow.Instance,
	state core.WorkflowInstanceState,
	executedEvents, activityEvents, timerEvents []*history.Event,
	workflowEvents []history.WorkflowEvent,
) error {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unlock instance, but keep it sticky to the current worker
	var completedAt *time.Time
	if state == core.WorkflowInstanceStateContinuedAsNew || state == core.WorkflowInstanceStateFinished {
		t := time.Now()
		completedAt = &t
	}

	res, err := tx.ExecContext(
		ctx,
		`UPDATE instances SET locked_until = NULL, sticky_until = $1, completed_at = $2, state = $3 WHERE instance_id = $4 AND execution_id = $5 AND worker = $6`,
		time.Now().Add(b.options.StickyTimeout),
		completedAt,
		state,
		instance.InstanceID,
		instance.ExecutionID,
		b.workerName,
	)
	if err != nil {
		return fmt.Errorf("unlocking instance: %w", err)
	}

	changedRows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking for unlocked workflow instances: %w", err)
	} else if changedRows != 1 {
		return errors.New("could not find workflow instance to unlock")
	}

	// Remove handled events from task
	if len(executedEvents) > 0 {
		args := make([]interface{}, 0, len(executedEvents)+1)
		args = append(args, instance.InstanceID, instance.ExecutionID)
		for _, e := range executedEvents {
			args = append(args, e.ID)
		}

		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM pending_events WHERE instance_id = $1 AND execution_id = $2 AND event_id = ANY($3)`,
			args...,
		); err != nil {
			return fmt.Errorf("deleting handled new events: %w", err)
		}
	}

	// Insert new events generated during this workflow execution to the history
	if err := insertHistoryEvents(ctx, tx, instance, executedEvents); err != nil {
		return fmt.Errorf("inserting new history events: %w", err)
	}

	// Schedule activities
	for _, e := range activityEvents {
		if err := scheduleActivity(ctx, tx, instance, e); err != nil {
			return fmt.Errorf("scheduling activity: %w", err)
		}
	}

	// Timer events
	if err := insertPendingEvents(ctx, tx, instance, timerEvents); err != nil {
		return fmt.Errorf("scheduling timers: %w", err)
	}

	for _, event := range executedEvents {
		switch event.Type {
		case history.EventType_TimerCanceled:
			if err := removeFutureEvent(ctx, tx, instance, event.ScheduleEventID); err != nil {
				return fmt.Errorf("removing future event: %w", err)
			}
		}
	}

	// Insert new workflow events
	groupedEvents := history.EventsByWorkflowInstance(workflowEvents)

	for targetInstance, events := range groupedEvents {
		// Are we creating a new sub-workflow instance?
		m := events[0]
		if m.HistoryEvent.Type == history.EventType_WorkflowExecutionStarted {
			a := m.HistoryEvent.Attributes.(*history.ExecutionStartedAttributes)
			// Create new instance
			if err := createInstance(ctx, tx, m.WorkflowInstance, a.Metadata); err != nil {
				if errors.Is(err, backend.ErrInstanceAlreadyExists) {
					if err := insertPendingEvents(ctx, tx, instance, []*history.Event{
						history.NewPendingEvent(time.Now(), history.EventType_SubWorkflowFailed, &history.SubWorkflowFailedAttributes{
							Error: workflowerrors.FromError(backend.ErrInstanceAlreadyExists),
						}, history.ScheduleEventID(m.WorkflowInstance.ParentEventID)),
					}); err != nil {
						return fmt.Errorf("inserting sub-workflow failed event: %w", err)
					}

					continue
				}

				return fmt.Errorf("creating sub-workflow instance: %w", err)
			}
		}

		// Insert pending events for target instance
		historyEvents := []*history.Event{}
		for _, m := range events {
			historyEvents = append(historyEvents, m.HistoryEvent)
		}
		if err := insertPendingEvents(ctx, tx, &targetInstance, historyEvents); err != nil {
			return fmt.Errorf("inserting messages: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing complete workflow transaction: %w", err)
	}

	return nil
}

func (b *postgresBackend) ExtendWorkflowTask(ctx context.Context, taskID string, instance *core.WorkflowInstance) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	until := time.Now().Add(b.options.WorkflowLockTimeout)
	res, err := tx.ExecContext(
		ctx,
		`UPDATE instances SET locked_until = $1 WHERE instance_id = $2 AND execution_id = $3 AND worker = $4`,
		until,
		instance.InstanceID,
		instance.ExecutionID,
		b.workerName,
	)
	if err != nil {
		return fmt.Errorf("extending workflow task lock: %w", err)
	}

	if rowsAffected, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("determining if workflow task was extended: %w", err)
	} else if rowsAffected == 0 {
		return errors.New("could not extend workflow task")
	}

	return tx.Commit()
}

func (b *postgresBackend) GetActivityTask(ctx context.Context) (*backend.ActivityTask, error) {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock next activity
	now := time.Now()
	res := tx.QueryRowContext(
		ctx,
		`SELECT a.id, a.activity_id, a.instance_id, a.execution_id,
			a.event_type, a.timestamp, a.schedule_event_id, at.data, a.visible_at
			FROM activities a
			JOIN attributes at ON at.event_id = a.activity_id AND at.instance_id = a.instance_id AND at.execution_id = a.execution_id
			WHERE a.locked_until IS NULL OR a.locked_until < $1
			LIMIT 1
			FOR UPDATE SKIP LOCKED`,
		now,
	)

	var id int64
	var instanceID, executionID string
	var attributes []byte
	event := &history.Event{}

	if err := res.Scan(
		&id, &event.ID, &instanceID, &executionID, &event.Type,
		&event.Timestamp, &event.ScheduleEventID, &attributes, &event.VisibleAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("finding activity task to lock: %w", err)
	}

	a, err := history.DeserializeAttributes(event.Type, attributes)
	if err != nil {
		return nil, fmt.Errorf("deserializing attributes: %w", err)
	}

	event.Attributes = a

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE activities SET locked_until = $1, worker = $2 WHERE id = $3`,
		now.Add(b.options.ActivityLockTimeout),
		b.workerName,
		id,
	); err != nil {
		return nil, fmt.Errorf("locking activity: %w", err)
	}

	t := &backend.ActivityTask{
		ID:               event.ID,
		WorkflowInstance: core.NewWorkflowInstance(instanceID, executionID),
		Event:            event,
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return t, nil
}

func (b *postgresBackend) CompleteActivityTask(ctx context.Context, instance *workflow.Instance, id string, event *history.Event) error {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove activity
	if res, err := tx.ExecContext(
		ctx,
		`DELETE FROM activities WHERE activity_id = $1 AND instance_id = $2 AND execution_id = $3 AND worker = $4`,
		id,
		instance.InstanceID,
		instance.ExecutionID,
		b.workerName,
	); err != nil {
		return fmt.Errorf("completing activity: %w", err)
	} else {
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("checking for completed activity: %w", err)
		}

		if affected == 0 {
			return errors.New("could not find locked activity")
		}
	}

	// Insert new event generated during this workflow execution
	if err := insertPendingEvents(ctx, tx, instance, []*history.Event{event}); err != nil {
		return fmt.Errorf("inserting new events for completed activity: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (b *postgresBackend) ExtendActivityTask(ctx context.Context, activityID string) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	until := time.Now().Add(b.options.ActivityLockTimeout)
	_, err = tx.ExecContext(
		ctx,
		`UPDATE activities SET locked_until = $1 WHERE activity_id = $2 AND worker = $3`,
		until,
		activityID,
		b.workerName,
	)
	if err != nil {
		return fmt.Errorf("extending activity lock: %w", err)
	}

	if err := tx.Commit(); err != nil {
		if errors.Is(err, sql.ErrTxDone) {
			return nil
		}

		return err
	}

	return nil
}

func scheduleActivity(ctx context.Context, tx *sql.Tx, instance *core.WorkflowInstance, event *history.Event) error {
	// Attributes are already persisted via the history, we do not need to add them again.

	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO activities (activity_id, instance_id, execution_id, event_type, timestamp, schedule_event_id, visible_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		event.ID,
		instance.InstanceID,
		instance.ExecutionID,
		event.Type,
		event.Timestamp,
		event.ScheduleEventID,
		event.VisibleAt,
	)

	return err
}
