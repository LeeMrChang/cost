"""Backup lifecycle tests; only the external MySQL processes are substituted."""
import gzip
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[2]
BACKUP = ROOT / 'deploy/backup/backup.sh'
VERIFY = ROOT / 'deploy/backup/verify.sh'
MYSQL_FAKE = r'''#!/usr/bin/env python3
import json, os, sys, time
from pathlib import Path
a = sys.argv[1:]
name = Path(sys.argv[0]).name
with open(os.environ['CALL_LOG'], 'a') as f: f.write(json.dumps([name] + a) + '\n')
if name == 'mysqldump':
    required = ['--single-transaction', '--no-tablespaces', '--set-gtid-purged=OFF', '--skip-triggers']
    if any(x not in a for x in required): sys.exit(9)
    if os.environ.get('DUMP_FAIL') == '1':
        print('secret-that-must-not-leak', file=sys.stderr)
        print('-- partial')
        sys.exit(1)
    if os.environ.get('DUMP_EMPTY') != '1': print('CREATE TABLE entries (id INT); INSERT INTO entries VALUES (42);')
elif name == 'mysqld':
    if '--initialize-insecure' in a: sys.exit(0)
    time.sleep(20)
elif name in ('mysql', 'mysqlcheck', 'mysqladmin'):
    if '--no-defaults' not in a or '--protocol=socket' not in a: sys.exit(9)
    if not any(x.startswith('--socket=') and '/cost-restore.' in x for x in a): sys.exit(9)
    if name == 'mysql' and any('t_schema_migrations' in x for x in a):
        if os.environ.get('REPORT_FAIL') == '1': sys.exit(1)
        if os.environ.get('MISSING_CATEGORY') == '1' and any('FROM t_categories' in x for x in a): sys.exit(1)
        print('schema_versions\t005_icost_realignment.sql\nentries_count\t1\nincome_count\t1\nincome_cent\t42\nexpense_count\t0\nexpense_cent\t0\naccounts_count\t2\ntransfers_count\t0')
    if name == 'mysql' and '-e' not in a:
        data = sys.stdin.read()
        if os.environ.get('RESTORE_FAIL') == '1': sys.exit(1)
        if 'VALUES (42)' not in data: sys.exit(8)
'''

class BackupTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.backups = self.root / 'backups'
        self.backups.mkdir()
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        self.log = self.root / 'calls.jsonl'
        for name in ('mysqldump', 'mysqld', 'mysql', 'mysqlcheck', 'mysqladmin'):
            path = self.bin / name
            path.write_text(MYSQL_FAKE)
            path.chmod(0o755)
        self.env = dict(os.environ, PATH=str(self.bin) + ':' + os.environ['PATH'],
                        CALL_LOG=str(self.log), COST_BACKUP_DIR=str(self.backups),
                        COST_RESTORE_WORK_DIR=str(self.root / 'verify'),
                        COST_MYSQL_PASSWORD='not-in-command-or-log',
                        COST_BACKUP_INTERVAL_SECONDS='86400', COST_BACKUP_RETENTION_DAYS='14',
                        COST_ISOLATED_RESTORE='yes')

    def run_script(self, script=BACKUP, command='once', **env):
        self.assertTrue(script.is_file(), 'backup/restore lifecycle implementation is missing')
        return subprocess.run(['bash', str(script), command], env=dict(self.env, **env),
                              capture_output=True, text=True, timeout=10)

    def seed(self, name, age=0):
        path = self.backups / name
        with gzip.open(path, 'wt') as f: f.write('CREATE TABLE entries (id INT); INSERT INTO entries VALUES (42);\n')
        timestamp = time.time() - age
        os.utime(path, (timestamp, timestamp))
        return path

    def test_success_publishes_private_complete_backup_and_healthy_state(self):
        result = self.run_script()
        self.assertEqual(result.returncode, 0, result.stderr)
        files = list(self.backups.glob('cost-*.sql.gz'))
        self.assertEqual(len(files), 1)
        with gzip.open(files[0], 'rt') as f: self.assertIn('VALUES (42)', f.read())
        self.assertEqual(files[0].stat().st_mode & 0o777, 0o600)
        self.assertEqual(self.backups.stat().st_mode & 0o777, 0o700)
        self.assertFalse(list(self.backups.glob('*.partial*')))
        self.assertEqual(self.run_script(command='health').returncode, 0)
        self.assertNotIn(self.env['COST_MYSQL_PASSWORD'], self.log.read_text())

    def test_failed_dump_preserves_expired_backup_and_reports_unhealthy(self):
        old = self.seed('cost-old.sql.gz', 30 * 86400)
        original = old.read_bytes()
        result = self.run_script(DUMP_FAIL='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(old.read_bytes(), original)
        self.assertEqual(list(self.backups.glob('cost-*.sql.gz')), [old])
        self.assertFalse(list(self.backups.glob('*.partial*')))
        self.assertIn('failed', (self.backups / 'backup.status').read_text())
        self.assertNotIn('secret-that-must-not-leak', result.stdout + result.stderr)
        self.assertNotEqual(self.run_script(command='health').returncode, 0)

    def test_retention_only_removes_expired_generated_backups_after_success(self):
        old = self.seed('cost-old.sql.gz', 15 * 86400)
        recent = self.seed('cost-recent.sql.gz', 12 * 86400)
        unrelated = self.seed('manual-important.sql.gz', 90 * 86400)
        result = self.run_script()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(old.exists())
        self.assertTrue(recent.exists())
        self.assertTrue(unrelated.exists())

    def test_empty_dump_never_becomes_successful_backup(self):
        result = self.run_script(DUMP_EMPTY='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(list(self.backups.glob('cost-*.sql.gz')))

    def test_bad_config_does_not_run_dump(self):
        result = self.run_script(COST_BACKUP_RETENTION_DAYS='0')
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.log.exists())

    def test_stale_health_is_unhealthy(self):
        self.backups.joinpath('backup.status').write_text(f'success {int(time.time())-100000} old\n')
        self.assertNotEqual(self.run_script(command='health').returncode, 0)

    def test_restore_guard_rejects_without_explicit_isolation(self):
        result = self.run_script(VERIFY, COST_ISOLATED_RESTORE='')
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.log.exists())

    def test_restore_loads_latest_backup_into_new_socket_database_and_cleans_up(self):
        self.seed('cost-example.sql.gz')
        result = self.run_script(VERIFY)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('success', (self.backups / 'verify.status').read_text())
        self.assertFalse(list((self.root / 'verify').glob('cost-restore.*')))
        self.assertEqual(self.run_script(VERIFY, command='health').returncode, 0)
        calls = [json.loads(s) for s in self.log.read_text().splitlines()]
        self.assertTrue(any(c[0] == 'mysqlcheck' for c in calls))
        self.assertNotIn(self.env['COST_MYSQL_PASSWORD'], self.log.read_text())

    def test_restore_writes_private_reconciliation_report(self):
        self.seed('cost-example.sql.gz')
        result = self.run_script(VERIFY)
        self.assertEqual(result.returncode, 0, result.stderr)
        report = self.backups / 'verify-report.tsv'
        self.assertTrue(report.exists(), 'restore must provide a reconciliation report')
        self.assertIn('income_cent\t42', report.read_text())
        self.assertIn('schema_versions\t005_icost_realignment.sql', report.read_text())
        self.assertEqual(report.stat().st_mode & 0o777, 0o600)
        self.assertNotIn('income_cent', result.stdout + result.stderr)

    def test_missing_core_tables_fails_verification_without_replacing_report(self):
        self.seed('cost-example.sql.gz')
        report = self.backups / 'verify-report.tsv'
        report.write_text('previous good report')
        result = self.run_script(VERIFY, REPORT_FAIL='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(report.read_text(), 'previous good report')

    def test_missing_category_table_is_not_a_successful_restore(self):
        self.seed('cost-example.sql.gz')
        result = self.run_script(VERIFY, MISSING_CATEGORY='1')
        self.assertNotEqual(result.returncode, 0)

    def test_restore_import_failure_preserves_backup_and_is_unhealthy(self):
        source = self.seed('cost-example.sql.gz')
        original = source.read_bytes()
        result = self.run_script(VERIFY, RESTORE_FAIL='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(source.read_bytes(), original)
        self.assertIn('failed', (self.backups / 'verify.status').read_text())
        self.assertFalse(list((self.root / 'verify').glob('cost-restore.*')))
        self.assertNotEqual(self.run_script(VERIFY, command='health').returncode, 0)

    def test_corrupt_backup_never_reaches_database(self):
        self.backups.joinpath('cost-bad.sql.gz').write_bytes(b'not a gzip')
        self.assertNotEqual(self.run_script(VERIFY).returncode, 0)
        self.assertFalse(self.log.exists())

if __name__ == '__main__':
    unittest.main()
