"""Shared test helpers."""

import os
import tempfile
import unittest

from backend.commander import Commander
from backend.db import open_database


class CommanderTestCase(unittest.TestCase):
    """Creates a Commander backed by a temporary database with one seeded
    display ("TEST-1"), so handlers can be exercised without a real
    hyprctl."""

    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.database = open_database(os.path.join(tmp.name, "test.db"))
        self.addCleanup(self.database.close)
        self.c = Commander(self.database)
        # Fixed seed — avoids depending on hyprctl in the handler tests.
        self.database.execute(
            """INSERT INTO displays (name, mirror_of, video, timer, seed, width, height, theme)
			 VALUES ('TEST-1', '', 0, 60, 1, 1920, 1080, 0)"""
        )

    def tempdir(self):
        """Returns a fresh temporary folder, removed after the test."""
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        return tmp.name
