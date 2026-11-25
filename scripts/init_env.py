"""Create local random credentials once, without printing them."""
import secrets
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SECRET_KEYS = (
    'POSTGRES_PASSWORD', 'RABBITMQ_PASSWORD', 'ADMIN_PASSWORD',
    'OPERATOR_PASSWORD', 'INTERNAL_TOKEN',
)


def create_env(path, **overrides):
    values = dict(
        line.split('=', 1)
        for line in (ROOT / '.env.example').read_text().splitlines()
        if line and not line.startswith('#')
    )
    values.update({key: secrets.token_hex(20) for key in SECRET_KEYS})
    values.update(overrides)
    with path.open('x') as file:
        file.writelines(f'{key}={value}\n' for key, value in values.items())
    path.chmod(0o600)
    return values


if __name__ == '__main__':
    try:
        create_env(ROOT / '.env')
    except FileExistsError:
        raise SystemExit('.env already exists; left unchanged') from None
    print('Created .env with random local credentials; do not commit it.')
