import importlib.util
import contextlib
import io
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[1] / 'profile-cost-home.py'


class HomepageProfileTests(unittest.TestCase):
    def setUp(self):
        spec = importlib.util.spec_from_file_location('homepage_profile', SCRIPT)
        self.profile = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.profile)

    def test_aggregates_duration_units_and_nearest_rank_percentiles(self):
        rows = [f'level=INFO msg=request method=GET path=/api/v1/stats duration={d}'
                for d in ['500µs', '2ms', '10ms', '1.5s', '1m2s']]
        report = self.profile.report('\n'.join(rows))
        row = report[0]
        self.assertEqual(row['method'], 'GET')
        self.assertEqual(row['path'], '/api/v1/stats')
        self.assertEqual(row['count'], 5)
        self.assertEqual(row['p50_ms'], 10)
        self.assertEqual(row['p95_ms'], 62000)
        self.assertEqual(row['max_ms'], 62000)

    def test_normalizes_entry_ids_and_discards_every_unrelated_log_field(self):
        raw = '\n'.join([
            'msg=request method=PATCH path=/api/v1/entries/987?token=private-query duration=25ms openid=private-user',
            'msg=request method=GET path=/api/v1/auth/me duration=3ms token=private-token',
            'msg=error method=GET path=/api/v1/stats duration=1s error=private-secret',
            'msg=request method=GET path=/api/v1/avatars/private-avatar.jpg duration=5ms',
            'msg=request method=GET path=/health duration=2ms',
            'msg=request method=GET path=/api/v1/stats duration=invalid',
        ])
        result = self.profile.report(raw)
        self.assertEqual(len(result), 2)
        self.assertEqual(result[1]['path'], '/api/v1/entries/{id}')
        self.assertNotIn('private', str(result))
        self.assertNotIn('987', str(result))

    def test_empty_or_only_health_logs_have_no_samples(self):
        self.assertEqual(self.profile.report(''), [])
        self.assertEqual(self.profile.report('msg=request method=GET path=/health duration=1ms'), [])

    def test_cli_reads_stderr_logs_but_prints_only_safe_aggregates(self):
        proc = subprocess.CompletedProcess([], 0, '', 'msg=request method=GET path=/api/v1/stats duration=45ms token=private-token\n')
        output = io.StringIO()
        with patch.object(self.profile.subprocess, 'run', return_value=proc) as run, contextlib.redirect_stdout(output):
            self.assertEqual(self.profile.main(['--since', '2m']), 0)
        self.assertIn('GET /api/v1/stats 1 45.000 45.000 45.000', output.getvalue())
        self.assertNotIn('private-token', output.getvalue())
        self.assertEqual(run.call_args.args[0], ['docker', 'logs', '--since', '2m', '--tail', '10000', 'cost-cloud-api-1'])

    def test_cli_docker_failure_does_not_expose_error_log(self):
        proc = subprocess.CompletedProcess([], 1, 'private-stdout', 'private-secret')
        output = io.StringIO()
        with patch.object(self.profile.subprocess, 'run', return_value=proc), contextlib.redirect_stdout(output), contextlib.redirect_stderr(output):
            self.assertEqual(self.profile.main([]), 1)
        self.assertNotIn('private', output.getvalue())

    def test_cli_rejects_invalid_container_before_calling_docker(self):
        with patch.object(self.profile.subprocess, 'run') as run, contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit):
                self.profile.main(['--container', '/bad-container'])
        run.assert_not_called()


if __name__ == '__main__':
    unittest.main()
