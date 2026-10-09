"""add/del/browse libraries (wallpaper folders) and ffmpeg thumbnails."""

import json
import os
import subprocess
from dataclasses import dataclass, field

from .commander import error_response, payload, Response

# IMAGE_EXTS lists the file extensions treated as images (lowercase).
IMAGE_EXTS = {".jpg", ".jpeg", ".png", ".gif", ".webp"}

# VIDEO_EXTS lists the file extensions treated as videos (lowercase).
VIDEO_EXTS = {".mp4", ".avi", ".mov", ".mkv", ".webm"}


@dataclass
class LibraryCount:
    """How many images and videos a library contains."""

    images: int = 0
    videos: int = 0


@dataclass
class Library:
    """One wallpaper folder that the user added. thumbs holds the paths of
    the generated thumbnail images, or None when there are none (sent as
    JSON null, as the UI already expects)."""

    id: int = 0
    path: str = ""
    thumbs: list = None
    count: LibraryCount = field(default_factory=LibraryCount)


def file_ext(p):
    """Returns the extension of a path like Go's filepath.Ext: everything
    from the last "." of the file name (".jpg" for a file named ".jpg"),
    or "" when there is none."""
    name = os.path.basename(p)
    i = name.rfind(".")
    return name[i:] if i >= 0 else ""


def register(c):
    """Registers the commands used to manage wallpaper folders."""
    c.register("add_library", add_library)
    c.register("del_library", del_library)
    c.register("browse_libraries", browse_libraries)


def add_library(c, req):
    """Handler for "add_library". Steps:
     1. Reject an empty path.
     2. Check the folder was not already added.
     3. Scan the folder: count images/videos and generate thumbnails.
     4. Insert the new library row in the database.
     5. Return the created library as JSON."""
    # Step 1: the path is required.
    if req.path == "":
        return error_response("invalid_path", "library path is empty")

    # Step 2: reject a folder that is already in the list.
    if library_exists(c, req.path):
        return error_response("folder_exists", "folder already in library")

    # Step 3: walk the folder, count files, and create thumbnails.
    library = generate_library(req.path)

    # Step 4: save the library in the database.
    insert_library(c, library)

    # Step 5: answer with the library.
    return payload("add_library", library)


def generate_library(path):
    """Scans a folder and builds a Library. Steps:
     1. Check the path exists and is a directory.
     2. Create a "thumbs" folder inside it if it does not exist.
     3. Walk every file: count images and videos (skipping the thumbs
        folder) and generate a thumbnail for each media file with ffmpeg.
     4. Store the image/video counts.
     5. Collect the paths of all generated thumbnail files."""
    library = Library(path=path)

    # Step 1: the path must exist and must be a directory (raises when it
    # does not exist, like os.Stat in Go).
    os.stat(path)
    if not os.path.isdir(path):
        raise NotADirectoryError(f"invalid argument: {path}")

    # Step 2: create the thumbs directory if needed.
    thumbs_dir = os.path.join(path, "thumbs")
    if not os.path.exists(thumbs_dir):
        os.mkdir(thumbs_dir, 0o755)

    # Step 3: walk the folder in lexical order (like Go's filepath.Walk),
    # failing on unreadable entries. The thumbs folder is not entered so
    # generated thumbnails are not counted.
    def fail(err):
        raise err

    images = videos = 0
    for dirpath, dirnames, filenames in os.walk(path, onerror=fail):
        dirnames.sort()
        if dirpath == path and "thumbs" in dirnames:
            dirnames.remove("thumbs")
        for name in sorted(filenames):
            p = os.path.join(dirpath, name)
            ext = file_ext(name).lower()
            # Thumbnail errors are ignored so one bad file does not abort
            # the whole walk.
            if ext in IMAGE_EXTS:
                images += 1
                _try(generate_thumb, p, thumbs_dir)
            elif ext in VIDEO_EXTS:
                videos += 1
                _try(generate_video_thumb, p, thumbs_dir)

    # Step 4: store the counts.
    library.count = LibraryCount(images=images, videos=videos)

    # Step 5: collect the generated thumbnails (.jpg files in thumbs/).
    try:
        entries = sorted(os.scandir(thumbs_dir), key=lambda e: e.name)
    except OSError:
        entries = []
    thumbs = [
        os.path.join(thumbs_dir, e.name)
        for e in entries
        if not e.is_dir() and e.name.lower().endswith(".jpg")
    ]
    library.thumbs = thumbs or None
    return library


def _try(fn, *args):
    """Calls fn and swallows its error."""
    try:
        fn(*args)
    except Exception:  # noqa: BLE001
        pass


def _thumb_path(file_path, thumbs_dir):
    """Builds the thumbnail name without a double extension
    (video.mp4 -> video.jpg, not video.mp4.jpg)."""
    base = os.path.basename(file_path)
    ext = file_ext(base)
    if ext:
        base = base[: -len(ext)]
    return os.path.join(thumbs_dir, base + ".jpg")


def _ffmpeg(args, label):
    """Runs ffmpeg and raises with its own stderr on failure. stdin is
    /dev/null (plus -nostdin): ffmpeg reads stdin interactively and would
    otherwise eat the JSON commands the UI writes to this process."""
    proc = subprocess.run(
        ["ffmpeg", "-nostdin", *args],
        stdin=subprocess.DEVNULL,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.PIPE,
        text=True,
        errors="replace",
    )
    if proc.returncode != 0:
        raise RuntimeError(f"{label}: exit status {proc.returncode}: {proc.stderr}")


def generate_video_thumb(file_path, thumbs_dir):
    """Creates a thumbnail for a video: ffmpeg overwrites the output, reads
    the input directly (seekable, avoids broken pipes on mp4 files with moov
    at the end), picks a representative frame and scales it to 320x240."""
    save_path = _thumb_path(file_path, thumbs_dir)
    _ffmpeg(
        ["-y", "-i", file_path, "-vframes", "1", "-vf", "thumbnail,scale=320:240:flags=lanczos", "-q:v", "2", save_path],
        "ffmpeg " + os.path.basename(file_path),
    )


def generate_thumb(file_path, thumbs_dir):
    """Creates a thumbnail for an image (known image types only): ffmpeg
    decodes the first frame and scales it to 320x240."""
    if file_ext(file_path).lower() not in IMAGE_EXTS:
        return
    save_path = _thumb_path(file_path, thumbs_dir)
    _ffmpeg(
        ["-y", "-i", file_path, "-vframes", "1", "-vf", "scale=320:240:flags=lanczos", "-q:v", "2", save_path],
        "ffmpeg thumb " + os.path.basename(file_path),
    )


def del_library(c, req):
    """Handler for "del_library". Steps:
     1. Require an id or a path (at least one of them).
     2. Delete the row (by id if given, otherwise by path).
     3. Wake all wallpaper workers: a display that pointed to this library
        will fall back to its defensive path (restore default wallpaper)."""
    from . import worker

    # Step 1: both fields empty means the request is invalid.
    if req.id == 0 and req.path == "":
        return error_response("invalid_id_path", "library id/path missing")

    # Step 2: delete by id when an id was sent, otherwise by path.
    if req.id != 0:
        c.database.execute("DELETE FROM libraries WHERE id = ?", (req.id,))
    else:
        c.database.execute("DELETE FROM libraries WHERE path = ?", (req.path,))

    # Step 3: wake the workers so they notice the library is gone.
    worker.signal_all_workers()
    return Response(type="del_library")


def browse_libraries(c, req):
    """Handler for "browse_libraries": every library as a JSON array."""
    return payload("browse_libraries", fetch_libraries(c))


def fetch_libraries(c):
    """Reads every library row, ordered by id. The thumbs column stores a
    JSON string, which is decoded here."""
    rows = c.database.execute(
        "SELECT id, path, thumbs, images, videos FROM libraries ORDER BY id"
    ).fetchall()
    libraries = []
    for id_, path, thumbs, images, videos in rows:
        libraries.append(
            Library(
                id=id_,
                path=path,
                thumbs=json.loads(thumbs) if thumbs else None,
                count=LibraryCount(images=images, videos=videos),
            )
        )
    return libraries


def library_exists(c, path):
    """Reports whether a library with the given path is already saved."""
    return c.database.execute("SELECT 1 FROM libraries WHERE path = ?", (path,)).fetchone() is not None


def insert_library(c, lib):
    """Saves a new library row and fills in lib.id from the auto-increment
    primary key. The thumbs list is stored as a JSON string."""
    cur = c.database.execute(
        "INSERT INTO libraries (path, thumbs, images, videos) VALUES (?, ?, ?, ?)",
        (lib.path, json.dumps(lib.thumbs, ensure_ascii=False), lib.count.images, lib.count.videos),
    )
    lib.id = cur.lastrowid
