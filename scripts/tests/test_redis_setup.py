"""Run the installer against a Docker boundary substitute, never a business server."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'setup-cost-redis.sh'
FAKE = r'''#!/usr/bin/env python3
import json, os, sys, time
a = sys.argv[1:]
with open(os.environ['CALL_LOG'], 'a') as f: f.write(json.dumps(a)+'\n')
if a == ['info'] and os.environ.get('FAIL_DOCKER'): sys.exit(1)
if a[:2] == ['network','inspect'] and os.environ.get('FAIL_NETWORK'): sys.exit(1)
if a[:2] == ['container','inspect']:
    if os.environ.get('UNKNOWN_CONTAINER'): print('unrelated|redis')
    elif os.environ.get('OWNER_DIR') or (os.environ.get('STATE_FILE') and os.path.exists(os.environ['STATE_FILE'])):
        d=os.environ.get('OWNER_DIR') or open(os.environ['STATE_FILE']).read()
        print('cost-cache|redis'+('|'+d+'|'+d+'/compose.yaml' if 'working_dir' in ' '.join(a) else ''))
    else: sys.exit(1)
if a[:1] == ['port']:
    if os.environ.get('FAIL_PORT'): sys.exit(1)
    if os.environ.get('PUBLIC_PORT'): print('6379/tcp -> 0.0.0.0:6379')
if 'compose' in a:
    if 'up' in a and os.environ.get('FAIL_UP'): sys.exit(1)
    if 'up' in a and os.environ.get('BLOCK_UP'):
        open(os.environ['BLOCK_UP']+'.ready','w').close()
        for _ in range(500):
            if os.path.exists(os.environ['BLOCK_UP']+'.release'): break
            time.sleep(.01)
        else: sys.exit(1)
    if 'up' in a and os.environ.get('STATE_FILE'):
        from pathlib import Path
        open(os.environ['STATE_FILE'],'w').write(str(Path(a[a.index('--env-file')+1]).parent))
    if 'exec' in a: print('NOAUTH' if os.environ.get('FAIL_PING') else 'PONG')
'''


class RedisSetupTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.target = self.root / 'redis'
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        self.log = self.root / 'calls.jsonl'
        p = self.bin / 'docker'
        p.write_text(FAKE)
        p.chmod(0o755)
        lock = self.bin / 'flock'
        lock.write_text('#!/usr/bin/env python3\nimport fcntl,sys\ntry: fcntl.flock(int(sys.argv[-1]),fcntl.LOCK_EX|fcntl.LOCK_NB)\nexcept BlockingIOError: sys.exit(1)\n')
        lock.chmod(0o755)

    def run_setup(self, target=None, **flags):
        env = dict(os.environ, PATH=str(self.bin)+':'+os.environ['PATH'], CALL_LOG=str(self.log), **flags)
        return subprocess.run(['bash', str(SCRIPT), str(target or self.target)], env=env, capture_output=True, text=True)

    def calls(self):
        return [json.loads(x) for x in self.log.read_text().splitlines()] if self.log.exists() else []

    def test_installs_only_redis_with_private_generated_credentials(self):
        result = self.run_setup()
        self.assertEqual(result.returncode, 0, result.stderr)
        env = (self.target / '.env').read_text()
        password = re.search(r'^COST_REDIS_PASSWORD=([0-9a-f]{64})$', env, re.M).group(1)
        self.assertEqual((self.target / '.env').stat().st_mode & 0o777, 0o600)
        self.assertEqual(self.target.stat().st_mode & 0o777, 0o700)
        self.assertNotIn(password, result.stdout + result.stderr)
        starts = [a for a in self.calls() if 'compose' in a and 'up' in a]
        self.assertEqual(len(starts), 1)
        self.assertEqual(starts[0][-1], 'redis')
        self.assertIn('--no-deps', starts[0])
        self.assertIn('--wait', starts[0])
        self.assertFalse(any(x in a for a in self.calls() for x in ['stop', 'down', 'rm', 'mysql', 'api']))
        self.assertIn(['port', 'cost-cache-redis-1'], self.calls())

    def test_retry_preserves_password_and_configuration(self):
        self.assertEqual(self.run_setup().returncode, 0)
        before = (self.target / '.env').read_bytes()
        self.assertEqual(self.run_setup(OWNER_DIR=str(self.target.resolve())).returncode, 0)
        self.assertEqual((self.target / '.env').read_bytes(), before)

    def test_same_project_from_another_directory_is_not_taken_over(self):
        self.assertEqual(self.run_setup().returncode, 0)
        before = len(self.calls())
        other = self.root / 'other-installation'
        result = self.run_setup(target=other, OWNER_DIR=str(self.target.resolve()))
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((other / '.env').exists())
        self.assertFalse(any('up' in a for a in self.calls()[before:]))

    def test_concurrent_different_directories_cannot_rotate_the_same_container(self):
        barrier = self.root / 'barrier'
        state = self.root / 'owner'
        env = dict(os.environ, PATH=str(self.bin)+':'+os.environ['PATH'], CALL_LOG=str(self.log),
                   STATE_FILE=str(state), BLOCK_UP=str(barrier))
        first = subprocess.Popen(['bash', str(SCRIPT), str(self.target)], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        try:
            deadline = time.monotonic()+5
            while not Path(str(barrier)+'.ready').exists() and first.poll() is None and time.monotonic()<deadline:
                time.sleep(.01)
            self.assertTrue(Path(str(barrier)+'.ready').exists(), 'first installer must reach the held up stage')
            second = self.run_setup(target=self.root/'parallel', STATE_FILE=str(state))
            self.assertNotEqual(second.returncode, 0)
        finally:
            Path(str(barrier)+'.release').touch()
            stdout, stderr = first.communicate(timeout=10)
        self.assertEqual(first.returncode, 0, stdout+stderr)
        self.assertEqual(state.read_text(), str(self.target.resolve()))

    def test_unknown_directory_is_not_overwritten(self):
        self.target.mkdir()
        existing = self.target / '.env'
        existing.write_text('existing-private-config\n')
        self.assertNotEqual(self.run_setup().returncode, 0)
        self.assertEqual(existing.read_text(), 'existing-private-config\n')
        self.assertFalse(any('up' in a for a in self.calls()))

    def test_missing_network_or_unknown_container_never_starts_service(self):
        for flag in ['FAIL_NETWORK', 'UNKNOWN_CONTAINER']:
            with self.subTest(flag=flag):
                self.assertNotEqual(self.run_setup(**{flag:'1'}).returncode, 0)
                self.assertFalse(any('up' in a for a in self.calls()))

    def test_start_or_ping_failure_is_not_reported_as_success(self):
        for flag in ['FAIL_UP', 'FAIL_PING', 'FAIL_PORT']:
            with self.subTest(flag=flag):
                result = self.run_setup(**{flag:'1'})
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn('Redis ready', result.stdout)

    def test_unexpected_public_port_stops_only_the_managed_redis(self):
        result = self.run_setup(PUBLIC_PORT='1')
        self.assertNotEqual(result.returncode, 0)
        stops = [a for a in self.calls() if 'stop' in a]
        self.assertEqual(len(stops), 1)
        self.assertEqual(stops[0][-1], 'redis')

    def test_changed_installed_config_is_not_silently_replaced(self):
        self.assertEqual(self.run_setup().returncode, 0)
        p = self.target / 'compose.yaml'
        p.write_text('locally-reviewed-config\n')
        self.assertNotEqual(self.run_setup().returncode, 0)
        self.assertEqual(p.read_text(), 'locally-reviewed-config\n')


if __name__ == '__main__':
    unittest.main()
