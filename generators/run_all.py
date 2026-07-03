import subprocess
import sys
import time
import argparse

def main():
    parser = argparse.ArgumentParser(description="Run all ZAEE stream generators")
    parser.add_argument("--speedup", type=float, default=1.0, help="Speed multiplier for all generators (default: 1.0)")
    args = parser.parse_args()

    print(f"Starting all generators with a {args.speedup}x speedup...\n")

    # List of generator scripts to run
    generators = [
        "generator_machine_a.py",
        "generator_machine_b.py",
        "generator_machine_c.py",
        "generator_secom.py"
    ]

    processes = []
    try:
        # Spawn each generator as a separate subprocess
        for script in generators:
            cmd = [sys.executable, script, "--speedup", str(args.speedup)]
            p = subprocess.Popen(cmd)
            processes.append(p)
            
        print("\nAll generators are running. Press Ctrl+C to stop all.\n")
        
        # Wait indefinitely for keyboard interrupt
        while True:
            time.sleep(1)
            
    except KeyboardInterrupt:
        print("\nStopping all generators...")
        for p in processes:
            p.terminate()
            p.wait()
        print("All generators stopped cleanly.")

if __name__ == "__main__":
    main()
