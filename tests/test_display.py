import json
import unittest

from backend import display
from backend.commander import Display, Request
from backend.status import load_status
from tests.helpers import CommanderTestCase


class TestGetMonitors(unittest.TestCase):
    def test_get_monitors(self):
        """hyprctl can list the connected monitors (skipped without it)."""
        try:
            displays = display.get_monitors()
        except Exception as e:  # noqa: BLE001
            self.skipTest(f"hyprctl unavailable: {e}")
        self.assertGreater(len(displays), 0, "expected at least 1 monitor")
        for d in displays:
            self.assertNotEqual(d.name, "", "expected monitor name")


class TestDisplayCommands(CommanderTestCase):
    def test_browse_displays(self):
        """browse_displays returns the single seeded display."""
        res = self.c.exec(Request(cmd="browse_displays"))
        self.assertEqual(res.type, "browse_displays", res.message)
        displays = json.loads(res.message)
        self.assertEqual(len(displays), 1)
        self.assertEqual(displays[0]["name"], "TEST-1")

    def test_edit_display(self):
        """edit_display returns AND persists the updated fields."""
        res = self.c.exec(Request(cmd="edit_display", display=Display(name="TEST-1", theme=1, video=True, timer=5)))
        self.assertEqual(res.type, "edit_display", res.message)
        updated = json.loads(res.message)
        self.assertEqual((updated["theme"], updated["video"], updated["timer"]), (1, True, 5))

        stored = display.get_display(self.c, "TEST-1")
        self.assertIsNotNone(stored)
        self.assertEqual((stored.theme, stored.video, stored.timer), (1, True, 5))

    def test_edit_display_empty_name(self):
        """editing a display without a name fails with a stable code."""
        res = self.c.exec(Request(cmd="edit_display", display=Display(theme=1)))
        self.assertEqual(res.type, "error")
        self.assertEqual(res.code, "display_empty")

    def test_unknown_command(self):
        """an unknown command name fails with a stable code."""
        res = self.c.exec(Request(cmd="nope"))
        self.assertEqual(res.type, "error")
        self.assertEqual(res.code, "unknown_command")

    def test_status_persists(self):
        """get_status probes the system and saves a label code and color."""
        res = self.c.exec(Request(cmd="get_status"))
        self.assertEqual(res.type, "get_status", res.message)
        stored = load_status(self.c)
        self.assertNotEqual(stored.color, "")
        self.assertIn(stored.label, ("running", "degraded"))


class TestRequestParsing(unittest.TestCase):
    def test_from_json(self):
        """the UI's request shape parses, with zero values for missing keys."""
        req = Request.from_json({"cmd": "edit_display", "display": {"name": "DP-1", "timer": 120.0, "video": True}})
        self.assertEqual(req.display, Display(name="DP-1", timer=120, video=True))
        self.assertEqual(Request.from_json({"cmd": "browse_displays"}).display, Display())

    def test_from_json_wrong_types(self):
        """wrong field types are rejected, like Go's json.Unmarshal."""
        for bad in ({"cmd": 1}, {"cmd": "x", "id": "1"}, {"cmd": "x", "display": {"video": 1}}, []):
            with self.assertRaises(ValueError):
                Request.from_json(bad)


if __name__ == "__main__":
    unittest.main()
