import sys
import argparse
from confluent_kafka import Producer

def main():
    parser = argparse.ArgumentParser(description="Pipe stdout to Kafka")
    parser.add_argument("--broker", default="localhost:9092", help="Kafka broker")
    parser.add_argument("--topic", default="zaee_ingest", help="Kafka topic")
    args = parser.parse_args()

    # Configure Producer
    conf = {
        'bootstrap.servers': args.broker,
        'client.id': 'python-generator'
    }
    producer = Producer(conf)

    print(f"[Producer] Connecting to {args.broker}, topic: {args.topic}", file=sys.stderr)

    def delivery_callback(err, msg):
        if err:
            print(f"[Producer] ERROR: Message failed delivery: {err}", file=sys.stderr)

    try:
        for line in sys.stdin:
            line = line.strip()
            if not line:
                continue
            
            # Since the generators might print some log lines, we only send JSON (starting with '{')
            if line.startswith('{'):
                producer.produce(args.topic, line.encode('utf-8'), callback=delivery_callback)
                producer.poll(0)
            else:
                # Pass through logs to stderr
                print(line, file=sys.stderr)
                
    except KeyboardInterrupt:
        pass
    finally:
        print("[Producer] Flushing messages...", file=sys.stderr)
        producer.flush()

if __name__ == "__main__":
    main()
