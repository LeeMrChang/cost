"""Exercise the deployment state machine without accessing a real Docker daemon."""
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'deploy-cloud.sh'
RELEASE = 'a' * 40 + '-123-1'
FAKE_DOCKER = r'''#!/usr/bin/env python3
import json, os, sys
a = sys.argv[1:]
if a == ['info'] and os.environ.get('FORCE_SUDO') == '1' and not os.environ.get('VIA_SUDO'):
    sys.exit(1)
if 'compose' in a and os.environ.get('COST_IMAGE_TAG') != 'ci-' + os.environ['RELEASE_ID']:
    sys.exit('Compose did not receive the release image tag')
if 'compose' in a and ('exec' in a or 'run' in a):
    # Compose can consume the remote bash -s program unless stdin is closed.
    sys.stdin.read()
with open(os.environ['CALL_LOG'], 'a') as f:
    f.write(json.dumps(a) + '\n')
fail = os.environ.get('FAIL_STAGE', '')
client = ' -uroot' in ' '.join(a)
if client and os.environ.get('MISSING_SOCKET') == '1':
    if '--protocol=tcp' not in ' '.join(a) or '-h127.0.0.1' not in ' '.join(a):
        sys.exit("Can't connect to local MySQL server through default socket (2002)")
if client and 'compose' in a and any('SELECT 1' in x for x in a) and fail == 'preflight':
    sys.exit('MySQL root TCP authentication failed')
if 'build' in a and fail == 'build': sys.exit(1)
if any('mysqldump' in x and '--no-create-info' not in x for x in a):
    if fail == 'backup': sys.exit(1)
    print('-- fixture SQL backup')
elif 'inspect' in a:
    print('sha256:old-image')
elif 'ps' in a:
    print('old-api' if a[-1] == 'api' else 'existing-mysql')
elif 'run' in a and 'migrate' in a:
    if fail == 'migrate': sys.exit(1)
elif any('--no-create-info' in x for x in a):
    print('changed' if fail == 'restore' and a[0] == 'exec' else 'same-data')
elif 'up' in a and fail == 'health': sys.exit(1)
'''


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / 'deploy').mkdir()
        (self.root / 'deploy/.env').write_text('server-only-secret\n')
        (self.root / 'incoming').mkdir()
        self.log = self.root / 'calls.jsonl'
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        for name, body in [('docker', FAKE_DOCKER), ('curl', '#!/bin/sh\nexit 0\n'),
                           ('flock', '#!/bin/sh\nexit 0\n'),
                           ('sudo', '#!/usr/bin/env python3\nimport os,sys\nos.environ.pop("COST_IMAGE_TAG", None)\nos.environ["VIA_SUDO"]="1"\nos.execvp(sys.argv[2], sys.argv[2:])\n')]:
            p = self.bin / name
            p.write_text(body)
            p.chmod(0o755)
        self.archive()

    def archive(self, member='backend/go.mod'):
        with tarfile.open(self.root / f'incoming/{RELEASE}.tar.gz', 'w:gz') as tf:
            for name in [member, 'deploy/compose.cloud.yaml']:
                data = b'fixture\n'
                info = tarfile.TarInfo(name)
                info.size = len(data)
                tf.addfile(info, io.BytesIO(data))

    def run_deploy(self, fail='', release=RELEASE, sudo=False, missing_socket=False):
        env = dict(os.environ, PATH=str(self.bin) + ':' + os.environ['PATH'],
                   CALL_LOG=str(self.log), FAIL_STAGE=fail, RELEASE_ID=release,
                   FORCE_SUDO='1' if sudo else '0',
                   MISSING_SOCKET='1' if missing_socket else '0')
        return subprocess.run(['bash', '-s', '--', release, str(self.root)],
                              input=SCRIPT.read_text(), env=env, capture_output=True, text=True)

    def calls(self):
        return [json.loads(s) for s in self.log.read_text().splitlines()] if self.log.exists() else []

    def test_success_preserves_env_and_publishes_after_migration(self):
        result = self.run_deploy()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls()
        index = lambda word: next(i for i, a in enumerate(calls) if word in a)
        self.assertLess(index('build'), index('stop'))
        self.assertLess(index('stop'), index('migrate'))
        self.assertLess(index('migrate'), index('up'))
        self.assertTrue((self.root / 'current').is_symlink())
        self.assertEqual((self.root / 'deploy/.env').read_text(), 'server-only-secret\n')
        self.assertTrue(list((self.root / 'backups').glob('*.sql')))

    def test_success_restarts_backup_services_after_migration(self):
        result = self.run_deploy()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls()
        migration = next(i for i, c in enumerate(calls) if 'migrate' in c)
        starts = [i for i, c in enumerate(calls) if 'up' in c and 'backup' in c and 'backup-verify' in c]
        self.assertEqual(len(starts), 1, 'deployment must apply the released backup workers')
        self.assertLess(migration, starts[0])

    def test_build_failure_keeps_old_api_running(self):
        self.assertNotEqual(self.run_deploy('build').returncode, 0)
        self.assertFalse(any('stop' in c or 'migrate' in c for c in self.calls()))

    def test_sudo_keeps_the_unique_release_image_tag(self):
        result = self.run_deploy(sudo=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((self.root / 'current/SUCCESS').exists())

    def test_missing_default_socket_uses_tcp_for_backup_and_restore(self):
        result = self.run_deploy(missing_socket=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((self.root / 'current/SUCCESS').exists())
        clients = [c for c in self.calls() if ' -uroot' in ' '.join(c)]
        self.assertGreaterEqual(len(clients), 7)

    def test_mysql_preflight_failure_does_not_stop_api(self):
        self.assertNotEqual(self.run_deploy('preflight').returncode, 0)
        self.assertFalse(any('stop' in c or 'migrate' in c for c in self.calls()))

    def test_backup_failure_restarts_original_container(self):
        self.assertNotEqual(self.run_deploy('backup').returncode, 0)
        self.assertIn(['start', 'old-api'], self.calls())
        self.assertFalse(any('migrate' in c for c in self.calls()))

    def test_restore_mismatch_prevents_migration_and_restarts_old_api(self):
        self.assertNotEqual(self.run_deploy('restore').returncode, 0)
        self.assertIn(['start', 'old-api'], self.calls())
        self.assertFalse(any('migrate' in c for c in self.calls()))

    def test_migration_failure_never_restarts_old_code(self):
        self.assertNotEqual(self.run_deploy('migrate').returncode, 0)
        self.assertNotIn(['start', 'old-api'], self.calls())
        self.assertFalse(any('up' in c for c in self.calls()))
        self.assertFalse((self.root / 'current').exists())

    def test_unhealthy_api_is_not_published(self):
        self.assertNotEqual(self.run_deploy('health').returncode, 0)
        self.assertFalse((self.root / 'current').exists())
        self.assertNotIn(['start', 'old-api'], self.calls())

    def test_unsafe_archive_rejected_before_docker(self):
        self.archive('../escape')
        self.assertNotEqual(self.run_deploy().returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_archive_cannot_replace_server_env(self):
        self.archive('deploy/.env')
        self.assertNotEqual(self.run_deploy().returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_invalid_release_rejected(self):
        self.assertNotEqual(self.run_deploy(release='../../bad').returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_successful_release_is_idempotent(self):
        first = self.run_deploy()
        self.assertEqual(first.returncode, 0, first.stderr)
        before = self.calls()
        self.assertEqual(self.run_deploy().returncode, 0)
        self.assertEqual(self.calls(), before)


if __name__ == '__main__':
    unittest.main()
