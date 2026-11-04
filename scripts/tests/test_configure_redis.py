import pathlib
import subprocess
import tempfile
import unittest

ROOT=pathlib.Path(__file__).resolve().parents[2]
class ConfigureRedisTest(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
        self.root=pathlib.Path(self.tmp.name)
        self.redis=self.root/'redis.env';self.api=self.root/'api.env'
        self.secret='a'*64
        self.redis.write_text('COST_REDIS_PASSWORD='+self.secret+'\nCOST_REDIS_IMAGE=redis:8.4.8-alpine\nCOST_REDIS_NETWORK=cost-cloud_default\n')
        self.original='COST_MYSQL_PASSWORD=test-only-db-secret\n# preserved comment\nWECHAT_APP_ID=wx-test\n'
        self.api.write_text(self.original)
    def run_config(self):
        return subprocess.run(['python3',str(ROOT/'scripts/configure-cost-redis.py'),'--redis-env',str(self.redis),'--api-env',str(self.api)],text=True,capture_output=True)
    def test_enables_cache_preserving_environment_without_printing_secrets(self):
        for _ in range(2):
            r=self.run_config();self.assertEqual(r.returncode,0,r.stderr)
            result=self.api.read_text()
            self.assertTrue(result.startswith(self.original))
            self.assertIn('COST_REDIS_ADDR=cost-redis:6379\n',result)
            self.assertIn('COST_REDIS_PASSWORD='+self.secret+'\n',result)
            self.assertEqual(result.count('COST_REDIS_ADDR='),1)
            self.assertEqual(self.api.stat().st_mode&0o777,0o600)
            self.assertNotIn(self.secret,r.stdout+r.stderr)
            self.assertNotIn('test-only-db-secret',r.stdout+r.stderr)
    def test_bad_or_duplicate_password_leaves_api_environment_unchanged(self):
        for text in ['COST_REDIS_PASSWORD=bad\n','COST_REDIS_PASSWORD='+self.secret+'\nCOST_REDIS_PASSWORD='+self.secret+'\n']:
            self.redis.write_text(text);r=self.run_config()
            self.assertNotEqual(r.returncode,0)
            self.assertEqual(self.api.read_text(),self.original)
            self.assertNotIn(self.secret,r.stdout+r.stderr)
    def test_refuses_symlinks_and_missing_api_env(self):
        source=self.root/'linked';source.symlink_to(self.redis);self.redis=source
        self.assertNotEqual(self.run_config().returncode,0)
        self.redis=self.root/'redis.env';self.api.unlink()
        self.assertNotEqual(self.run_config().returncode,0)
        self.assertFalse(self.api.exists())
    def test_duplicate_api_settings_are_not_silently_rewritten(self):
        self.api.write_text(self.original+'COST_REDIS_ADDR=a:6379\nCOST_REDIS_ADDR=b:6379\n')
        before=self.api.read_bytes();self.assertNotEqual(self.run_config().returncode,0);self.assertEqual(self.api.read_bytes(),before)
    def test_export_and_whitespace_settings_are_replaced(self):
        self.api.write_text(self.original+' export COST_REDIS_ADDR = old:6379\n COST_REDIS_PASSWORD = "old-password"\n')
        result=self.run_config();self.assertEqual(result.returncode,0,result.stderr)
        text=self.api.read_text();self.assertNotIn('old:6379',text);self.assertNotIn('old-password',text)
        self.assertEqual(text.count('COST_REDIS_ADDR'),1)
    def test_mixed_export_duplicate_settings_are_rejected(self):
        self.api.write_text(self.original+'export COST_REDIS_ADDR = a:6379\nCOST_REDIS_ADDR=b:6379\n')
        before=self.api.read_bytes();self.assertNotEqual(self.run_config().returncode,0);self.assertEqual(self.api.read_bytes(),before)
    def test_multiline_values_are_rejected_without_corrupting_credentials(self):
        self.api.write_text(self.original+"WECHAT_APP_SECRET='first-line\nCOST_REDIS_ADDR=part-of-secret\nlast-line'\n")
        before=self.api.read_bytes();result=self.run_config();self.assertNotEqual(result.returncode,0);self.assertEqual(self.api.read_bytes(),before)
        self.assertNotIn('part-of-secret',result.stdout+result.stderr)
