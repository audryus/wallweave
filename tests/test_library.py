import json
import os
import unittest

from backend.commander import Display, Request
from backend.library import generate_library
from backend.worker import list_wallpaper_files
from tests.helpers import CommanderTestCase


class TestLibrary(CommanderTestCase):
    def test_generate_library_empty(self):
        """an empty folder keeps its path and reports 0 images/0 videos."""
        d = self.tempdir()
        lib = generate_library(d)
        self.assertEqual(lib.path, d)
        self.assertEqual((lib.count.images, lib.count.videos), (0, 0))
        self.assertIsNone(lib.thumbs)

    def test_add_and_browse_libraries_ids(self):
        """the whole lifecycle: add two, reject a duplicate, list, delete by
        id, delete by path."""
        dir1, dir2 = self.tempdir(), self.tempdir()

        res = self.c.exec(Request(cmd="add_library", path=dir1))
        self.assertEqual(res.type, "add_library", res.message)
        self.assertEqual(json.loads(res.message)["id"], 1)

        res = self.c.exec(Request(cmd="add_library", path=dir2))
        self.assertEqual(res.type, "add_library", res.message)
        self.assertEqual(json.loads(res.message)["id"], 2)

        res = self.c.exec(Request(cmd="add_library", path=dir1))
        self.assertEqual(res.type, "error")
        self.assertEqual(res.code, "folder_exists")

        res = self.c.exec(Request(cmd="browse_libraries"))
        self.assertEqual(res.type, "browse_libraries", res.message)
        libs = json.loads(res.message)
        self.assertEqual([lib["id"] for lib in libs], [1, 2])
        # Same JSON shape the UI already reads.
        self.assertEqual(libs[0]["count"], {"images": 0, "videos": 0})
        self.assertIsNone(libs[0]["thumbs"])

        res = self.c.exec(Request(cmd="del_library", id=1))
        self.assertEqual(res.type, "del_library", res.message)
        libs = json.loads(self.c.exec(Request(cmd="browse_libraries")).message)
        self.assertEqual([lib["id"] for lib in libs], [2])

        res = self.c.exec(Request(cmd="del_library", path=dir2))
        self.assertEqual(res.type, "del_library", res.message)
        self.assertEqual(json.loads(self.c.exec(Request(cmd="browse_libraries")).message), [])

    def test_invalid_path(self):
        """empty add path and delete without id/path fail with stable codes."""
        res = self.c.exec(Request(cmd="add_library", path=""))
        self.assertEqual((res.type, res.code), ("error", "invalid_path"))
        res = self.c.exec(Request(cmd="del_library"))
        self.assertEqual((res.type, res.code), ("error", "invalid_id_path"))

    def test_add_missing_folder(self):
        """a folder that does not exist is a technical error (no code)."""
        res = self.c.exec(Request(cmd="add_library", path="/nonexistent/wallweave"))
        self.assertEqual((res.type, res.code), ("error", ""))
        self.assertNotEqual(res.message, "")


class TestListWallpaperFiles(CommanderTestCase):
    def test_list_wallpaper_files(self):
        """images in subfolders are found, thumbs/ is skipped, unknown
        extensions are ignored, videos only when the display allows them."""
        root = self.tempdir()
        for f in [
            "a.jpg", "b.PNG", "clip.mp4", "notes.txt",
            "sub/c.webp", "sub/deeper/clip2.webm",
            "thumbs/a.jpg", "thumbs/clip.jpg",
        ]:
            p = os.path.join(root, f)
            os.makedirs(os.path.dirname(p), exist_ok=True)
            open(p, "w").close()

        def rel(files):
            return sorted(os.path.relpath(f, root) for f in files)

        self.assertEqual(rel(list_wallpaper_files(Display(seed=1), root)), ["a.jpg", "b.PNG", "sub/c.webp"])
        self.assertEqual(
            rel(list_wallpaper_files(Display(seed=1, video=True), root)),
            ["a.jpg", "b.PNG", "clip.mp4", "sub/c.webp", "sub/deeper/clip2.webm"],
        )
        # Same seed → same order; the order is what the rotation relies on.
        self.assertEqual(list_wallpaper_files(Display(seed=7), root), list_wallpaper_files(Display(seed=7), root))
        self.assertIsNone(list_wallpaper_files(Display(), ""))


if __name__ == "__main__":
    unittest.main()
