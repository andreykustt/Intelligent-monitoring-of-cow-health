"""Curl-based API client; credentials sent through stdin, never printed."""
import subprocess
import tempfile
import json
import os
import time
from pathlib import Path

from init_env import ROOT

ENV_FILE = Path(os.environ.get('COWHEALTH_ENV_FILE', ROOT / '.env'))
env = dict(line.split('=', 1) for line in ENV_FILE.read_text().splitlines() if line and not line.startswith('#'))
ORIGIN = env.get('APP_ORIGIN', 'http://localhost:8080')


def compose_args(*args):
    command = ['docker', 'compose', '--file', str(ROOT / 'compose.yaml'), '--env-file', str(ENV_FILE)]
    if project := os.environ.get('COWHEALTH_COMPOSE_PROJECT'):
        command.extend(['--project-name', project])
    return [*command, *args]


def wait_for_simulation(client, simulation_id, state, timeout=30):
    deadline = time.monotonic() + timeout
    while True:
        simulation = client.call('/simulations/' + simulation_id)
        actual = simulation['state']
        if actual == state:
            return simulation
        if actual in {'stopped', 'failed', 'completed'} or time.monotonic() >= deadline:
            raise RuntimeError(f'Simulation {simulation_id}: expected {state}, got {actual}')
        time.sleep(0.5)


class Client:
    def __init__(self):
        self.temp = tempfile.TemporaryDirectory(prefix='cowhealth-curl-')
        self.cookie = str(Path(self.temp.name) / 'cookies')
        self.csrf = ''
    def call(self, path, method='GET', body=None, expected=200, headers=None):
        options = {'url':ORIGIN+'/api/v1'+path, 'request':method, 'cookie':self.cookie,
                   'cookie-jar':self.cookie, 'max-time':'20', 'write-out':'\\n%{http_code}'}
        lines = ['silent', 'show-error']
        for key, value in options.items():
            lines.append(key+' = '+json.dumps(value))
        for key,value in {'Content-Type':'application/json','Origin':ORIGIN,'X-CSRF-Token':self.csrf,**(headers or {})}.items():
            lines.append('header = '+json.dumps(key+': '+value))
        if body is not None:
            lines.append('data = '+json.dumps(json.dumps(body)))
        result = subprocess.run(['curl','--config','-'], input='\n'.join(lines)+'\n', text=True, capture_output=True, check=True)
        raw,status = result.stdout.rsplit('\n',1)
        data = json.loads(raw)
        assert int(status) == expected, (method,path,status,data)
        return data
    def login(self, role):
        self.call('/auth/login', 'POST', {'username':role, 'password':env[role.upper()+'_PASSWORD']})
        me = self.call('/auth/me')
        assert me['role'] == role
        self.csrf = me['csrf_token']
