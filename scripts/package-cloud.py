#!/usr/bin/env python3
"""Build a minimal source deployment archive, without local credentials or data."""
import argparse
from pathlib import Path
import tarfile


def build_archive(root: Path, output: Path) -> int:
    fixed = [
        'backend/Dockerfile', 'backend/.dockerignore', 'backend/go.mod', 'backend/go.sum',
        'deploy/compose.cloud.yaml', 'deploy/.env.example', 'deploy/Caddyfile',
        'deploy/mysql/01-users.sh',
        'deploy/backup/common.sh', 'deploy/backup/backup.sh', 'deploy/backup/verify.sh',
        'docs/ops/2026-10-22-docker-cloud-deployment.md',
    ]
    files = [root / name for name in fixed]
    for folder in ('backend/cmd', 'backend/internal'):
        for path in (root / folder).rglob('*'):
            if path.suffix in ('.go', '.sql') and not path.name.endswith('_test.go'):
                files.append(path)
    for path in files:
        if not path.is_file() or path.is_symlink() or any(p.is_symlink() for p in path.parents if p != root):
            raise ValueError(f'Expected regular source file: {path.relative_to(root)}')
    output.parent.mkdir(parents=True, exist_ok=True)
    # Exclusive creation avoids replacing an existing reviewed deployment archive.
    with output.open('xb') as stream:
        with tarfile.open(fileobj=stream, mode='w:gz') as archive:
            for path in sorted(set(files)):
                archive.add(path, arcname=str(path.relative_to(root)), recursive=False)
    return len(set(files))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    count = build_archive(Path(__file__).resolve().parents[1], args.output.resolve())
    print(f'Packaged {count} source files: {args.output}')
