"""HTTP/broker/DB acceptance; no browser and no additional packages."""
import json
import subprocess
import time
import uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path
from http_client import Client, env
admin=Client()
admin.login('admin')

def sql(query):
    p=subprocess.run(['docker','compose','exec','-T','postgres','sh','-c','psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -v ON_ERROR_STOP=1'],input=query,text=True,capture_output=True,check=True)
    return p.stdout.strip()

def internal(event,expected):
    code = '''import json,urllib.request,urllib.error
payload = PAYLOAD
req = urllib.request.Request('http://ingest-service:8080/internal/v1/telemetry',data=json.dumps(payload).encode(),headers={'Content-Type':'application/json','Authorization':'Bearer '+TOKEN})
try:
 with urllib.request.urlopen(req,timeout=15) as r: status=r.status
except urllib.error.HTTPError as e: status=e.code
print(status)
'''.replace('PAYLOAD',repr(event)).replace('TOKEN',repr(env['INTERNAL_TOKEN']))
    p=subprocess.run(['docker','compose','exec','-T','ml-service','python','-'],input=code,text=True,capture_output=True,check=True)
    assert int(p.stdout.strip())==expected,(p.stdout,p.stderr)

sim=admin.call('/simulations')['items'][0]['id']
now=datetime.now(timezone.utc).replace(microsecond=0)
latest={'schema_version':1,'event_id':str(uuid.uuid4()),'simulation_id':sim,'cow_id':'COW-200','sensor_id':'COW-200:activity_index','recorded_at':now.isoformat(),'metric_type':'activity_index','value':.44,'source':'replay'}
internal(latest,202)
internal(latest,202)
old=latest | {'event_id':str(uuid.uuid4()),'recorded_at':(now-timedelta(hours=2)).isoformat(),'value':.2}
internal(old,202)
for _ in range(30):
    count=sql("SELECT count(*) FROM sensor_measurements WHERE event_id IN ('"+latest['event_id']+"','"+old['event_id']+"');")
    if count=='2':break
    time.sleep(.5)
assert count=='2',count
assert sql("SELECT event_id FROM cow_latest_state WHERE cow_id='COW-200' AND metric_type='activity_index';")==latest['event_id']
print('PASS duplicate event dedupe, old-event history, latest-value ordering')
assert admin.call('/cows/COW-200')['latest']['activity_index']['value']==.44
print('PASS committed reading is visible through REST')
# Stop only inference; keep HTTP request runner alive by using ingest container wget below.
subprocess.run(['docker','compose','stop','ml-service'],check=True,capture_output=True)
try:
    event=latest | {'event_id':str(uuid.uuid4()),'recorded_at':(now+timedelta(seconds=1)).isoformat()}
    # BusyBox wget in an existing Go service: no package/browser installation.
    code='wget -q -O - --header="Content-Type: application/json" --header="Authorization: Bearer $INTERNAL_TOKEN" --post-data="$1" http://ingest-service:8080/internal/v1/telemetry'
    p=subprocess.run(['docker','compose','exec','-T','ingest-service','sh','-c',code,'check',json.dumps(event)],text=True,capture_output=True,check=True)
    assert json.loads(p.stdout)['accepted_by']=='broker'
    for _ in range(30):
        count=sql("SELECT count(*) FROM sensor_measurements WHERE event_id='"+event['event_id']+"';")
        if count=='1':break
        time.sleep(.5)
    assert count=='1'
    print('PASS ML outage does not block telemetry ingestion or storage')
finally:
    subprocess.run(['docker','compose','start','ml-service'],check=True,capture_output=True)
subprocess.run(['docker','compose','stop','rabbitmq'],check=True,capture_output=True)
try:
    event=latest | {'event_id':str(uuid.uuid4())}
    internal(event,503)
    print('PASS broker outage returns 503 rather than false acceptance')
finally:
    subprocess.run(['docker','compose','start','rabbitmq'],check=True,capture_output=True)
print('Delivery checks passed; services restored')
