package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"testing"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/google/uuid"
)

const testHost = "localhost"
const testPort = 5432
const testUser = "root"
const testPassword = "root"
const testDatabase = "test"
const testSchema = "public"

// Creating and dropping databases is terribly inefficient, but easiest for complete test isolation. For
// the future consider nested transactions, or manually TRUNCATE-ing the tables in-between tests.

func Test_PostgresBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	var dbName string

	test.BackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		db, err := pgxpool.New(context.Background(), fmt.Sprintf("postgres://%s:%d@%s:%s/postgres", testHost, testPort, testUser, testPassword))
		if err != nil {
			panic(err)
		}

		dbName = fmt.Sprintf("%s_%s", testDatabase, strings.Replace(uuid.NewString(), "-", "", -1))
		if _, err := db.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
			panic(fmt.Errorf("creating database: %w", err))
		}

		db.Close()

		options = append(options, backend.WithStickyTimeout(0))

		return NewPostgresBackend("localhost", 3306, testUser, testPassword, dbName, testSchema, WithBackendOptions(options...))
	}, func(b test.TestBackend) {
		b.(*postgresBackend).db.Close()

		db, err := pgxpool.New(context.Background(), fmt.Sprintf("postgres://%s:%d@%s:%s/postgres", testHost, testPort, testUser, testPassword))
		if err != nil {
			panic(err)
		}

		if _, err := db.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)); err != nil {
			panic(fmt.Errorf("dropping database: %w", err))
		}

		db.Close()
	})
}

func TestPostgresBackendE2E(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	var dbName string

	test.EndToEndBackendTest(t, func(options ...backend.BackendOption) test.TestBackend {
		db, err := pgxpool.New(context.Background(), fmt.Sprintf("postgres://%s:%d@%s:%s/postgres", testHost, testPort, testUser, testPassword))
		if err != nil {
			panic(err)
		}

		dbName = fmt.Sprintf("%s_%s", testDatabase, strings.Replace(uuid.NewString(), "-", "", -1))
		if _, err := db.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
			panic(fmt.Errorf("creating database: %w", err))
		}

		db.Close()

		options = append(options, backend.WithStickyTimeout(0))

		return NewPostgresBackend("localhost", 3306, testUser, testPassword, dbName, testSchema, WithBackendOptions(options...))
	}, func(b test.TestBackend) {
		b.(*postgresBackend).db.Close()

		db, err := pgxpool.New(context.Background(), fmt.Sprintf("postgres://%s:%d@%s:%s/postgres", testHost, testPort, testUser, testPassword))
		if err != nil {
			panic(err)
		}

		if _, err := db.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)); err != nil {
			panic(fmt.Errorf("dropping database: %w", err))
		}

		db.Close()
	})
}

var _ test.TestBackend = (*postgresBackend)(nil)

func (b *postgresBackend) GetFutureEvents(ctx context.Context) ([]*history.Event, error) {
	tx, err := b.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// There is no index on `visible_at`, but this is okay for test only usage.
	futureEvents, err := tx.Query(
		ctx,
		"SELECT pe.id, pe.sequence_id, pe.instance_id, pe.execution_id, pe.event_type, pe.timestamp, pe.schedule_event_id, pe.visible_at, a.data FROM pending_events pe JOIN attributes a ON a.id = pe.id AND a.instance_id = pe.instance_id AND a.execution_id = pe.execution_id WHERE pe.visible_at IS NOT NULL",
	)
	if err != nil {
		return nil, fmt.Errorf("getting history: %w", err)
	}

	defer futureEvents.Close()

	f := make([]*history.Event, 0)

	for futureEvents.Next() {
		var instanceID string
		var attributes []byte

		fe := &history.Event{}

		if err := futureEvents.Scan(
			&fe.ID,
			&fe.SequenceID,
			&instanceID,
			&fe.Type,
			&fe.Timestamp,
			&fe.ScheduleEventID,
			&attributes,
			&fe.VisibleAt,
		); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}

		a, err := history.DeserializeAttributes(fe.Type, attributes)
		if err != nil {
			return nil, fmt.Errorf("deserializing attributes: %w", err)
		}

		fe.Attributes = a

		f = append(f, fe)
	}

	return f, nil
}
