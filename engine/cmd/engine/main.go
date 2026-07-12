package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"zaee-engine/pkg/config"
	"zaee-engine/pkg/fusion"
	"zaee-engine/pkg/processor"
	"zaee-engine/pkg/resilience"
	"zaee-engine/pkg/schema"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func main() {
	validateFlag := flag.Bool("validate", false, "Validate the config file and exit")
	configPath := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	if *validateFlag {
		fmt.Printf("[ZAEE Engine] Validating config file at %s...\n", *configPath)
		cfg, err := config.Load(*configPath)
		if err != nil {
			fmt.Printf("Validation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Validation successful! Schema looks good.")
		fmt.Printf("Parsed %d sensors from config.\n", len(cfg.Sensors))
		os.Exit(0)
	}

	fmt.Printf("[ZAEE Engine] Starting Phase 2: Schema Inference\n")

	// Load Config (non-fatal if missing for now, allows pure dynamic inference)
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("[ZAEE Engine] Warning: Could not load %s (%v). Running in pure dynamic mode.\n", *configPath, err)
		cfg = &config.Config{}
	}

	// Initialize Schema Registry
	registry := schema.NewRegistry()

	// Pre-load sensors from config into registry
	for _, s := range cfg.Sensors {
		var hb time.Duration = 60 * time.Second
		if s.HeartbeatInterval != "" {
			d, err := time.ParseDuration(s.HeartbeatInterval)
			if err == nil {
				hb = d
			}
		} else if cfg.Engine.Heartbeat.DefaultInterval != "" {
			d, err := time.ParseDuration(cfg.Engine.Heartbeat.DefaultInterval)
			if err == nil {
				hb = d
			}
		}

		sc := &schema.SensorSchema{
			ID:                s.ID,
			Tier:                     s.Tier,
			ExpectsExternalHeartbeat: s.ExpectsExternalHeartbeat,
			HeartbeatInterval:        hb,
			LastHeartbeatTime: time.Now(), // Initialize to now
			LastSeenTime:      time.Now(), // Initialize to now
			Fields:            make(map[string]*schema.Field),
		}
		for _, f := range s.Fields {
			sc.Fields[f.Name] = &schema.Field{
				Name:  f.Name,
				Type:  f.Type,
				State: schema.StateConditional, // pre-declared fields
			}
		}
		registry.RegisterSensor(sc)
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://zaee:zaee_password@localhost:5432/zaee?sslmode=disable"
	}
	resil := resilience.InitResilience(context.Background(), dbURL, redisURL, 5)

	proc := processor.NewProcessor(registry, resil)

	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}
	ingestTopic := os.Getenv("INGEST_TOPIC")
	if ingestTopic == "" {
		ingestTopic = "zaee_ingest"
	}
	outputTopic := os.Getenv("OUTPUT_TOPIC")
	if outputTopic == "" {
		outputTopic = "zaee_output"
	}

	fmt.Printf("[ZAEE Engine] Brokers: %s | Ingest: %s | Output: %s\n", brokers, ingestTopic, outputTopic)

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"group.id":          "zaee_core_engine_group",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		fmt.Printf("Failed to create consumer: %s\n", err)
		os.Exit(1)
	}

	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": brokers})
	if err != nil {
		fmt.Printf("Failed to create producer: %s\n", err)
		os.Exit(1)
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	c.SubscribeTopics([]string{ingestTopic}, nil)

	fmt.Println("[ZAEE Engine] Listening for data...")

	// 2. Check for unclean shutdown BEFORE writing startup event
	lastEvent, lastTime, err := resil.CheckLastLifecycleSafe()
	if err == nil && lastEvent == "heartbeat" && time.Since(lastTime) > 10*time.Second {
		crashDuration := time.Since(lastTime).String()
		fmt.Printf("[ZAEE Engine] Unclean shutdown detected. Duration: %s\n", crashDuration)
		
		flagPayload := fmt.Sprintf(`{"flag":"unclean_shutdown","crash_duration":"%s","timestamp":"%s"}`, crashDuration, time.Now().UTC().Format(time.RFC3339Nano))
		deliveryChan := make(chan kafka.Event)
		p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &outputTopic, Partition: kafka.PartitionAny},
			Value:          []byte(flagPayload),
		}, deliveryChan)
		<-deliveryChan
	}
	
	// 3. Write startup event
	resil.WriteLifecycleEventSafe("startup")

	run := true

	// 4. Background DB Heartbeat Checker
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for run {
			<-ticker.C
			resil.WriteLifecycleEventSafe("heartbeat")
		}
	}()

	// Background Sensor Heartbeat Checker
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for run {
			<-ticker.C
			heartbeats := proc.CheckHeartbeats()
			for _, hbPayload := range heartbeats {
				deliveryChan := make(chan kafka.Event)
				err := p.Produce(&kafka.Message{
					TopicPartition: kafka.TopicPartition{Topic: &outputTopic, Partition: kafka.PartitionAny},
					Value:          hbPayload,
				}, deliveryChan)
				if err != nil {
					fmt.Printf("Produce error for heartbeat/dropout: %v\n", err)
					close(deliveryChan)
				} else {
					<-deliveryChan
					close(deliveryChan)
					fmt.Printf("[ZAEE Engine] Emitted dropout flag for silent sensor\n")
				}
			}
		}
	}()

	fusionManager, err := fusion.NewManager(resil.GetRedis(), registry, cfg)
	if err != nil {
		fmt.Printf("Failed to initialize fusion manager: %v\n", err)
	} else {
		// Start fusion background processor
		go func() {
			for fused := range fusionManager.OutChan() {
				outPayload, err := proc.Evaluate(fused.SensorID, fused.Timestamp, fused.Fields, schema.InferResult{Flags: fused.Flags})
				if err != nil {
					fmt.Printf("[Core Pipe] Failed to evaluate fused message: %v\n", err)
					continue
				}
				if outPayload != nil {
					deliveryChan := make(chan kafka.Event)
					err = p.Produce(&kafka.Message{
						TopicPartition: kafka.TopicPartition{Topic: &outputTopic, Partition: kafka.PartitionAny},
						Value:          outPayload,
					}, deliveryChan)
					if err == nil {
						<-deliveryChan
						close(deliveryChan)
					}
				}
			}
		}()
	}

	for run {
		select {
		case sig := <-sigchan:
			fmt.Printf("Caught signal %v: terminating\n", sig)
			resil.WriteLifecycleEventSafe("shutdown_clean")
			run = false
		default:
			ev := c.Poll(100)
			if ev == nil {
				continue
			}

			switch e := ev.(type) {
			case *kafka.Message:
				// 1. Process payload dynamically
				var data map[string]interface{}
				if err := json.Unmarshal(e.Value, &data); err != nil {
					fmt.Printf("[Core Pipe] Failed to parse json: %v\n", err)
					continue
				}

				sensorID, ok := data["sensor_id"].(string)
				if !ok || sensorID == "" {
					fmt.Printf("[Core Pipe] Missing sensor_id\n")
					continue
				}

				fieldsMap, ok := data["fields"].(map[string]interface{})
				if !ok {
					fieldsMap = make(map[string]interface{})
				}

				timestampStr, ok := data["timestamp"].(string)
				if !ok {
					timestampStr = time.Now().UTC().Format(time.RFC3339)
				}
				payloadTime, err := time.Parse(time.RFC3339Nano, timestampStr)
				if err != nil {
					payloadTime, _ = time.Parse(time.RFC3339, timestampStr)
					if payloadTime.IsZero() {
						payloadTime = time.Now().UTC()
					}
				}

				inferResult, _ := registry.Infer(sensorID, fieldsMap, payloadTime, resil)

				// Lookup tier
				sensorSchema, exists := registry.GetSensor(sensorID)
				tier := "standard"
				if exists {
					tier = sensorSchema.Tier
				}

				if tier == "critical" || fusionManager == nil {
					// Bypass Fusion
					outPayload, err := proc.Evaluate(sensorID, payloadTime, fieldsMap, inferResult)
					if err != nil {
						fmt.Printf("[Core Pipe] Failed to evaluate critical message: %v\n", err)
						continue
					}
					if outPayload != nil {
						deliveryChan := make(chan kafka.Event)
						err = p.Produce(&kafka.Message{
							TopicPartition: kafka.TopicPartition{Topic: &outputTopic, Partition: kafka.PartitionAny},
							Value:          outPayload,
							Key:            e.Key,
						}, deliveryChan)
						if err == nil {
							<-deliveryChan
							close(deliveryChan)
						}
					}
				} else {
					// Route to Fusion Buffer
					// Note: Schema violation flags will be lost for standard sensors due to fusion,
					// but LOCF flags will be applied. A full production implementation would merge them.
					fusionManager.AddMessage(sensorID, payloadTime, fieldsMap, inferResult)
				}

			case kafka.Error:
				fmt.Fprintf(os.Stderr, "%% Error: %v\n", e)
			}
		}
	}

	fmt.Println("[ZAEE Engine] Closing components...")
	c.Close()
	p.Close()
}
