"""Administrative API and cross-farm checks through curl."""
import secrets
import subprocess
import uuid
from http_client import Client

a=Client();a.login('admin')
a.call('/users/'+a.call('/auth/me')['id'],'PATCH',{'role':'operator','farms':['demo'],'active':False},409)
print('PASS last administrator cannot be disabled')
name='test-'+str(uuid.uuid4())[:8];password=secrets.token_hex(16)
u=a.call('/users','POST',{'username':name,'password':password,'role':'operator','farms':['demo'],'active':True})
b=Client();b.call('/auth/login','POST',{'username':name,'password':password});me=b.call('/auth/me');b.csrf=me['csrf_token']
a.call('/users/'+u['id'],'PATCH',{'role':'operator','farms':['demo'],'active':False})
b.call('/auth/me',expected=401)
a.call('/users/'+u['id'],'PATCH',{'role':'operator','farms':['demo'],'active':True})
new_password=secrets.token_hex(16)
a.call('/users/'+u['id']+'/password','POST',{'password':new_password})
b.call('/auth/login','POST',{'username':name,'password':password},401)
b.call('/auth/login','POST',{'username':name,'password':new_password})
print('PASS user creation, block revokes session, password reset')
query="INSERT INTO farms VALUES('scope-test','Scope test') ON CONFLICT DO NOTHING; INSERT INTO cows(id,farm_id,name) VALUES('SCOPE-TEST','scope-test','Isolated') ON CONFLICT DO NOTHING;"
subprocess.run(['docker','compose','exec','-T','postgres','sh','-c','psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1'],input=query,text=True,capture_output=True,check=True)
a.call('/cows/SCOPE-TEST',expected=404)
a.call('/cows/SCOPE-TEST/measurements?metric_type=activity_index',expected=404)
a.call('/users/'+u['id'],'PATCH',{'role':'operator','farms':['scope-test'],'active':True},403)
a.call('/stream?farm_id=scope-test',expected=403)
print('PASS cross-farm card/history/WS isolation and scope assignment rejection')
