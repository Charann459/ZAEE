import os
import time
import boto3
from confluent_kafka import Consumer, KafkaError, KafkaException

KAFKA_BROKER = os.getenv("KAFKA_BROKER", "localhost:9092")

def main():
    print("[Cloud Bridge] Starting AWS CloudWatch publisher...")
    
    # Initialize AWS CloudWatch client
    try:
        cloudwatch = boto3.client('cloudwatch')
    except Exception as e:
        print(f"[Cloud Bridge] Failed to initialize boto3 client: {e}")
        print("[Cloud Bridge] Please ensure you have run 'aws configure' with appropriate credentials.")
        return

    consumer = Consumer({
        'bootstrap.servers': KAFKA_BROKER,
        'group.id': 'zaee_cloudwatch_bridge',
        'auto.offset.reset': 'latest',
        'enable.auto.commit': True
    })

    consumer.subscribe(['zaee_ingest', 'zaee_output'])

    ingest_count_this_window = 0
    output_count_this_window = 0
    
    PUBLISH_INTERVAL = 10.0 # 10 seconds window
    last_publish_time = time.time()

    print(f"[Cloud Bridge] Listening to Kafka broker {KAFKA_BROKER}.")
    print(f"[Cloud Bridge] Publishing windowed counts every {PUBLISH_INTERVAL}s...")

    try:
        while True:
            # Poll with a short timeout to remain responsive
            msg = consumer.poll(timeout=1.0)
            
            if msg is not None:
                if msg.error():
                    if msg.error().code() != KafkaError._PARTITION_EOF:
                        print(f"Kafka error: {msg.error()}")
                else:
                    topic = msg.topic()
                    if topic == 'zaee_ingest':
                        ingest_count_this_window += 1
                    elif topic == 'zaee_output':
                        output_count_this_window += 1

            current_time = time.time()
            if current_time - last_publish_time >= PUBLISH_INTERVAL:
                # Time to publish to CloudWatch
                print(f"[Publish] Window counts -> Ingest: {ingest_count_this_window} | Output: {output_count_this_window}")
                
                try:
                    cloudwatch.put_metric_data(
                        Namespace='ZAEE/ImpactAnalysis',
                        MetricData=[
                            {
                                'MetricName': 'RawIngestPayloads',
                                'Value': ingest_count_this_window,
                                'Unit': 'Count',
                                'Dimensions': [{'Name': 'Environment', 'Value': 'Demo'}]
                            },
                            {
                                'MetricName': 'EngineOutputPayloads',
                                'Value': output_count_this_window,
                                'Unit': 'Count',
                                'Dimensions': [{'Name': 'Environment', 'Value': 'Demo'}]
                            }
                        ]
                    )
                    print("[Cloud Bridge] Successfully published to AWS CloudWatch.")
                except Exception as e:
                    print(f"[Cloud Bridge] Failed to publish to CloudWatch: {e}")

                # Reset counters for the next window as requested
                ingest_count_this_window = 0
                output_count_this_window = 0
                last_publish_time = current_time

    except KeyboardInterrupt:
        print("[Cloud Bridge] Shutting down...")
    finally:
        consumer.close()

if __name__ == "__main__":
    main()
