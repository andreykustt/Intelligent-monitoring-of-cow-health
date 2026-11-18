"""API acceptance using curl; creates demo records."""
import time
import subprocess
import uuid
from datetime import datetime, timezone
from http_client import Client

admin, operator, anon = Client(), Client(), Client()
anon.call('/cows', expected=401)
admin.login('admin')
operator.login('operator')
print('PASS login, session identity, anonymous rejection')
operator.call('/users', expected=403)
operator.call('/simulations', 'POST', {}, expected=403)
operator.call('/cows', 'POST', {}, expected=403)
admin.call('/cows', 'POST', {}, expected=403, headers={'X-CSRF-Token':'wrong'})
admin.call('/cows?page_size=101', expected=422)
admin.call('/cows?sort=sql-injection', expected=422)
admin.call('/cows?farm_id=not-permitted')['items'] == []
assert admin.call('/cows?farm_id=not-permitted')['total'] == 0
print('PASS RBAC, CSRF, scope and query validation')
page = operator.call('/cows')
assert page['total'] >= 200 and len(page['items']) == 25
cow = page['items'][0]['id']
operator.call('/cows/'+cow)
operator.call('/cows/UNKNOWN', expected=404)
assert len(operator.call('/metrics')) == 9
operator.call('/analytics/overview')
operator.call('/telemetry/history?cow_id='+cow+'&metric_type=activity_index')
operator.call('/telemetry/history?metric_type=not-known', expected=422)
print('PASS list, card, metric registry, overview and history')
payloads = {
 'milkings':{'yield_l':12,'flow_l_min':1.5,'temperature_c':35,'conductivity_fl':4,'conductivity_fr':4,'conductivity_rl':4,'conductivity_rr':4},
 'inspections':{'appetite':'обычный','gait':'без замечаний','condition':'без замечаний'},
 'laboratory-results':{'test':'bhb','value':1.2,'unit':'mmol/L'},
 'body-condition-records':{'bcs':3.25,'hoof':'без замечаний'},
 'veterinary-examinations':{'reproductive_status':'не указан','notes':'Проверка учебной записи'},
}
for kind, payload in payloads.items():
    key = str(uuid.uuid4())
    body = {'cow_id':cow,'observed_at':datetime.now(timezone.utc).isoformat(),'payload':payload}
    created = operator.call('/'+kind, 'POST', body, 201, {'Idempotency-Key':key})
    repeated = operator.call('/'+kind, 'POST', body, 201, {'Idempotency-Key':key})
    assert created['id'] == repeated['id']
    path = '/'+kind+'/'+created['id']
    assert operator.call(path)['version'] == 1
    revised = body | {'version':1, 'reason':'Уточнение учебной записи'}
    operator.call(path, 'PATCH', revised)
    operator.call(path, 'PATCH', revised, 409)
    assert len(operator.call(path+'/revisions')) == 2
assert operator.call('/cows/'+cow+'/health-summary')['total'] >= 5
print('PASS all manual forms, idempotency, corrections, optimistic locking and revisions')
config = {'cow_count':2,'metrics_enabled':['body_temperature_c','heart_rate_bpm','activity_index'],
 'interval_ms_by_metric':{'body_temperature_c':5000,'heart_rate_bpm':5000,'activity_index':5000},
 'duration_seconds':120,'mode':'synthetic','scenario':'normal','seed':42}
sim = admin.call('/simulations', 'POST', {'farm_id':'demo','config':config}, 202)
for _ in range(40):
    current = admin.call('/simulations/'+sim['id'])
    if current['state'] == 'running':
        break
    time.sleep(.5)
assert current['state'] == 'running', current
for _ in range(40):
    readings = operator.call('/cows/'+cow)['latest']
    if len(readings) >= 3:
        break
    time.sleep(.5)
assert len(readings) >= 3, readings
subprocess.run(['python3','scripts/check_stream.py'],check=True)
admin.call('/simulations/'+sim['id']+'/pause','POST',{},202)
for _ in range(40):
    if admin.call('/simulations/'+sim['id'])['state']=='paused':
        break
    time.sleep(.5)
assert admin.call('/simulations/'+sim['id'])['state']=='paused'
admin.call('/simulations/'+sim['id']+'/resume','POST',{},202)
for _ in range(40):
    if admin.call('/simulations/'+sim['id'])['state']=='running':
        break
    time.sleep(.5)
assert admin.call('/simulations/'+sim['id'])['state']=='running'
admin.call('/simulations/'+sim['id']+'/stop','POST',{},202)
print('PASS simulator start, persisted telemetry, pause, resume and stop')
operator.call('/auth/logout','POST',{})
operator.call('/auth/me',expected=401)
print('PASS logout revokes session')
print('API acceptance passed')
