CREATE TABLE IF NOT EXISTS cold_start_checkpoints (
    id              SERIAL PRIMARY KEY,
    sensor_id       TEXT NOT NULL,
    milestone       TEXT NOT NULL,
    completed_at    TIMESTAMPTZ NOT NULL,
    metadata        JSONB
);

CREATE TABLE IF NOT EXISTS engine_lifecycle (
    id          SERIAL PRIMARY KEY,
    event       TEXT NOT NULL, -- 'heartbeat', 'shutdown_clean', 'startup'
    occurred_at TIMESTAMPTZ NOT NULL
);
