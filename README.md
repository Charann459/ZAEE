# Zero Assumption ETL Engine (ZAEE)

An adaptive, real-time ETL engine deployed as middleware. It utilizes dynamic schema inference, deadband compression (SDT), and priority-tiered filtering to ingest heterogeneous data streams, eliminate redundant data, and reduce cloud transmission overhead — without making any assumptions about the upstream sources or downstream destinations.

## Architecture

The project splits into distinct layers to separate the high-performance hot path from human-facing tooling:
- **Core Engine (Go)**: High-throughput, low-latency ingestion, filtering, and fusion path.
- **Message Broker (Kafka)**: Ordered, partitioned ingestion (`zaee_ingest`) and output (`zaee_output`).
- **State Cache (Redis)**: In-memory store for baseline values, windowing state, and LOCF join lookups.
- **Audit/Flag Storage (PostgreSQL)**: Relational storage for anomaly flags and human resolution trails.
- **Dashboard Backend (Python FastAPI + aiokafka)**: Real-time flag routing using Server-Sent Events (SSE).
- **Dashboard Frontend (Vanilla HTML/JS/CSS)**: Premium web UI for monitoring live anomalies and the audit trail.
- **Generators (Python)**: Simulations for multi-rate sensors, dropouts, schema drift, and type oscillation.

---

## Prerequisites

- **Docker** & **Docker Compose**
- **Go 1.21+** (if running the engine outside Docker)
- **Python 3.10+** (for the Dashboard backend and Data Generators)

---

## How to Run the Entire Project

### 1. Start the Infrastructure & Engine
The system relies on a Kafka broker, Redis, and PostgreSQL. We have also fully containerized the core engine. Spin them all up locally via Docker Compose:
```bash
cd infra
docker-compose up -d --build
```
*(This starts Zookeeper, Kafka, Redis, Postgres, and the **ZAEE Engine** in the background).*

### 2. View Engine Logs (Optional)
Since the engine is running inside Docker, you don't need Go installed on your machine. You can view its real-time logs to see it ingesting data:
```bash
docker-compose logs -f engine
```

### 3. Start the Dashboard (Backend & Frontend)
The dashboard provides human observability into flagged events (schema drifts, dropped sensors, etc.).

**Backend:**
```bash
cd dashboard/backend
python -m venv venv

# Activate the virtual environment
# On Windows (PowerShell):
.\venv\Scripts\Activate.ps1
# On macOS/Linux:
source venv/bin/activate

pip install -r requirements.txt

# Run the FastAPI server on port 8000
uvicorn main:app --reload
```

**Frontend:**
The frontend uses vanilla HTML/JS/CSS. You can open it directly in your browser:
```bash
# Double click or open in browser:
dashboard/frontend/index.html
```

### 4. Run the Data Generators
With the engine waiting and the dashboard listening, start injecting simulated sensor data.
```bash
cd generators
pip install -r requirements.txt

# Run a specific scenario, such as type oscillation / schema drift:
python type_oscillation_sim.py

# Or run the standard steady-state generators:
python run_all.py
```
*Observe the dashboard to see flags actively emitted (e.g., when the schema drifts or a sensor drops out).*

---

## Running the Tests & Generating Evidence Reports

The project has comprehensive unit, integration, and system tests.
To run the full suite and generate the Capstone Evidence Report:
```bash
cd engine
docker build --target builder -t zaee-builder .
docker run --rm zaee-builder go test -v -race ./...
```
*(To format the test results into the HTML evidence report, use the provided `report_generator.py` script against the JSON test outputs).*
