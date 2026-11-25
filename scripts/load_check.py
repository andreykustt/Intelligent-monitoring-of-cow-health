"""Measure one 200-cow simulation on a disposable Compose stack."""
import argparse
import json
import math
import platform
import statistics
import subprocess
import sys
import time
from collections import Counter
from datetime import datetime, timezone
from http.client import HTTPException
from http.cookiejar import MozillaCookieJar
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import HTTPCookieProcessor, ProxyHandler, build_opener
from uuid import UUID

from http_client import ORIGIN, Client, compose_args, wait_for_simulation

COWS = 200
INTERVAL_MS = 5000
REQUEST_PERIOD = 0.25
SEED = 42


def stored_snapshot(simulation_id):
    # Normalize the ID before embedding it into the SQL sent on stdin.
    simulation_id = UUID(simulation_id)
    query = (
        'SELECT count(*), extract(epoch FROM statement_timestamp()) '
        f"FROM sensor_measurements WHERE simulation_id = '{simulation_id}';"
    )
    result = subprocess.run(
        compose_args(
            'exec', '-T', 'postgres', 'sh', '-c',
            'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -v ON_ERROR_STOP=1',
        ),
        input=query, capture_output=True, text=True, check=True,
    )
    count, timestamp = result.stdout.strip().split('|')
    return int(count), float(timestamp)


def sample_http(client, duration):
    cookies = MozillaCookieJar(client.cookie)
    cookies.load()
    opener = build_opener(ProxyHandler({}), HTTPCookieProcessor(cookies))
    latencies = []
    errors = Counter()
    started = time.monotonic()
    deadline = started + duration
    while (sent := time.monotonic()) < deadline:
        try:
            with opener.open(ORIGIN + '/api/v1/cows', timeout=5) as response:
                response.read()
                status = response.status
        except HTTPError as error:
            status = error.code
            error.close()
        except (URLError, OSError, HTTPException) as error:
            status = type(error).__name__
        if status == 200:
            latencies.append((time.monotonic() - sent) * 1000)
        else:
            errors[str(status)] += 1
        time.sleep(max(0, min(sent + REQUEST_PERIOD, deadline) - time.monotonic()))
    return sorted(latencies), dict(errors), time.monotonic() - started


def measure(duration, warmup):
    docker = subprocess.run(
        ['docker', 'info', '--format', '{{.OSType}}/{{.Architecture}} {{.NCPU}} {{.MemTotal}}'],
        capture_output=True, text=True, check=True,
    )
    docker_os, docker_cpus, docker_memory = docker.stdout.strip().split()
    client = Client()
    client.login('admin')
    metrics = [item['code'] for item in client.call('/metrics')]
    config = {
        'cow_count': COWS,
        'metrics_enabled': metrics,
        'interval_ms_by_metric': dict.fromkeys(metrics, INTERVAL_MS),
        'duration_seconds': duration + warmup + 180,
        'mode': 'synthetic',
        'scenario': 'normal',
        'seed': SEED,
    }
    simulation_id = client.call('/simulations', 'POST', {'farm_id': 'demo', 'config': config}, 202)['id']
    try:
        wait_for_simulation(client, simulation_id, 'running', timeout=120)
        if warmup:
            sample_http(client, warmup)
        count_before, db_started = stored_snapshot(simulation_id)
        latencies, errors, seconds = sample_http(client, duration)
        count_after, db_ended = stored_snapshot(simulation_id)
    finally:
        measuring_failed = sys.exc_info()[0] is not None
        try:
            client.call(f'/simulations/{simulation_id}/stop', 'POST', {}, 202)
            wait_for_simulation(client, simulation_id, 'stopped', timeout=30)
        except (RuntimeError, AssertionError, subprocess.SubprocessError) as error:
            if not measuring_failed:
                raise
            print(f'Could not confirm simulation stop: {error}', file=sys.stderr)
    events = count_after - count_before
    db_seconds = db_ended - db_started
    attempts = len(latencies) + sum(errors.values())
    return {
        'date': datetime.now(timezone.utc).isoformat(timespec='seconds'),
        'host': platform.platform(),
        'docker': {'platform': docker_os, 'cpus': int(docker_cpus), 'memory_bytes': int(docker_memory)},
        'simulation_id': simulation_id,
        'cows': COWS,
        'metrics': metrics,
        'interval_ms': INTERVAL_MS,
        'seed': SEED,
        'warmup_seconds': warmup,
        'target_duration_seconds': duration,
        'http_measured_seconds': round(seconds, 3),
        'db_measured_seconds': round(db_seconds, 3),
        'stored_events': events,
        'db_writes_per_second': round(events / db_seconds, 2),
        'http_request_period_seconds': REQUEST_PERIOD,
        'http_requests': attempts,
        'http_requests_per_second': round(attempts / seconds, 2),
        'http_errors': errors,
        'http_p95_ms': round(latencies[math.ceil(len(latencies) * 0.95) - 1], 2) if latencies else None,
        'http_median_ms': round(statistics.median(latencies), 2) if latencies else None,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--duration', type=int, default=600, help='measured seconds')
    parser.add_argument('--warmup', type=int, default=30, help='warmup seconds')
    parser.add_argument('--output', type=Path, help='optional JSON report path')
    args = parser.parse_args()
    if args.duration < 1 or args.warmup < 0 or args.duration + args.warmup + 180 > 86400:
        parser.error('duration must be positive, warmup nonnegative, and the run at most 86400 seconds')
    report = measure(args.duration, args.warmup)
    data = json.dumps(report, indent=2) + '\n'
    print(data, end='')
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(data, encoding='utf-8')
    if report['stored_events'] <= 0 or report['http_p95_ms'] is None:
        raise SystemExit('No stored telemetry or successful API samples during the measurement')


if __name__ == '__main__':
    main()
