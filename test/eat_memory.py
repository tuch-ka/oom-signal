#!/usr/bin/env python3

import argparse
import os
import signal
import subprocess
import sys
from concurrent.futures import ProcessPoolExecutor, ThreadPoolExecutor
from time import sleep

import psutil


def print_memory(worker_id: int) -> None:
    mem = psutil.Process(os.getpid()).memory_info()
    print(f"  worker {worker_id} memory: {mem.rss / (1024 ** 2):.0f} MB RSS", flush=True)


def handle_sigusr1(signum, frame):
    print("Received SIGUSR1 from oom-signal", flush=True)
    os._exit(0)


def allocate_block(blocks: list, size_mb: int) -> None:
    block = bytearray(size_mb * 1024**2)
    for i in range(0, len(block), 4096):
        block[i] = 1
    blocks.append(block)


def increase_threads(worker_id: int, step_mb: int) -> None:
    blocks: list = []
    for i in range(100):
        print(f"worker {worker_id}: iteration {i}", flush=True)
        allocate_block(blocks, step_mb)
        print_memory(worker_id)
        sleep(1)


def increase_processes(worker_id: int, step_mb: int) -> None:
    signal.signal(signal.SIGUSR1, handle_sigusr1)
    blocks: list = []
    for i in range(100):
        print(f"worker {worker_id}: iteration {i}", flush=True)
        allocate_block(blocks, step_mb)
        print_memory(worker_id)
        sleep(1)


def run_threads(workers: int, step_mb: int) -> None:
    with ThreadPoolExecutor(max_workers=workers) as executor:
        futures = [executor.submit(increase_threads, i, step_mb) for i in range(workers)]
        for f in futures:
            f.result()


def run_processes(workers: int, step_mb: int) -> None:
    with ProcessPoolExecutor(max_workers=workers) as executor:
        futures = [executor.submit(increase_processes, i, step_mb) for i in range(workers)]
        for f in futures:
            f.result()


def main() -> None:
    signal.signal(signal.SIGUSR1, handle_sigusr1)

    parser = argparse.ArgumentParser()
    parser.add_argument("mode", nargs="?", default="threads", choices=["threads", "processes"])
    parser.add_argument("--step", type=int, default=64, help="Memory allocation step in MB (default: 64)")
    parser.add_argument("--workers", type=int, default=4, help="Number of workers (default: 4)")
    args = parser.parse_args()

    monitor = subprocess.Popen(
        ["oom-signal", "--pid=" + str(os.getpid())],
    )

    print(f"Starting memory test: mode={args.mode} workers={args.workers} step={args.step}MB pid={os.getpid()}", flush=True)

    try:
        if args.mode == "processes":
            run_processes(args.workers, args.step)
        else:
            run_threads(args.workers, args.step)
    except MemoryError:
        print("MemoryError: OOM", flush=True)
        sys.exit(1)
    finally:
        monitor.terminate()
        monitor.wait()


if __name__ == "__main__":
    main()
