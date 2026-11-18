"""Create a 60-minute synthetic replay for a complete ML window in ~36 seconds."""
import json
import random
import sys
import urllib.request
from pathlib import Path
from datetime import datetime, timedelta, timezone
sys.path.insert(0, str(Path(__file__).parent))
# Reuse only the small client, without running smoke's top-level scenarios.
from http_client import Client, env
client = Client()
client.login('admin')
end = datetime.now(timezone.utc).replace(second=0, microsecond=0)
end -= timedelta(minutes=end.minute % 5)
rng = random.Random(42)
records=[]
for minute in range(60):
    at=end-timedelta(minutes=60-minute)
    for metric,value in [('body_temperature_c',41+rng.gauss(0,.1)),('heart_rate_bpm',110+rng.gauss(0,.4)),('activity_index',.1+rng.gauss(0,.001))]:
        records.append({'cow_id':'COW-001','metric_type':metric,'value':value,'recorded_at':at.isoformat()})
config={'cow_count':1,'metrics_enabled':['body_temperature_c','heart_rate_bpm','activity_index'],'interval_ms_by_metric':{'body_temperature_c':5000,'heart_rate_bpm':5000,'activity_index':5000},'duration_seconds':120,'mode':'replay','scenario':'anomaly','seed':42,'playback_speed':100,'replay':records}
result=client.call('/simulations','POST',{'farm_id':'demo','config':config},202)
print(json.dumps({'simulation_id':result['id'],'expected_window_end':end.isoformat(),'synthetic':True}))
