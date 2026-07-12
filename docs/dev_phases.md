# Adaptive ETL Engine — Development Phases

A sequenced build plan. Each phase produces something testable before the next phase adds complexity. Phases are ordered by dependency, not by importance — later phases often depend entirely on the stability of earlier ones.

---

## Phase 0 — Sensor Data Generators

Build simulated sensor sources before touching the engine itself. Every later phase needs real data to test against, and the generators must simulate a range of behaviors, not just clean uniform output. This is a standalone tool, decoupled from the engine, that outputs to whatever the ingestion layer expects — mirroring the engine's own boundary philosophy. Reusable later for load testing in Phase 10.

**Deliverables:**
- Multiple sensor types firing at different fixed rates (e.g. 10Hz, 15Hz, once-per-minute) to give Phase 6's fusion logic something genuine to solve
- Stable baseline behavior with small natural noise, for realistic deadband testing
- Occasional anomaly spikes injected on a schedule or randomly, to test deadband pass-through and cold start anomaly detection
- Conditional/threshold-triggered sensors that stay silent until a simulated condition is breached, then fire — needed for Tier 1 and ping testing
- A dropout simulator — a generator that stops emitting mid-run, to test heartbeat, LOCF staleness, and dashboard flagging
- A schema drift simulator — starts with N fields, later adds or changes one, to stress-test Phase 9 before it exists
- A multi-source mode that runs several simulated machines/sensors at once with different sensor counts and shapes, to test adaptive inference against more than one data shape simultaneously

---

## Phase 1 — Core Pipe (Foundation)

Build the simplest possible version of the engine's contract: receive a JSON payload, return a JSON payload. No intelligence yet.

**Deliverables:**
- Basic ingestion endpoint and output endpoint
- Message broker setup (Kafka or equivalent) for reliable data flow
- Offset commits so no data is lost on restart
- No filtering, no schema awareness, no tiers — purely proving the pipe works end to end with data from Phase 0's generators

---

## Phase 2 — Schema Inference

Add dynamic schema discovery. No filtering decisions yet, just observation and structure-building.

**Deliverables:**
- Engine reads incoming payloads and builds a schema from field names and types without pre-configuration
- YAML config parser and validator built here, since schema and config need to interact from the start
- Dry-run config validation mode

---

## Phase 3 — Cold Start and Checklist

Build the checklist-based dynamic learning window.

**Deliverables:**
- Sensor ID discovery
- Field discovery per sensor
- Sample rate calculation
- Baseline distribution building (mean, variance, min/max per field)
- Dynamic ETA calculation based on ingress rate, bottlenecked by the slowest pending checklist item
- Two-level verification stage — manual confirmation and auto-verify-from-config paths
- Per-item timeout fallback for checklist items that never complete

This phase produces a working baseline for every learned field. Phase 4 depends on this entirely.

---

## Phase 4 — Deadband and Heartbeat

With baselines available, implement the actual filtering logic. This is the first demo-able milestone showing real bandwidth savings.

**Deliverables:**
- Deadband threshold evaluation per field, auto-calculated from baseline or manually overridden via config
- Heartbeat emission at configurable intervals to solve the silent sensor problem
- LOCF on the output side for stream continuity
- Developer override controls — disable deadband or heartbeat per sensor/field

---

## Phase 5 — Tiering and Conditional Sensors

Implement the three-tier classification system.

**Deliverables:**
- Critical / Standard / Flagged tier classification logic
- Ping mechanism for conditional sensors during cold start
- Manual declaration path for sensors that don't support ping
- Tier-aware routing wired into Phase 4's filtering logic, ensuring critical sensors bypass deadband, baseline, and LOCF entirely

---

## Phase 6 — Sensor Fusion and Windowing

Build the Redis-backed stateful latest-value join to handle asynchronous, multi-rate sensors.

**Deliverables:**
- Windowing logic (e.g. tumbling window) for clubbing sensor readings by timestamp and ID
- Redis state cache lookups to fill gaps from slower/faster sensors
- Stitched, complete rows passed into Phase 4's deadband evaluation
- Flag marking for fused fields that were gap-filled vs. natively present in the window

Depends on Phase 4's baseline and deadband logic being stable, since fused rows feed directly into it.

---

## Phase 7 — Resilience

Make the engine production-credible rather than a demo script.

**Deliverables:**
- Checkpointing for cold start state at each completed checklist milestone
- Checkpointing for normal operation state (schema, baseline, Redis cache contents)
- Restart recovery logic — detect checkpoint type and resume appropriately rather than restarting from zero
- Power loss / unclean shutdown handling
- Dashboard flag for unclean restart events, with timestamp of the gap

---

## Phase 8 — Flagging and Dashboard (Completed)

Build the flag emission logic and the human-facing review layer. Sequenced near the end since flags depend on most other components already existing to have something to flag.

**Deliverables:**
- Flag emission for: schema contradictions, sensor dropout, unrecognized sensor ID, stalled checklist items, unresponsive conditional sensors, undeclared new fields, prolonged LOCF/stale values, high anomaly rate during cold start, unclean restarts
- Dashboard UI showing only flags — no other engine data
- Human acknowledgment workflow with required free-text note
- Audit trail — permanent log of every flag, who acknowledged it, when, and their note
- Visual distinction between acknowledged and unacknowledged flags

---

## Phase 9 — Partial Updates and Schema Drift (Completed)

One of the harder architectural pieces, intentionally sequenced after the core system is stable.

**Deliverables:**
- Isolated mini cold start scoped to a single newly added sensor or field, without disrupting active processing elsewhere
- Schema drift detection — contradiction handling between live data and committed schema
- Promotion logic for a flagged field to "learned" status once it passes its own isolated checklist
- Integration with Phase 0's schema drift simulator for validation

---

## Phase 10 — Testing and Defense Prep (Completed)

**Deliverables:**
- Load testing using Phase 0's generators, including multi-source mode at scale
- Failure injection testing for power-loss and restart scenarios from Phase 7
- Metrics collection: bandwidth reduction percentage, cold start time across varying data densities, flag response and acknowledgment times
- Defense documentation: architecture walkthrough, hypothesis validation against collected metrics, known limitations and design boundary explanation

---

## Suggested Cut Line

If time is constrained, Phases 0 through 6 represent the architecture's core identity and should be fully built and demoable. Phases 7 through 9 are what separate a working prototype from a defensible production-grade design — if time runs short, these can be designed on paper in detail even if not fully implemented, and that level of thinking will still hold up in a defense setting.
