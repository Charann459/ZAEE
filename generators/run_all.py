import subprocess
import sys
import time
import argparse

def main():
    parser = argparse.ArgumentParser(description="Run all ZAEE stream generators")
    parser.add_argument("--speedup", type=float, default=1.0, help="Speed multiplier for all generators (default: 1.0)")
    parser.add_argument("--kafka", action="store_true", help="Pipe output to Kafka instead of printing to screen")
    parser.add_argument("--broker", default="localhost:9092", help="Kafka broker")
    args = parser.parse_args()

    print(f"Starting all generators with a {args.speedup}x speedup...\n")

    generators = [
        "generator_machine_a.py",
        "generator_machine_b.py",
        "generator_machine_c.py",
        "generator_secom.py"
    ]

    processes = []
    kafka_proc = None
    
    try:
        # If kafka flag is set, start the stdout_to_kafka.py process
        if args.kafka:
            print(f"Kafka mode enabled. Routing output to zaee_ingest topic at {args.broker}...\n")
            kafka_proc = subprocess.Popen([sys.executable, "stdout_to_kafka.py", "--broker", args.broker], stdin=subprocess.PIPE)

        for script in generators:
            cmd = [sys.executable, script, "--speedup", str(args.speedup)]
            
            if args.kafka:
                # Pipe stdout of generators to the stdin of the kafka producer
                p = subprocess.Popen(cmd, stdout=kafka_proc.stdin)
            else:
                p = subprocess.Popen(cmd)
                
            processes.append(p)
            
        print("\nAll generators are running. Press Ctrl+C to stop all.\n")
        
        while True:
            time.sleep(1)
            
    except KeyboardInterrupt:
        print("\nStopping all generators...")
        for p in processes:
            p.terminate()
            p.wait()
            
        if kafka_proc:
            kafka_proc.stdin.close()
            kafka_proc.wait()
            
        print("All generators stopped cleanly.")

if __name__ == "__main__":
    main()
