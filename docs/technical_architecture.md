# Adaptive ETL Engine — Technical Architecture & Stack

This document covers the technical implementation side of the project — language choices, component breakdown, deployment shape, and infrastructure. For the conceptual design (tiers, deadband logic, cold start, schema handling) see the main Design Document.

---

## 1. Stack Overview

| Layer | Language/Tech | Why |
|---|---|---|
| Core Engine | **Go** | High-throughput, low-latency, concurrent-by-default hot path. Single compiled binary, easy to containerize, no runtime dependency overhead. |
| Sensor Data Generators | **Python** | Not performance-critical. Fast to write, easy to add randomness, noise, and anomaly injection logic. |
| Dashboard Backend | **Python (FastAPI + aiokafka)** | Off the hot path. Fast to build CRUD/REST endpoints for flags and acknowledgments. Uses Server-Sent Events (SSE) via `EventBus` for real-time frontend updates. |
| Dashboard Frontend | **Vanilla HTML/JS/CSS** | Premium aesthetics with glassmorphism, completely native without heavy framework overhead. Connects to the SSE endpoint to stay live. |
| Message Broker | **Apache Kafka** | Durable, ordered, partition-based ingestion. Industry standard for streaming pipelines, defensible choice for a thesis panel. |
| State Cache | **Redis** | In-memory store for baseline values, LOCF state, and sensor fusion lookups. Matches the low-latency requirement of the deadband and join logic. |
| Audit/Flag Storage | **PostgreSQL (or SQLite for dev)** | Relational storage for the flag audit trail — acknowledgment, notes, timestamps. |
| Cold Path Storage (test) | **MinIO (S3-compatible) or local filesystem** | Stand-in for AWS S3 during development, avoids real cloud costs. |
| Config | **YAML** (`gopkg.in/yaml.v3`) | Already decided — human readable, git-friendly. |

**Two-language split:** Go owns everything in the engine's hot path and anything performance-sensitive. Python owns everything around it — generating test data, dashboard tooling, operator-facing surfaces. No third language introduced.

---

## 2. Go Library Choices (Core Engine)

| Need | Library |
|---|---|
| Kafka client | `confluent-kafka-go` or `segmentio/kafka-go` |
| Redis client | `go-redis/redis` |
| Statistical computation (baseline mean/variance/distribution) | `gonum.org/v1/gonum/stat` |
| YAML parsing | `gopkg.in/yaml.v3` |
| Concurrency | native goroutines + channels (no external library needed) |
| JSON handling | standard library `encoding/json`, or `json-iterator/go` if profiling shows it's a bottleneck |
| HTTP (if engine exposes any internal API) | standard library `net/http` or `gin-gonic/gin` if more routing structure is needed |

---

## 3. Component Breakdown

```
┌─────────────────────────────────────────────────────────┐
│                    SENSOR GENERATORS (Python)             │
│   - Multi-rate emitters  - Anomaly injector               │
│   - Dropout simulator    - Schema drift simulator         │
└───────────────────────────┬────────────────────────────────┘
                             │ produces JSON to
                             v
                    ┌─────────────────┐
                    │   Kafka Topic    │  (raw ingest)
                    └────────┬─────────┘
                             │
                             v
┌─────────────────────────────────────────────────────────┐
│                    CORE ENGINE (Go)                        │
│                                                             │
│  Consumer ──▶ Schema Inference ──▶ Cold Start Manager      │
│                                          │                  │
│                                          v                  │
│                              Tier Classifier                │
│                          (critical / standard / flagged)    │
│                                          │                  │
│            ┌─────────────────────────────┼───────────────┐ │
│            v                              v               │ │
│      Critical Path                  Standard Path         │ │
│      (bypass all)              Windowing + Redis Fusion   │ │
│            │                              │               │ │
│            │                              v               │ │
│            │                     Deadband Evaluator        │ │
│            │                       (Redis baseline)        │ │
│            │                              │                 │
│            │                              v                 │ │
│            │                      Heartbeat / LOCF          │ │
│            │                              │                 │ │
│            └──────────────┬───────────────┘                 │
│                            v                                  │
│                   Flag Emitter ──────────▶  (to Postgres /    │
│                            │                  flag topic)     │
│                            v                                  │
│                   Output Formatter                            │
└─────────────────────────┬───────────────────────────────────┘
                           │ produces JSON to
                           v
                  ┌─────────────────┐
                  │  Kafka Topic     │  (clean output)
                  └────────┬─────────┘
                           │
              ┌────────────┴────────────┐
              v                          v
       Hot Path Storage           Cold Path Storage
       (Redis / consumer)         (MinIO / S3-compatible)


┌─────────────────────────────────────────────────────────┐
│                  DASHBOARD (Python/FastAPI)                │
│   Consumes flag topic ──▶ Postgres ──▶ REST API ──▶ Frontend│
│   Acknowledgment endpoint writes back to Postgres audit log │
└─────────────────────────────────────────────────────────┘
```

---

## 4. Repository Structure (suggested)

```
adaptive-etl-engine/
├── engine/                  # Go core engine
│   ├── cmd/
│   │   └── engine/main.go
│   ├── internal/
│   │   ├── coldstart/       # checklist logic, ETA calc, verification
│   │   ├── schema/          # inference, drift detection, versioning
│   │   ├── tiering/         # critical/standard/flagged classification
│   │   ├── deadband/        # threshold evaluation
│   │   ├── heartbeat/
│   │   ├── locf/
│   │   ├── fusion/          # windowing + Redis stateful join
│   │   ├── flags/           # flag emission logic
│   │   ├── config/          # YAML parsing + validation
│   │   ├── checkpoint/      # Phase 7 resilience/restart logic
│   │   └── kafka/           # producer/consumer wrappers
│   └── go.mod
│
├── generators/               # Python sensor simulators
│   ├── generators/
│   │   ├── steady_sensor.py
│   │   ├── multi_rate.py
│   │   ├── anomaly_injector.py
│   │   ├── dropout_sim.py
│   │   ├── conditional_sensor.py
│   │   └── schema_drift_sim.py
│   └── requirements.txt
│
├── dashboard/
│   ├── backend/              # FastAPI app
│   │   ├── api/
│   │   ├── models/
│   │   └── main.py
│   └── frontend/              # React or plain HTML/JS
│
├── config/
│   └── engine_config.yaml
│
├── infra/
│   ├── docker-compose.yaml   # Kafka, Redis, Postgres, MinIO, engine, dashboard
│   └── k8s/ (optional, later)
│
└── docs/
    ├── design_doc.md
    ├── dev_phases.md
    └── architecture.md (this file)
```

---

## 5. Local Development Environment

For capstone development and demoing, everything runs via Docker Compose rather than real cloud infrastructure:

- Kafka (with Zookeeper or KRaft mode)
- Redis
- PostgreSQL
- MinIO (S3-compatible, for cold path testing)
- Engine container (Go binary)
- Dashboard backend container (FastAPI)
- Generators run as standalone Python processes, not containerized, for easy iteration during testing

This keeps the entire stack runnable on a single laptop with `docker-compose up`, with no AWS costs incurred until you're ready to demonstrate real cloud deployment, if at all.

---

## 6. Deployment Story (for defense)

The engine compiles to a single Go binary, making it trivially containerizable and deployable to any environment — a cloud VM, a Kubernetes pod, or even an edge device, despite the design explicitly not depending on edge placement. This reinforces the "middleware, not edge-specific" framing from the design doc: the same binary runs wherever the developer chooses to put it.

For a live demo, Docker Compose locally is sufficient. If time allows, a single cloud deployment (e.g. one EC2 instance or a small Kubernetes cluster) running the same Compose-equivalent stack would let you demonstrate real network conditions, but this is optional polish, not core to the capstone's grading criteria.

---

## 7. Why This Split Is Defensible

If asked in your defense why two languages instead of one: the engine's hot path — handling potentially thousands of messages per second, doing in-memory lookups, evaluating thresholds — benefits directly from Go's compiled performance and goroutine-based concurrency model. Everything else in the system (generating test data, serving a dashboard, handling human acknowledgment workflows) is not performance-sensitive and benefits more from Python's development speed. This isn't language proliferation for its own sake — every component's language was chosen based on where it sits relative to the engine's performance-critical boundary.
