#!/usr/bin/env python3
"""Enable the existing private Redis for the next Cost API deployment; never print secrets."""
import argparse
import os
from pathlib import Path
import re
import tempfile


def assignments(lines):
    found = {}
    for i, line in enumerate(lines):
        if not line.strip() or line.lstrip().startswith('#'):
            continue
        match = re.fullmatch(r'\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=(.*)', line.rstrip('\r\n'))
        if match is None:
            raise ValueError('Unsupported API environment syntax; no changes made.')
        key, value = match.groups()
        value = value.lstrip()
        if value.startswith(("'", '"')):
            quote = value[0]
            pattern = re.escape(quote) + r'(?:\\.|[^' + re.escape(quote) + r'\\])*' + re.escape(quote) + r'\s*(?:#.*)?'
            if re.fullmatch(pattern, value) is None:
                raise ValueError('Multiline or unsupported quoted values require manual review.')
        found.setdefault(key, []).append(i)
    return found


def configure(redis_env: Path, api_env: Path):
    if redis_env.is_symlink() or api_env.is_symlink() or not redis_env.is_file() or not api_env.is_file():
        raise ValueError('Both environments must be existing regular files, not symlinks.')
    pairs = [line.split('=', 1) for line in redis_env.read_text().splitlines() if line and not line.startswith('#')]
    values = dict(pairs)
    if len(values) != len(pairs) or set(values) != {'COST_REDIS_PASSWORD', 'COST_REDIS_IMAGE', 'COST_REDIS_NETWORK'}:
        raise ValueError('Invalid managed Redis environment.')
    password = values['COST_REDIS_PASSWORD']
    if re.fullmatch(r'[0-9a-f]{64}', password) is None:
        raise ValueError('Invalid managed Redis credentials.')
    settings = {'COST_REDIS_ADDR': 'cost-redis:6379', 'COST_REDIS_PASSWORD': password}
    lines = api_env.read_text().splitlines(keepends=True)
    indices_by_key = assignments(lines)
    for key, value in settings.items():
        indices = indices_by_key.get(key, [])
        if len(indices) > 1:
            raise ValueError('Duplicate Redis settings in API environment; inspect before configuring.')
        if indices:
            lines[indices[0]] = key+'='+value+'\n'
        else:
            if lines and not lines[-1].endswith('\n'):
                lines[-1] += '\n'
            lines.append(key+'='+value+'\n')
    temp_name = None
    try:
        with tempfile.NamedTemporaryFile(mode='w', dir=api_env.parent, prefix='.redis-config-', delete=False) as f:
            temp_name = f.name
            os.chmod(temp_name, 0o600)
            f.writelines(lines)
            f.flush()
            os.fsync(f.fileno())
        os.replace(temp_name, api_env)
        temp_name = None
    finally:
        if temp_name is not None:
            os.unlink(temp_name)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--redis-env', type=Path, default=Path('/home/ubuntu/cost/redis/.env'))
    parser.add_argument('--api-env', type=Path, default=Path('/home/ubuntu/cost/deploy/.env'))
    args = parser.parse_args()
    try:
        configure(args.redis_env, args.api_env)
    except (OSError, ValueError, UnicodeError):
        parser.exit(1, 'Redis configuration failed; check file paths and environment format. No credentials printed.\n')
    print('Redis configured for the next Cost API deployment. Credentials retained privately; no service restarted.')
