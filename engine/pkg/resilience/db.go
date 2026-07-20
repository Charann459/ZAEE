package resilience

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Resilience struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

func InitResilience(ctx context.Context, dbURL, redisURL string, maxRetries int) *Resilience {
	var db *pgxpool.Pool
	var rdb *redis.Client

	// PostgreSQL Backoff
	backoff := 1 * time.Second
	for attempts := 0; attempts < maxRetries; attempts++ {
		fmt.Printf("[ZAEE Resilience] Connecting to PostgreSQL (Attempt %d/%d)...\n", attempts+1, maxRetries)
		pool, err := pgxpool.New(ctx, dbURL)
		if err == nil {
			err = pool.Ping(ctx)
			if err == nil {
				db = pool
				fmt.Println("[ZAEE Resilience] PostgreSQL connected successfully.")
				break
			}
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	if db == nil {
		fmt.Println("[ZAEE Resilience] WARNING: PostgreSQL unavailable, proceeding without persistence (Graceful Degradation)")
	}

	// Redis Backoff
	backoff = 1 * time.Second
	for attempts := 0; attempts < maxRetries; attempts++ {
		fmt.Printf("[ZAEE Resilience] Connecting to Redis (Attempt %d/%d)...\n", attempts+1, maxRetries)
		opts, err := redis.ParseURL(redisURL)
		if err == nil {
			client := redis.NewClient(opts)
			if err := client.Ping(ctx).Err(); err == nil {
				rdb = client
				fmt.Println("[ZAEE Resilience] Redis connected successfully.")
				break
			}
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	if rdb == nil {
		fmt.Println("[ZAEE Resilience] WARNING: Redis unavailable, proceeding without distributed caching (Graceful Degradation)")
	}

	return &Resilience{
		db:  db,
		rdb: rdb,
	}
}

func (r *Resilience) GetRedis() *redis.Client {
	return r.rdb
}

func (r *Resilience) WriteCheckpointSafe(sensorID, milestone string, metadata interface{}) {
	if r.db == nil {
		fmt.Println("[ZAEE Resilience] PostgreSQL unavailable, skipping checkpoint.")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	_, err := r.db.Exec(ctx,
		`INSERT INTO cold_start_checkpoints (sensor_id, milestone, completed_at, metadata) VALUES ($1, $2, NOW(), $3)`,
		sensorID, milestone, metadata,
	)
	if err != nil {
		fmt.Printf("[ZAEE Resilience] Error writing checkpoint for %s (%s): %v\n", sensorID, milestone, err)
	}
}

func (r *Resilience) WriteLifecycleEventSafe(event string) {
	if r.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := r.db.Exec(ctx,
		`INSERT INTO engine_lifecycle (event, occurred_at) VALUES ($1, NOW())`,
		event,
	)
	if err != nil {
		fmt.Printf("[ZAEE Resilience] Error writing lifecycle event '%s': %v\n", event, err)
	}
}

func (r *Resilience) CheckLastLifecycleSafe() (string, time.Time, error) {
	if r.db == nil {
		return "", time.Time{}, fmt.Errorf("postgres unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var event string
	var occurredAt time.Time
	err := r.db.QueryRow(ctx,
		`SELECT event, occurred_at FROM engine_lifecycle ORDER BY occurred_at DESC LIMIT 1`,
	).Scan(&event, &occurredAt)

	if err != nil {
		return "", time.Time{}, err
	}
	return event, occurredAt, nil
}

func (r *Resilience) LoadCheckpointsSafe(sensorID string) ([]byte, error) {
	if r.db == nil {
		return nil, fmt.Errorf("postgres unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var metadata []byte
	err := r.db.QueryRow(ctx,
		`SELECT metadata FROM cold_start_checkpoints WHERE sensor_id = $1 ORDER BY completed_at DESC LIMIT 1`,
		sensorID,
	).Scan(&metadata)

	if err != nil {
		return nil, err
	}
	
	
	return metadata, nil
}

func (r *Resilience) DeleteSDTStateSafe(sensorID, fieldName string) {
	if r.rdb == nil {
		fmt.Printf("[ZAEE Resilience] Redis unavailable, skipping SDT state deletion for %s:%s\n", sensorID, fieldName)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := fmt.Sprintf("sdt_state:%s:%s", sensorID, fieldName)
	err := r.rdb.Del(ctx, key).Err()
	if err != nil {
		fmt.Printf("[ZAEE Resilience] Error deleting SDT state for %s: %v\n", key, err)
	} else {
		fmt.Printf("[ZAEE Resilience] Deleted stale SDT state from Redis: %s\n", key)
	}
}
