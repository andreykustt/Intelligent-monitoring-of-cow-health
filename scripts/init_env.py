"""Create local random credentials once, without printing them."""
from pathlib import Path
import secrets
p = Path('.env')
if p.exists():
    raise SystemExit('.env already exists; left unchanged')
data = Path('.env.example').read_text()
for key in ['POSTGRES_PASSWORD', 'RABBITMQ_PASSWORD', 'ADMIN_PASSWORD', 'OPERATOR_PASSWORD', 'INTERNAL_TOKEN']:
    data = data.replace(key+'=\n', key+'='+secrets.token_hex(20)+'\n')
p.write_text(data)
p.chmod(0o600)
print('Created .env with random local credentials; do not commit it.')
