"""Verify actual WS upgrade and data frames using curl, no browser."""
import json
import subprocess
from http_client import Client, ORIGIN

client=Client()
client.login('operator')
client.call('/stream?farm_id=forbidden',expected=403)
options={'url':ORIGIN+'/api/v1/stream?farm_id=demo','cookie':client.cookie,'include':None,'no-buffer':None,'max-time':'3'}
headers={'Origin':ORIGIN,'Connection':'Upgrade','Upgrade':'websocket','Sec-WebSocket-Version':'13','Sec-WebSocket-Key':'dGhlIHNhbXBsZSBub25jZQ=='}
lines=['silent','show-error']+[k if v is None else k+' = '+json.dumps(v) for k,v in options.items()]
lines += ['header = '+json.dumps(k+': '+v) for k,v in headers.items()]
r=subprocess.run(['curl','--config','-'],input=('\n'.join(lines)+'\n').encode(),capture_output=True)
raw=r.stdout.decode('utf8',errors='replace')
assert r.returncode in [0,28],r.stderr
assert raw.startswith('HTTP/1.1 101'),raw[:400]
assert 'event_id' in raw and ('telemetry.update.v1' in raw or 'simulation.status_changed.v1' in raw), 'No committed event received; run an active simulation first'
print('PASS WebSocket scope rejection, HTTP 101 upgrade and committed event frames via curl')
