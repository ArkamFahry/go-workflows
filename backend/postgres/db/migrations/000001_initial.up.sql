BEGIN;

CREATE TABLE IF NOT EXISTS instances
(
    id                       BIGSERIAL PRIMARY KEY,
    instance_id              VARCHAR(128) NOT NULL,
    execution_id             VARCHAR(128) NOT NULL,
    parent_instance_id       VARCHAR(128),
    parent_execution_id      VARCHAR(128),
    parent_schedule_event_id BIGINT,
    metadata                 BYTEA,
    state                    INT          NOT NULL,
    created_at               TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at             TIMESTAMP,
    locked_until             TIMESTAMP,
    sticky_until             TIMESTAMP,
    worker                   VARCHAR(64)
);

CREATE UNIQUE INDEX idx_instances_instance_id_execution_id ON instances (instance_id, execution_id);
CREATE INDEX idx_instances_locked_until_completed_at ON instances (completed_at, locked_until, sticky_until, worker);
CREATE INDEX idx_instances_parent_instance_id_parent_execution_id ON instances (parent_instance_id, parent_execution_id);

CREATE TABLE IF NOT EXISTS pending_events
(
    id                BIGSERIAL PRIMARY KEY,
    event_id          VARCHAR(128) NOT NULL,
    sequence_id       BIGINT       NOT NULL, -- not used but keep for now for query compat
    instance_id       VARCHAR(128) NOT NULL,
    execution_id      VARCHAR(128) NOT NULL,
    event_type        INT          NOT NULL,
    timestamp         TIMESTAMP    NOT NULL,
    schedule_event_id BIGINT       NOT NULL,
    attributes        BYTEA        NOT NULL,
    visible_at        TIMESTAMP
);

CREATE INDEX idx_pending_events_inid_exid ON pending_events (instance_id, execution_id);
CREATE INDEX idx_pending_events_inid_exid_visible_at_schedule_event_id ON pending_events (instance_id, execution_id, visible_at, schedule_event_id);

CREATE TABLE IF NOT EXISTS history
(
    id                BIGSERIAL PRIMARY KEY,
    event_id          VARCHAR(64)  NOT NULL,
    sequence_id       BIGINT       NOT NULL,
    instance_id       VARCHAR(128) NOT NULL,
    execution_id      VARCHAR(128) NOT NULL,
    event_type        INT          NOT NULL,
    timestamp         TIMESTAMP    NOT NULL,
    schedule_event_id BIGINT       NOT NULL,
    attributes        BYTEA        NOT NULL,
    visible_at        TIMESTAMP
);

CREATE INDEX idx_history_instance_id_execution_id ON history (instance_id, execution_id);
CREATE INDEX idx_history_instance_id_execution_id_sequence_id ON history (instance_id, execution_id, sequence_id);

CREATE TABLE IF NOT EXISTS activities
(
    id                BIGSERIAL PRIMARY KEY,
    activity_id       VARCHAR(64)  NOT NULL,
    instance_id       VARCHAR(128) NOT NULL,
    execution_id      VARCHAR(128) NOT NULL,
    event_type        INT          NOT NULL,
    timestamp         TIMESTAMP    NOT NULL,
    schedule_event_id BIGINT       NOT NULL,
    visible_at        TIMESTAMP,
    locked_until      TIMESTAMP,
    worker            VARCHAR(64)
);

CREATE UNIQUE INDEX idx_activities_instance_id_execution_id_activity_id_worker ON activities (instance_id, execution_id, activity_id, worker);
CREATE INDEX idx_activities_locked_until ON activities (locked_until);

CREATE TABLE IF NOT EXISTS attributes
(
    id           BIGSERIAL PRIMARY KEY,
    event_id     VARCHAR(128) NOT NULL,
    instance_id  VARCHAR(128) NOT NULL,
    execution_id VARCHAR(128) NOT NULL,
    data         BYTEA        NOT NULL
);

CREATE UNIQUE INDEX idx_attributes_instance_id_execution_id_event_id ON attributes (instance_id, execution_id, event_id);
CREATE INDEX idx_attributes_event_id ON attributes (event_id);

COMMIT;
