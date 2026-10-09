import json
import os
import queue
import socket
import tempfile
import threading
import unittest

from backend.mpv import fit_option, mpv_command, mpv_loadfile


def fake_mpv(test, reply):
    """Listens on a unix socket and answers each request like mpv does:
    first an unrelated event line, then the reply for the request_id. Every
    received command is put on the returned queue."""
    tmp = tempfile.TemporaryDirectory()
    test.addCleanup(tmp.cleanup)
    sock = os.path.join(tmp.name, "mpv.sock")
    ln = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    ln.bind(sock)
    ln.listen()
    test.addCleanup(ln.close)

    got = queue.Queue()

    def serve():
        while True:
            try:
                conn, _ = ln.accept()
            except OSError:
                return
            with conn:
                line = conn.makefile("rb").readline()
                got.put(json.loads(line))
                conn.sendall(b'{"event":"start-file","playlist_entry_id":2}\n')
                conn.sendall(reply.encode() + b"\n")

    threading.Thread(target=serve, daemon=True).start()
    return sock, got


class TestMpv(unittest.TestCase):
    def test_loadfile(self):
        """loadfile sends the named-argument command with the fit option
        (skipping event lines), then unpauses mpv."""
        sock, got = fake_mpv(self, '{"data":{"playlist_entry_id":2},"request_id":1,"error":"success"}')
        mpv_loadfile(sock, "/walls/clip.MP4")
        cmd = got.get(timeout=2)["command"]
        self.assertEqual(cmd, {"name": "loadfile", "url": "/walls/clip.MP4", "flags": "replace", "options": "panscan=0"})
        self.assertEqual(got.get(timeout=2)["command"], ["set_property", "pause", False])

    def test_command_error(self):
        """an mpv error reply becomes an exception."""
        sock, _ = fake_mpv(self, '{"request_id":1,"error":"property unavailable"}')
        with self.assertRaises(RuntimeError):
            mpv_command(sock, ["get_property", "pid"])

    def test_command_no_socket(self):
        """a missing socket fails fast (so mpv_show starts mpvpaper)."""
        with tempfile.TemporaryDirectory() as d, self.assertRaises(OSError):
            mpv_command(os.path.join(d, "none.sock"), ["get_property", "pid"])

    def test_fit_option(self):
        """images cover the screen, videos keep their whole frame."""
        cases = {"/a/b.jpg": "panscan=1.0", "/a/b.PNG": "panscan=1.0", "/a/b.webm": "panscan=0", "/a/b.mkv": "panscan=0"}
        for path, want in cases.items():
            self.assertEqual(fit_option(path), want, path)


if __name__ == "__main__":
    unittest.main()
