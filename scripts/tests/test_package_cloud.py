"""The cloud archive contains all runtime files without data or secrets."""
import importlib.util
from pathlib import Path
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('packager', ROOT / 'scripts/package-cloud.py')
packager = importlib.util.module_from_spec(spec)
spec.loader.exec_module(packager)

class PackageTests(unittest.TestCase):
    def test_backup_workers_ship_in_cloud_release(self):
        with tempfile.TemporaryDirectory() as directory:
            archive = Path(directory) / 'release.tar.gz'
            packager.build_archive(ROOT, archive)
            with tarfile.open(archive) as tf:
                names = set(tf.getnames())
            self.assertTrue({'deploy/backup/common.sh', 'deploy/backup/backup.sh',
                             'deploy/backup/verify.sh'} <= names,
                            'compose backup services need their scripts in the release')
            self.assertFalse(any(n.endswith('.sql.gz') or n.endswith('/.env') for n in names))

if __name__ == '__main__':
    unittest.main()
