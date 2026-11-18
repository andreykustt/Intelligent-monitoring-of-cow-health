"""Build the tracked v1 contract. Run after intentional contract changes."""
import json
from pathlib import Path
S = {"type": "string"}
N = {"type": "number"}
T = {"type": "string", "format": "date-time"}

def obj(props, required=None):
    return {"type": "object", "properties": props, "required": required or list(props)}

def ref(name):
    return {"$ref": "#/components/schemas/" + name}

def arr(item):
    return {"type": "array", "items": item}

schemas = {
    "Identity": obj({"id": S, "username": S, "role": {"type": "string", "enum": ["admin", "operator"]}, "csrf_token": S, "farms": arr(S)}),
    "Login": obj({"username": S, "password": S}),
    "Measurement": obj({"value": N, "unit": S, "recorded_at": T, "received_at": T, "source": {"type":"string","enum":["synthetic","replay"]}, "stale": {"type":"boolean"}}),
    "Cow": obj({"id": S, "name": S, "farm_id": S, "status": {"type":"string","enum":["anomaly","no_anomaly","insufficient_data"]}, "latest": {"type":"object","additionalProperties":ref("Measurement")}, "analysis_at": {"type":"string","format":"date-time","nullable":True}, "analysis_delayed":{"type":"boolean","nullable":True}}),
    "CowInput": obj({"id": S, "name": S, "farm_id": S}),
    "Point": obj({"recorded_at": T, "value": {"type":"number","nullable":True}, "min": {"type":"number","nullable":True}, "max": {"type":"number","nullable":True}, "count": N}),
    "Metric": obj({"code": S, "name": S, "unit": S, "min_value": N, "max_value": N}),
    "ManualInput": obj({"cow_id": S,"observed_at":T,"payload":{"type":"object","additionalProperties":True},"version":{"type":"integer"},"reason":S},["cow_id","observed_at","payload"]),
    "ManualRecord": obj({"id":S,"kind":S,"cow_id":S,"observed_at":T,"created_at":T,"created_by":S,"created_by_name":S,"version":{"type":"integer"},"payload":{"type":"object","additionalProperties":True}}),
    "Alert": obj({"id":S,"cow_id":S,"state":{"type":"string","enum":["open","acknowledged","resolved"]},"created_at":T,"last_seen":T,"condition":S,"prediction_id":S}),
    "SimulationConfig": obj({"cow_count":{"type":"integer"},"metrics_enabled":arr(S),"interval_ms_by_metric":{"type":"object","additionalProperties":{"type":"integer"}},"duration_seconds":{"type":"integer"},"mode":{"type":"string","enum":["synthetic","replay"]},"playback_speed":N,"scenario":{"type":"string","enum":["normal","anomaly","missing"]},"seed":{"type":"integer"},"replay":arr(obj({"cow_id":S,"metric_type":S,"value":N,"recorded_at":T}))},["cow_count","metrics_enabled","interval_ms_by_metric","duration_seconds","mode","scenario","seed"]),
    "Simulation":obj({"id":S,"farm_id":S,"state":S,"config":ref("SimulationConfig"),"version":{"type":"integer"},"created_at":T,"progress":{"type":"object","additionalProperties":True}}),
    "User":obj({"id":S,"username":S,"role":{"type":"string","enum":["admin","operator"]},"active":{"type":"boolean"},"farms":arr(S)}),
    "Overview":obj({k:{"type":"integer"} for k in ["total","anomaly","no_anomaly","insufficient_data","with_data","alerts"]}),
    "Error":obj({"error":obj({"code":S,"message":S,"details":{"type":"object","additionalProperties":True}})}),
    "Generic":{"type":"object","additionalProperties":True},
}
for name in ["Cow","Alert","Simulation","User","ManualRecord"]:
    schemas[name+"Page"] = obj({"items":arr(ref(name)),"page":{"type":"integer"},"page_size":{"type":"integer"},"total":{"type":"integer"}})
paths = {}
def route(path, method, response="Generic", body=None, paged=False):
    params = []
    for part in path.split('/'):
        if part.startswith('{'):
            params.append({"name":part[1:-1],"in":"path","required":True,"schema":S})
    if method == 'get':
        for name in (["page","page_size"] if paged else []) + ["farm_id","q","sort","status","state","cow_id","metric_type","from","to"]:
            params.append({"name":name,"in":"query","schema":S})
    responses = {"200":{"description":"Success","content":{"application/json":{"schema":ref(response)}}}}
    for code in ['401','403','404','409','422','429','503']:
        responses[code] = {"description":"Typed error","content":{"application/json":{"schema":ref("Error")}}}
    op = {"operationId":method+'_'+path.strip('/').replace('/','_').replace('{','').replace('}','').replace('-','_'),"responses":responses,"parameters":params,"security":[{"session":[]}]}
    if method not in ['get'] and path != '/auth/login':
        params.append({"name":"X-CSRF-Token","in":"header","required":True,"schema":S})
    if body:
        op["requestBody"]={"required":True,"content":{"application/json":{"schema":ref(body)}}}
    if method=='post' and path=='/auth/login':
        op['security']=[]
    if method=='post' and path=='/simulations':
        op['requestBody']={"required":True,"content":{"application/json":{"schema":obj({"farm_id":S,"config":ref("SimulationConfig")})}}}
        responses['202']=responses.pop('200')
    if method=='post' and path in ['/milkings','/inspections','/laboratory-results','/body-condition-records','/veterinary-examinations']:
        params.append({"name":"Idempotency-Key","in":"header","required":True,"schema":S})
        responses['201']=responses.pop('200')
    paths.setdefault(path,{})[method]=op
for p,res,page in [('/auth/me','Identity',False),('/cows','CowPage',True),('/cows/{id}','Cow',False),('/metrics','Generic',False),('/telemetry/latest','Generic',True),('/telemetry/history','Generic',False),('/cows/{id}/measurements','Generic',False),('/cows/{id}/predictions','Generic',True),('/cows/{id}/health-summary','ManualRecordPage',True),('/alerts','AlertPage',True),('/alerts/{id}','Generic',False),('/simulations','SimulationPage',True),('/simulations/{id}','Simulation',False),('/users','UserPage',True),('/analytics/overview','Overview',False),('/analytics/{metric}','Generic',False)]:
    route(p,'get',res,paged=page)
for p,body in [('/auth/login','Login'),('/auth/logout',None),('/cows','CowInput'),('/simulations','Generic'),('/users','Generic'),('/users/{id}/password','Generic'),('/alerts/{id}/acknowledge','Generic'),('/alerts/{id}/resolve','Generic')]:
    route(p,'post',body=body)
for p,b in [('/cows/{id}','CowInput'),('/users/{id}','Generic'),('/simulations/{id}/config','SimulationConfig')]:
    route(p,'patch',body=b)
for action in ['pause','resume','stop']:
    route('/simulations/{id}/'+action,'post')
for kind in ['milkings','inspections','laboratory-results','body-condition-records','veterinary-examinations']:
    route('/'+kind,'post',body='ManualInput')
    route('/'+kind+'/{id}','get','ManualRecord')
    route('/'+kind+'/{id}','patch',body='ManualInput')
    route('/'+kind+'/{id}/revisions','get')
for p in ['/metrics','/telemetry/history','/cows/{id}/measurements','/analytics/{metric}']:
    paths[p]['get']['responses']['200']['content']['application/json']['schema']=arr(ref('Metric' if p=='/metrics' else 'Point'))
# Explicit mutation DTOs and acknowledgement shapes.
schemas['UserInput']=obj({'username':S,'password':S,'role':{'type':'string','enum':['admin','operator']},'farms':arr(S),'active':{'type':'boolean'}},['role','farms','active'])
schemas['PasswordInput']=obj({'password':S})
schemas['NoteInput']=obj({'note':S},[])
schemas['OK']=obj({'ok':{'type':'boolean'}})
schemas['ID']=obj({'id':S})
schemas['State']=obj({'state':S})
schemas['AcceptedSimulation']=obj({'id':S,'state':S})
schemas['Prediction']=obj({'id':S,'cow_id':S,'window_start':T,'window_end':T,'status':{'type':'string','enum':['anomaly','no_anomaly','insufficient_data']},'anomaly_score':{'type':'number','nullable':True},'model_version':S,'schema_version':{'type':'integer'},'created_at':T})
schemas['PredictionPage']=obj({'items':arr(ref('Prediction')),'page':{'type':'integer'},'page_size':{'type':'integer'},'total':{'type':'integer'}})
for p,m,b in [('/users','post','UserInput'),('/users/{id}','patch','UserInput'),('/users/{id}/password','post','PasswordInput'),('/alerts/{id}/acknowledge','post','NoteInput'),('/alerts/{id}/resolve','post','NoteInput')]:
    paths[p][m]['requestBody']['content']['application/json']['schema']=ref(b)
for p,m,result in [('/auth/login','post','OK'),('/auth/logout','post','OK'),('/users','post','ID'),('/users/{id}','patch','ID'),('/users/{id}/password','post','OK'),('/cows','post','CowInput'),('/cows/{id}','patch','CowInput'),('/cows/{id}/predictions','get','PredictionPage')]:
    paths[p][m]['responses']['200']['content']['application/json']['schema']=ref(result)
paths['/simulations']['post']['responses']['202']['content']['application/json']['schema']=ref('AcceptedSimulation')
for kind in ['milkings','inspections','laboratory-results','body-condition-records','veterinary-examinations']:
    paths['/'+kind]['post']['responses']['201']['content']['application/json']['schema']=ref('ID')
for action in ['pause','resume','stop']:
    op=paths['/simulations/{id}/'+action]['post']
    op['responses']['202']=op['responses'].pop('200')
# Match integer pagination and documented allowlists rather than arbitrary strings.
for path in paths.values():
    for op in path.values():
        for parameter in op['parameters']:
            if parameter['name'] in ['page','page_size']:
                parameter['schema']={'type':'integer','minimum':1,**({'maximum':100} if parameter['name']=='page_size' else {})}
schemas['TimelineEntry']={'oneOf':[obj({'entry_type':{'type':'string','enum':[kind]},'event_at':T,'data':ref(data)}) for kind,data in [('manual','ManualRecord'),('prediction','Prediction'),('alert','Alert')]]}
schemas['TimelinePage']=obj({'items':arr(ref('TimelineEntry')),'page':{'type':'integer'},'page_size':{'type':'integer'},'total':{'type':'integer'}})
paths['/cows/{id}/health-summary']['get']['responses']['200']['content']['application/json']['schema']=ref('TimelinePage')
contract={"openapi":"3.0.3","info":{"title":"Cow Health Monitor","version":"1.0.0"},"servers":[{"url":"/api/v1"}],"paths":paths,"components":{"securitySchemes":{"session":{"type":"apiKey","in":"cookie","name":"session"}},"schemas":schemas}}
Path('docs/openapi.json').write_text(json.dumps(contract,ensure_ascii=False,indent=2)+'\n')
