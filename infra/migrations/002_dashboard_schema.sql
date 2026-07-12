CREATE TABLE IF NOT EXISTS flags (
    id                SERIAL PRIMARY KEY,
    sensor_id         TEXT NOT NULL,
    field_name        TEXT,
    flag_type         TEXT NOT NULL,
    message           TEXT NOT NULL,
    first_detected_at TIMESTAMPTZ NOT NULL,
    last_detected_at  TIMESTAMPTZ NOT NULL,
    acknowledged      BOOLEAN NOT NULL DEFAULT false,
    acknowledged_at   TIMESTAMPTZ,
    acknowledged_by   TEXT,
    note              TEXT
);

-- Enforces only one unacknowledged flag per condition at a time.
-- Once acknowledged, the same condition can spawn a new flag row.
CREATE UNIQUE INDEX IF NOT EXISTS idx_active_flags 
ON flags (sensor_id, field_name, flag_type) 
WHERE acknowledged = false;
