#!/usr/bin/env python3

"""Функциональный тест динамического интервала опроса.

Сценарий:
1. Контейнер запускается с memory limit.
2. oom-signal запускается с порогом 80%.
3. Приложение аллоцирует память до ~60% лимита (стабильный уровень),
   затем делает резкий burst, пересекая порог 80%.
4. Замеряется задержка между burst-аллокацией и получением SIGUSR1.
5. Проверяется, что в stderr есть хотя бы одна строка смены интервала
   вида "[oom-signal] poll interval: ... → ...".
"""

import argparse
import os
import signal
import subprocess
import sys
import time

BLOCKS: list = []
_burst_time: float = 0
MAX_LATENCY_MS: int = 200


def handle_sigusr1(signum, frame):
    delay_ms = (time.monotonic() - _burst_time) * 1000
    print(f"Received SIGUSR1 from oom-signal (delay: {delay_ms:.1f}ms)", flush=True)

    if delay_ms > MAX_LATENCY_MS:
        print(f"FAIL: delay {delay_ms:.1f}ms > {MAX_LATENCY_MS}ms", flush=True)
        os._exit(1)

    print("PASS", flush=True)
    os._exit(0)


def allocate_mib(size_mib: int) -> None:
    block = bytearray(size_mib * 1024**2)
    for i in range(0, len(block), 4096):
        block[i] = 1
    BLOCKS.append(block)


def read_cgroup_limit() -> int:
    """Читает memory.max из cgroup v2."""
    try:
        with open("/sys/fs/cgroup/memory.max") as f:
            val = f.read().strip()
            if val == "max":
                return 0
            return int(val)
    except FileNotFoundError:
        return 0


def main() -> None:
    global _burst_time, MAX_LATENCY_MS

    parser = argparse.ArgumentParser()
    parser.add_argument("--max-latency-ms", type=int, default=200,
                        help="Максимально допустимая задержка сигнала (ms)")
    args = parser.parse_args()

    MAX_LATENCY_MS = args.max_latency_ms

    signal.signal(signal.SIGUSR1, handle_sigusr1)

    monitor = subprocess.Popen(
        ["oom-signal", f"--pid={os.getpid()}"],
        stderr=subprocess.PIPE,
        text=True,
    )

    limit_mib = read_cgroup_limit() // (1024 * 1024)
    if limit_mib == 0:
        print("FAIL: cannot read cgroup memory limit", flush=True)
        monitor.terminate()
        sys.exit(1)

    threshold_mib = int(limit_mib * 0.8)
    warmup_mib = int(limit_mib * 0.6)
    burst_mib = threshold_mib - warmup_mib + int(limit_mib * 0.1)

    print(f"Burst test: limit={limit_mib}MiB threshold={threshold_mib}MiB "
          f"warmup={warmup_mib}MiB burst={burst_mib}MiB "
          f"max_latency={MAX_LATENCY_MS}ms pid={os.getpid()}", flush=True)

    try:
        # Фаза 1: заполняем до ~60% лимита (стабильный уровень)
        allocate_mib(warmup_mib)
        print(f"Warmup: allocated {warmup_mib}MiB, waiting for monitor to settle", flush=True)
        time.sleep(1)

        # Фаза 2: burst — резкая аллокация через порог
        print(f"Burst: allocating {burst_mib}MiB...", flush=True)
        _burst_time = time.monotonic()
        allocate_mib(burst_mib)

        # Ждём сигнал (с таймаутом)
        time.sleep(5)

        print("FAIL: no signal received within timeout", flush=True)
        sys.exit(1)

    finally:
        monitor.terminate()
        stderr_output = monitor.stderr.read()

        has_interval_change = False
        for line in stderr_output.splitlines():
            print(f"[stderr] {line}", flush=True)
            if "poll interval:" in line:
                has_interval_change = True

        if not has_interval_change:
            print("FAIL: no poll interval change found in stderr", flush=True)
            sys.exit(1)


if __name__ == "__main__":
    main()
