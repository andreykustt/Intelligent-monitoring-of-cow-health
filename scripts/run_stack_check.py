"""Run integration checks against a disposable local Compose project."""
import argparse
import os
import socket
import subprocess
import sys
import tempfile
import uuid
from pathlib import Path

from init_env import ROOT, create_env


def free_port():
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]


def run(check, arguments):
    project = 'cowhealth-check-' + uuid.uuid4().hex[:12]
    port = free_port()
    with tempfile.TemporaryDirectory(prefix=project + '-') as directory:
        env_file = Path(directory) / '.env'
        values = create_env(env_file, APP_PORT=port, APP_ORIGIN=f'http://127.0.0.1:{port}')
        options = {
            'cwd': ROOT,
            'env': {
                **os.environ,
                **{key: str(value) for key, value in values.items()},
                'COWHEALTH_ENV_FILE': str(env_file),
                'COWHEALTH_COMPOSE_PROJECT': project,
            },
        }
        compose = [
            'docker', 'compose', '--file', str(ROOT / 'compose.yaml'),
            '--env-file', str(env_file), '--project-name', project,
        ]
        subprocess.run([*compose, 'config', '-q'], check=True, **options)
        try:
            subprocess.run(
                [*compose, 'up', '-d', '--build', '--wait', '--wait-timeout', '180'],
                check=True, **options,
            )
            subprocess.run(
                [sys.executable, str(ROOT / 'scripts' / (check + '.py')), *arguments],
                check=True, **options,
            )
        except (subprocess.CalledProcessError, KeyboardInterrupt):
            subprocess.run([*compose, 'logs', '--tail', '50'], check=False, **options)
            raise
        finally:
            result = subprocess.run([*compose, 'down', '--volumes'], check=False, **options)
            if result.returncode:
                print(f'Could not remove test Compose project {project}', file=sys.stderr)
                if sys.exc_info()[0] is None:
                    result.check_returncode()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('check', choices=['smoke'])
    parser.add_argument('arguments', nargs=argparse.REMAINDER)
    args = parser.parse_args()
    run(args.check, args.arguments)
