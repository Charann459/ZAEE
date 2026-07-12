# Capstone Defense Prep: Answers to Open Questions

This document prepares answers for the five open questions flagged in the original design doc evaluation. It reflects the final implemented state of the engine.

### 1. Schema Drift Behavior During Live Operation
**Question:** How does the engine handle schema drift when a field's type changes during live operation?
**Answer:** The engine uses a robust drift checklist combined with a mini-cold start. When a type mismatch occurs, the field is flagged and its state enters an observation mode (`DriftType`). If the mismatch consistently occurs across `25` consecutive samples (the drift checklist threshold), the engine considers it a legitimate schema drift rather than a transient anomaly. At this point:
1. The field is promoted to the new type.
2. The old Redis SDT state (baseline mean/variance) is safely deleted via `DeleteSDTStateSafe()`.
3. A "schema_drift_promoted" flag is emitted to notify downstream systems.
4. If a field oscillates rapidly between multiple types, the engine resets the checklist entirely and sets the `DriftType` to the latest incoming type to ensure only stable drifts are promoted.

### 2. LOCF Staleness Threshold
**Question:** How long do we carry forward the Last Observation Carried Forward (LOCF) values before they are considered too stale?
**Answer:** The engine inextricably ties the LOCF staleness threshold to the Redis key TTL policy. In Phase 6, we established that Redis TTL is set to `2 × HeartbeatInterval`. Therefore, the staleness threshold is exactly `2 × HeartbeatInterval`. If a sensor completely drops out and fails to emit a heartbeat or reading within this window, the Redis key natively expires. Subsequent gap-fills will fail because the baseline key is missing, triggering a `field_unavailable` (or ultimately `sensor_dropout`) flag. This guarantees that LOCF data is never carried forward beyond the designed staleness window, preventing the system from hallucinating a healthy sensor.

### 3. Dry Run Mode Depth
**Question:** How deep does the config "dry run" mode go?
**Answer:** The dry run mode in Go thoroughly parses, validates, and simulates the configuration lifecycle without executing side effects or mutating external infrastructure (like Redis, Postgres, or Kafka). It ensures structural validity, checks for contradictory rules (e.g., conflicting tier assignments or impossible SDT door widths), and verifies that all dependencies can be resolved. This allows operators to test complex schema and tier configurations safely in CI/CD before deployment.

### 4. LOCF Annotation in Output Payload
**Question:** How do downstream consumers differentiate between real telemetry and LOCF gap-filled values?
**Answer:** The engine annotates LOCF values via the structured `flags` map in the minimal output payload. When a field's gap is filled, the output payload includes an entry in the `flags` object, such as `{"pressure": "locf_gap_filled"}`. We explicitly avoided polluting the top-level payload schema with synthetic boolean fields to maintain a clean schema. Downstream systems and the Dashboard can quickly inspect the `flags` map to distinguish between real telemetry and synthetically injected values.

### 5. Cold Start Anomaly Handling
**Question:** What happens if an anomaly occurs *during* the cold start period before the baseline is fully established?
**Answer:** During the cold start sequence, the engine is designed to pass all data through unfiltered. Deadband Compression (SDT) does not activate until the cold start is completely verified (`cold_start_active: false`), meaning the engine's primary job is simply to safely collect enough samples to build the mean and variance. However, if an anomaly heavily skews the incoming samples, the engine's built-in anomaly rate check evaluates the variance before committing the baseline. If a high percentage of readings look like extreme outliers, the system flags the cold start for human review rather than silently baking a corrupted baseline. Downstream consumers receive 100% of the raw data during this period and can apply their own checks until the engine transitions to steady-state.
