"""Controlled local baseline. Measures HTTP wall time including curl process overhead."""
import json
import platform
import statistics
import subprocess
import time
from pathlib import Path
from http_client import Client

client=Client();client.login('admin')
def count():
    r=subprocess.run(['docker','compose','exec','-T','postgres','sh','-c','psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT count(*) FROM sensor_measurements"'],capture_output=True,text=True,check=True)
    return int(r.stdout.strip())
metrics=[m['code'] for m in client.call('/metrics')]
config={'cow_count':200,'metrics_enabled':metrics,'interval_ms_by_metric':{m:5000 for m in metrics},'duration_seconds':30,'mode':'synthetic','scenario':'normal','seed':42}
before=count();started=time.monotonic()
sim=client.call('/simulations','POST',{'farm_id':'demo','config':config},202)
latencies=[]
while time.monotonic()-started<20:
    t=time.monotonic();client.call('/cows');latencies.append((time.monotonic()-t)*1000)
    time.sleep(.25)
seconds=time.monotonic()-started;stored=count()-before
client.call('/simulations/'+sim['id']+'/stop','POST',{},202)
report={'date':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),'host':platform.platform(),'logical_cpus':__import__('os').cpu_count(),'cows':200,'metrics':len(metrics),'interval_ms':5000,'duration_measured_seconds':round(seconds,2),'stored_events_delta':stored,'observed_db_events_per_second':round(stored/seconds,2),'http_list_requests':len(latencies),'http_list_wall_p95_ms':round(sorted(latencies)[int(len(latencies)*.95)],2),'http_list_wall_median_ms':round(statistics.median(latencies),2),'notes':['Includes startup and curl subprocess overhead.','Single local run; not a capacity guarantee.','Other demo history remains in database.','1000-cow extended run and browser render latency not measured.']}
Path('docs/load-report.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report,indent=2))
