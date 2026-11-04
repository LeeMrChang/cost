#!/usr/bin/env python3
"""Aggregate safe request timings; never print raw logs, tokens or ledger data."""
import argparse
import math
import re
import subprocess
import sys

ROUTES = {'/api/v1/auth/me', '/api/v1/categories', '/api/v1/stats',
          '/api/v1/ledgers/me', '/api/v1/accounts', '/api/v1/entries'}
SCALES = {'ns': 0.000001, 'µs': 0.001, 'μs': 0.001, 'us': 0.001,
          'ms': 1, 's': 1000, 'm': 60000, 'h': 3600000}


def duration_ms(value):
    parts = re.findall(r'(\d+(?:\.\d+)?)(ns|µs|μs|us|ms|s|m|h)', value)
    if not parts or ''.join(n+unit for n, unit in parts) != value:
        return None
    return sum(float(n)*SCALES[unit] for n, unit in parts)


def report(raw):
    samples = {}
    for line in raw.splitlines():
        if not re.search(r'(?:^|\s)msg=request(?:\s|$)', line):
            continue
        fields = {key: value.strip('"') for key, value in re.findall(r'(?:^|\s)(method|path|duration)=([^\s]+)', line)}
        path = fields.get('path', '').split('?', 1)[0]
        if re.fullmatch(r'/api/v1/entries/\d+(?:/restore)?', path):
            path = re.sub(r'/\d+', '/{id}', path)
        if path not in ROUTES and path not in {'/api/v1/entries/{id}', '/api/v1/entries/{id}/restore'}:
            continue
        method = fields.get('method')
        ms = duration_ms(fields.get('duration', ''))
        if method not in {'GET', 'POST', 'PATCH', 'PUT', 'DELETE'} or ms is None:
            continue
        samples.setdefault((method, path), []).append(ms)
    result = []
    for (method, path), values in sorted(samples.items()):
        values.sort()
        percentile = lambda p: round(values[math.ceil(len(values)*p)-1], 3)
        result.append({'method': method, 'path': path, 'count': len(values),
                       'p50_ms': percentile(.5), 'p95_ms': percentile(.95), 'max_ms': round(values[-1], 3)})
    return result


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--container', default='cost-cloud-api-1')
    parser.add_argument('--since', default='5m', help='Short reproduction window, e.g. 2m or 5m')
    args = parser.parse_args(argv)
    if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.-]{0,127}', args.container) or not re.fullmatch(r'[1-9][0-9]*[smh]', args.since):
        parser.error('Invalid container or time window')
    try:
        proc = subprocess.run(['docker', 'logs', '--since', args.since, '--tail', '10000', args.container],
                              capture_output=True, text=True, errors='replace', timeout=30)
    except (OSError, subprocess.TimeoutExpired):
        print('Cannot read Docker logs; check Docker access on the server.', file=sys.stderr)
        return 1
    if proc.returncode:
        print('Docker log read failed; verify the container and permissions.', file=sys.stderr)
        return 1
    print(f'Window: last {args.since}; at most 10000 log lines; durations include server-side authentication and storage.')
    rows = report(proc.stdout+'\n'+proc.stderr)
    if not rows:
        print('No homepage/write request samples. Reproduce saving an entry and returning home, then run again.')
        return 0
    print('METHOD PATH COUNT P50_MS P95_MS MAX_MS')
    for row in rows:
        print('{method} {path} {count} {p50_ms:.3f} {p95_ms:.3f} {max_ms:.3f}'.format(**row))
    print('These are backend durations, not full phone rendering/network latency; compare with client-side timing.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
