"""
kafka_helper.py
---------------
Shared helper for generators to produce directly to Kafka,
eliminating the need for piping through stdout_to_kafka.py.

Usage in a generator:
    from kafka_helper import KafkaHelper
    kh = KafkaHelper(kafka_mode=args.kafka)
    kh.emit(payload)
    kh.flush()
"""

import sys
import json

class KafkaHelper:
    def __init__(self, kafka_mode: bool = False,
                 broker: str = 'localhost:9092',
                 topic: str = 'zaee_ingest'):
        self.kafka_mode = kafka_mode
        self.broker = broker
        self.topic = topic
        self._producer = None

        if kafka_mode:
            try:
                from confluent_kafka import Producer
                self._producer = Producer({
                    'bootstrap.servers': broker,
                    'client.id': 'python-generator'
                })
                print(f"[Producer] Connected directly to {broker}, topic: {topic}", file=sys.stderr, flush=True)
            except Exception as e:
                print(f"[Producer] ERROR: Could not connect to Kafka: {e}", file=sys.stderr, flush=True)
                sys.exit(1)

    def emit(self, payload: dict):
        msg = json.dumps(payload)
        if self.kafka_mode and self._producer:
            self._producer.produce(self.topic, msg.encode('utf-8'))
            self._producer.poll(0)
        else:
            print(msg, flush=True)

    def flush(self):
        if self.kafka_mode and self._producer:
            print("[Producer] Flushing messages...", file=sys.stderr, flush=True)
            self._producer.flush()
