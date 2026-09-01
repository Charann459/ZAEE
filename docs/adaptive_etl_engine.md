# Adaptive ETL Engine — Capstone Design Document

## Project Hypothesis

A real-time, middleware-deployed ETL engine that utilizes dynamic schema inference to ingest heterogeneous data streams, applying deadband compression and priority-tiered filtering to eliminate redundant data, reduce cloud transmission overhead, and deliver normalized, enriched payloads — without any assumption about the upstream source or downstream destination.

**Implementation Note (Phase 10 Complete):** This design document reflects the original architectural intent. Over the course of the project (Phases 1-10), certain decisions evolved. Specifically: Schema Drift employs a 25-sample checklist and oscillation prevention; LOCF uses a hard timeout to prevent staleness; the Dashboard is built with FastAPI/SSE rather than a Go backend. For defense preparation and exact implementation details, please see `docs/defense_prep.md` and `docs/technical_architecture.md`.

---

## 1. What This Engine Is

The Adaptive ETL Engine is a streaming middleware layer that sits between data ingestion and storage. It receives a JSON payload, processes it, and returns a JSON payload. That is the entire contract.

It does not care what generated the data — IoT sensors, race car telemetry, stock market APIs, factory machines, or anything else. It does not care where the output goes — AWS S3, Lambda, a database, another service. Everything outside the pipe is the developer's responsibility. The engine's job is to be the filter between the pump and the tank.

This boundary is intentional and a core design principle. It is what makes the engine universally applicable across industries and use cases without modification.

---

## 2. Core Architecture

### 2.1 Position in the Pipeline

```
[ Any Data Source ]
        |
        v
[ Ingestion Layer ]  ← developer's responsibility
        |
        v
┌───────────────────────────────┐
│     ADAPTIVE ETL ENGINE       │  ← this engine
│                               │
│  Cold Start → Schema Learn    │
│  Tier Classification          │
│  Deadband Filtering           │
│  LOCF + Sensor Fusion         │
│  Normalization                │
│  Flag Emission                │
└───────────────────────────────┘
        |
        v
[ Storage / Hot-Cold Path ]  ← developer's responsibility
```

The engine receives raw JSON in, returns cleaned normalized JSON out. It has no knowledge of and no dependency on anything upstream or downstream.

### 2.2 What the Engine Actually Does

- Dynamically infers and builds the schema from live data
- Classifies each sensor into a priority tier
- Applies deadband compression to suppress redundant readings
- Performs Last Observation Carried Forward (LOCF) to handle silent sensors
- Aligns asynchronous sensor timestamps using a stateful cache
- Emits a flag stream to the dashboard for any anomaly requiring human review
- Allows developers to override any automated behavior via a YAML config

---

## 3. Data Priority Tiers

Every sensor and every reading in the system belongs to exactly one of three tiers. The tier determines how the engine handles the data.

### Tier 1 — Critical

Conditional or safety sensors that only fire on threshold breach or event trigger. These represent the most important data points in the system.

- Bypass all ETL processing — no deadband, no baseline, no filtering, no windowing, no joins
- No baseline is established because by definition these sensors only fire in abnormal conditions
- Passed through to output immediately upon receipt with zero processing overhead and zero added latency
- Stamped with a critical flag in the output payload so downstream consumers treat it with urgency
- Filtering a safety alert because it does not breach a baseline would be a catastrophic design flaw — the tier system prevents this structurally

### Tier 2 — Standard

Active sensors that stream continuously. The full ETL pipeline applies.

- Deadband filtering
- LOCF
- Timestamp alignment and sensor fusion
- Windowing and joins
- Normalization

### Tier 3 — Flagged

Sensors that were expected but never confirmed during cold start, or sensors whose data contradicts the known schema.

- Passed through as-is without modification
- A flag is emitted to the dashboard indicating the engine has no schema knowledge about this field
- Downstream consumers are warned but not blocked
- Flagged sensors require human acknowledgment before the flag is cleared

---

## 4. Cold Start Window

When the engine is freshly installed against a new data source, it cannot immediately begin filtering. It needs time to learn the data before it can make intelligent decisions about what is normal and what is anomalous.

### 4.1 Purpose

The cold start window serves multiple purposes simultaneously:

- **Schema discovery** — detect what sensors exist, what fields each emits, at what rate
- **Baseline establishment** — build a statistical distribution per field (mean, variance, min, max)
- **Health verification** — confirm sensors are actually working, not just present
- **Reliability assessment** — distinguish stable signals from noisy or broken ones

During the cold start window, all data passes through to output unfiltered. The engine observes but does not suppress.

### 4.2 Checklist-Based Exit

The cold start does not end on a fixed timer. It ends when the engine is confident, measured by a checklist of conditions that must all be satisfied. The estimated time to completion is calculated dynamically based on current data ingress — more fields, more sensors, higher variance, longer the window.

**Checklist items in approximate completion order:**

1. **Sensor ID discovery** — all sensor IDs detected and either confirmed against config or flagged for manual review
2. **Field discovery per sensor** — all fields for each sensor seen at least once
3. **Sample rate stabilization** — enough cycles observed per sensor to calculate a reliable Hz estimate
4. **Baseline distribution** — minimum 50–100 samples per field to calculate a statistically meaningful mean and variance
5. **Anomaly rate assessment** — enough data to distinguish normal variance from genuine spikes
6. **Schema stability** — no new fields appeared in the last N readings, confirming discovery is complete

**ETA calculation:** at any point during cold start, the engine looks at the slowest uncompleted checklist item and projects completion time based on current ingress rate. The overall ETA is the maximum across all pending items — the bottleneck is always the slowest signal, not the average.

**Baseline is a distribution, not a single value.** A sensor that fluctuates between 88 and 92 normally has a different deadband than one that sits at a fixed 90. The engine learns this distinction during the window and auto-sets thresholds accordingly.

### 4.3 Partial Updates

When a new sensor is added to a running system, the engine does not restart. It opens a mini cold start scoped only to the new sensor. Everything else keeps running uninterrupted. The new sensor goes through its own checklist independently and joins the active pool when it completes.

### 4.4 Verification Stage

Cold start includes a two-level verification step to prevent schema errors from going unnoticed.

**Level 1 — Sensor verification:** Engine presents all discovered sensor IDs. Developer confirms these are the correct sensors and nothing is missing or unexpected.

**Level 2 — Field verification:** For each confirmed sensor, engine presents all discovered fields. Developer confirms each field is expected and correctly typed.

Only after both levels are confirmed does the engine commit the schema and begin baseline learning and filtering.

**Automated verification path:** If a developer provides a complete config upfront declaring sensor IDs, field names, field types, and sensor tiers, the engine skips manual verification and auto-confirms against the config. Any discrepancy between the config and what the engine actually detects generates an immediate flag.

### 4.5 Sensors That Cannot Be Learned

Some sensors only fire when a threshold is breached — safety valves, pressure relief sensors, emergency triggers. The engine cannot learn these through observation because they may never fire during the cold start window.

**Two handling paths:**

If the sensor supports ping, the engine sends a health probe during cold start — not to get readings, but to confirm the sensor is online and reachable. The sensor is marked as online, conditional, unlearned. No baseline is set.

If the sensor does not support ping, the developer manually registers it in the YAML config, declaring its sensor ID, field names, data types, and expected range when it does fire. The engine holds a slot open in the schema and trusts the declaration.

In both cases the sensor is classified as Tier 1 Critical and bypasses all filtering logic permanently.

---

## 5. Deadband Filtering

### 5.1 The Core Concept

If a sensor reads 90°C and continues reading 90°C for the next 100 milliseconds, transmitting all 100 readings is wasteful. Deadband filtering suppresses readings that fall within a tolerance threshold of the established baseline — only transmitting when something meaningfully changes.

**Deadband threshold** can be set per field in the YAML config. If not set, the engine auto-calculates an appropriate threshold from the baseline distribution established during cold start.

### 5.2 The Silent Sensor Problem

If the deadband filter suppresses all readings because a sensor is stable, the output looks identical to a sensor that has gone offline or lost connectivity. A blank stream and a stable stream are indistinguishable without additional information.

**Solution: Heartbeat mechanism.** Even when deadband suppresses readings, the engine emits a heartbeat at a configurable interval — a minimal payload confirming the sensor is alive and its last known value. The heartbeat interval is configurable per sensor in the YAML config.

**Solution: LOCF on the output side.** When a downstream dashboard receives no new reading, it carries the last known value forward rather than showing null. The stream appears continuous to analysts even when deadband is active.

**The combination** means: money is saved by not transmitting redundant data, but the analyst still sees a continuous real-time feed and knows immediately when a sensor genuinely goes offline versus simply being stable.

### 5.3 Developer Control

Developers have complete control over how deadband is applied. Via the YAML config:

- Set a specific threshold per field per sensor
- Set a heartbeat interval per sensor
- Disable deadband entirely for a specific sensor
- Disable LOCF for a specific sensor
- Override auto-calculated thresholds with manual values

No behavior is forced. Every automated decision can be overridden.

---

## 6. Asynchronous Sensor Fusion

### 6.1 The Problem

Not all sensors fire at the same rate. A temperature sensor might fire at 10Hz and a pressure sensor at 15Hz. Their timestamps will rarely align perfectly. Attempting to club their readings together using a rigid tumbling window produces Swiss Cheese data — rows with null values where the slower sensor missed the window.

### 6.2 The Solution — Stateful Latest-Value Join

The engine uses the Redis state cache (already present for deadband baseline storage) to fill in gaps before the deadband filter runs.

**The process:**

1. The stream processor opens a window (e.g. 100ms)
2. A reading arrives from the 10Hz sensor
3. The 15Hz sensor happens to miss this exact window
4. Before finalizing the clubbed row, the engine checks the expected schema and notices a field is missing
5. The engine queries Redis for the latest known value of the missing field
6. That value is stitched into the current row with a flag indicating it was LOCF-filled
7. The complete row is passed to the deadband filter
8. The deadband filter evaluates the unified, complete row

**Result:** downstream systems always receive perfectly structured, complete tabular rows regardless of how misaligned the raw sensor sampling rates are.

This is formally called Timestamp Alignment and Sensor Fusion. The engine does not just filter data — it guarantees structural completeness before passing anything downstream.

---

## 7. Schema Management

### 7.1 Dynamic Schema Inference

When a payload arrives from a new source, the engine reads the field names and types, recognizes the structure on the fly, and begins building a schema. No pre-configuration required for the engine to start ingesting. Configuration refines and overrides, it does not gate ingestion.

### 7.2 Schema Versioning and Drift

When a live schema changes — a sensor that previously sent 5 fields now sends 6, or a field is renamed — the engine does not restart. The new or changed field is isolated and goes through a mini cold start scoped only to that field. The rest of the schema continues operating uninterrupted.

If a live reading contradicts the committed schema — a field declared as float arrives as a string — the engine flags the contradiction immediately for human review, passes the payload through as-is with the flag attached, and does not attempt to silently coerce the value.

### 7.3 Schema States

Each field in the schema has one of three states:

- **Learned** — fully profiled during cold start, baseline and deadband set, active filtering applied
- **Conditional** — known to exist (via ping or manual declaration), no baseline because none is meaningful, always passes through without filtering
- **Flagged** — expected but never confirmed, or contradicts known schema, passes through with flag, requires human acknowledgment

---

## 8. The YAML Control Plane

The control plane is a YAML file that gives developers complete visibility and control over the engine's behavior. It is human readable, easy to version control in git, and simple enough for a small factory's IT person to edit without understanding engine internals.

### 8.1 Example Structure

```yaml
engine:
  cold_start:
    auto_verify: true         # skip manual verification if config is complete
  locf:
    enabled: true
  heartbeat:
    default_interval: 60s

sensors:
  - id: temp_sensor_01
    tier: standard
    fields:
      - name: temperature
        type: float
      - name: humidity
        type: float
    deadband:
      temperature: 0.5        # only transmit if value changes by more than 0.5
      humidity: 1.0
    heartbeat_interval: 30s

  - id: pressure_relief_valve
    tier: critical
    ping: true                # engine will ping for health check during cold start

  - id: legacy_vibration_sensor
    tier: standard
    fields:
      - name: vibration_rms
        type: float
    deadband:
      vibration_rms: null     # null = disable deadband for this field, always transmit
```

### 8.2 Config Validation

On startup, the engine validates the YAML before accepting it. A malformed or contradictory config is rejected with a clear error message. A dry run mode is available — parse and validate the config, report issues, without actually starting the engine.

---

## 9. Output Payload

The output is intentionally minimal. The engine's job is to clean and normalize data, not to wrap it in metadata.

**Standard output:**

```json
{
  "sensor_id": "temp_sensor_01",
  "timestamp": "2025-06-29T14:00:00.100Z",
  "tier": "standard",
  "fields": {
    "temperature": 91.2,
    "humidity": 54.0
  },
  "flags": null
}
```

**Output with flags:**

```json
{
  "sensor_id": "temp_sensor_01",
  "timestamp": "2025-06-29T14:00:00.100Z",
  "tier": "standard",
  "fields": {
    "temperature": 91.2,
    "humidity": "54.0"
  },
  "flags": {
    "humidity": "type_mismatch: expected float, received string"
  }
}
```

**Critical tier output:**

```json
{
  "sensor_id": "pressure_relief_valve",
  "timestamp": "2025-06-29T14:00:00.100Z",
  "tier": "critical",
  "fields": {
    "pressure_psi": 847.3
  },
  "flags": null
}
```

The `flags` field is null when everything is clean and populated only when the engine has something to report. Downstream consumers can check for null and move on in the common case.

LOCF-filled fields are not flagged in the output payload — that is expected, normal behavior. LOCF is only flagged on the dashboard if a sensor has been silent long enough that the carried value is becoming stale beyond a configurable threshold.

---

## 10. The Dashboard

The dashboard has one job: display flags. It is the engine's operational visibility layer and nothing else.

### 10.1 Flag Types

- Schema contradictions — config says float, received string
- Sensor dropout — sensor not seen since timestamp X
- Unrecognized sensor ID — a new sensor ID appeared that was not declared
- Cold start checklist stalled — a checklist item is taking unexpectedly long
- Conditional sensor not responding to ping
- New undeclared field detected on a known sensor
- LOCF active too long — sensor has been silent beyond the stale threshold
- Anomaly rate high during cold start — may indicate a malfunctioning sensor being used to set baseline

### 10.2 Human Acknowledgment

Every flag requires a human to manually acknowledge it before it is cleared. This is non-negotiable.

**Why this matters:**

Every flag has a paper trail. Someone saw it, someone made a decision, that decision is recorded. The acknowledgment includes a free text note field where the reviewer explains what they found or decided. That note is attached to the flag permanently.

Over time this log becomes genuinely valuable operational intelligence — a sensor that flags every Tuesday morning, a field type mismatch acknowledged 12 times before anyone fixed the source, a conditional sensor that repeatedly fails ping checks before eventually going offline permanently.

**Compliance benefit:** In regulated industries — pharmaceutical manufacturing, food processing, medical devices — there is often a legal requirement to demonstrate that anomalies were detected, reviewed, and actioned. The acknowledgment system with its audit trail satisfies this requirement out of the box without any extra effort from the developer using the engine. This was not designed for compliance, but it is a direct consequence of making the right engineering decision.

Unacknowledged flags are visually distinct from acknowledged ones. The dashboard makes it impossible to miss an open flag.

---

## 11. What This Is Formally Called

For project documentation, presentations, and defense:

**Dynamic Schema Inference / Schema-Agnostic Ingestion** — the engine adapts to any data source without pre-configuration, automatically discovering structure from live data.

**Deadband Compression / Exception-Based Reporting** — suppressing readings that fall within a tolerance threshold of the established baseline.

**Edge ETL / Streaming Middleware** — processing data in-flight before it reaches storage, rather than after.

**Stateful Latest-Value Join / Sensor Fusion** — using a cached state store to align asynchronous multi-rate sensor readings into complete unified rows.

**Last Observation Carried Forward (LOCF)** — filling silent sensor gaps with the last known good value to maintain stream continuity.

**Checklist-Based Dynamic Cold Start** — a self-timed learning window that exits on confidence, not on a fixed clock, with duration calculated from data ingress rate and complexity.

---

## 12. What the Engine Is Not Responsible For

To be explicit about the boundary:

- What generates the input data
- The network or protocol used to deliver data to the engine
- Authentication or authorization of data sources
- What happens to the output payload after it leaves the engine
- Cloud storage reliability, capacity, or failure handling
- Downstream dashboard or analytics tooling
- IoT device firmware or sensor hardware

These are all the developer's responsibility. The engine receives a JSON payload and returns a JSON payload. Everything else is outside scope.

---

## 13. Evaluation Summary — Strengths and Remaining Gaps

### Strengths

- Clean, enforceable boundary between the engine and everything else
- Production-grade deadband + heartbeat combination solving the silent sensor problem
- Developer control plane with no forced behaviors — every automated decision is overridable
- Three-tier priority system that structurally prevents safety data from ever being filtered
- Checklist-based dynamic cold start that is honest about confidence rather than hiding behind a fixed timer
- Partial update isolation that avoids costly full restarts on schema changes
- Two-path handling for conditional sensors covering both ping-capable and manually declared cases
- Human acknowledgment with audit trail that accidentally satisfies regulated industry compliance requirements
- YAML config that is simple enough for non-specialist operators

### Remaining Design Decisions

- **Schema drift behavior during live operation** — defined as flagged and isolated, but the exact trigger for promoting a flagged field to learned needs a precise rule
- **LOCF staleness threshold** — what is the configurable limit before a carried value is considered too stale to be useful, and what happens then
- **Dry run mode implementation depth** — how much of the engine runs in validation mode versus just config parsing
- **Output payload LOCF annotation** — currently LOCF is not surfaced in the output payload; if downstream systems need to know a value was carried rather than live, a mechanism is needed
- **Cold start anomaly handling** — if 40% of readings during cold start are outliers, does the engine flag and wait for human input before committing the baseline, or commit with a warning

---

*Document compiled from full design discussion — June 2025*
*Capstone Project: Adaptive Streaming ETL Middleware*
