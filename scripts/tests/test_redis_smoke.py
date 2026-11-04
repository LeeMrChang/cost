"""Exercise Redis on disposable GitHub runners through existing test discovery."""
import os
from pathlib import Path
import subprocess
import unittest


@unittest.skipUnless(os.environ.get("GITHUB_ACTIONS") == "true", "requires disposable GitHub runner with Docker")
class RedisDockerSmokeTest(unittest.TestCase):
    def test_authenticated_cache_container(self):
        root = Path(__file__).resolve().parents[2]
        subprocess.run(["bash", "scripts/tests/redis-smoke.sh"], cwd=root, check=True, timeout=420)
