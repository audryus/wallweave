"""The command layer of WallWeave: it receives JSON requests from the
frontend, runs the right handler, and returns a JSON response. Handlers
have access to the SQLite database."""

import json
from dataclasses import asdict, dataclass, field


def _int(value):
    """Reads a JSON number as an int, like Go's json.Unmarshal into int:
    missing → 0, integral floats are fine, anything else is invalid."""
    if value is None:
        return 0
    if isinstance(value, bool):
        raise ValueError("expected number, got bool")
    if isinstance(value, int):
        return value
    if isinstance(value, float) and value.is_integer():
        return int(value)
    raise ValueError(f"expected integer, got {value!r}")


def _str(value):
    """Reads a JSON string: missing → "", anything else is invalid."""
    if value is None:
        return ""
    if not isinstance(value, str):
        raise ValueError(f"expected string, got {value!r}")
    return value


def _bool(value):
    """Reads a JSON bool: missing → False, anything else is invalid."""
    if value is None:
        return False
    if not isinstance(value, bool):
        raise ValueError(f"expected bool, got {value!r}")
    return value


@dataclass
class Display:
    """One monitor of the system, with its wallpaper settings: which library
    (theme) is selected, the rotation timer, whether videos are allowed, a
    random seed for shuffling, and the resolution."""

    id: int = 0
    name: str = ""
    mirrorOf: str = ""
    video: bool = False
    timer: int = 0
    seed: int = 0
    width: int = 0
    height: int = 0
    theme: int = 0

    @classmethod
    def from_json(cls, obj):
        """Builds a Display from a decoded JSON object. Unknown keys are
        ignored and missing ones get their zero value (like Go). Raises
        ValueError when a field has the wrong type."""
        if obj is None:
            return cls()
        if not isinstance(obj, dict):
            raise ValueError("display must be an object")
        return cls(
            id=_int(obj.get("id")),
            name=_str(obj.get("name")),
            mirrorOf=_str(obj.get("mirrorOf")),
            video=_bool(obj.get("video")),
            timer=_int(obj.get("timer")),
            seed=_int(obj.get("seed")),
            width=_int(obj.get("width")),
            height=_int(obj.get("height")),
            theme=_int(obj.get("theme")),
        )


@dataclass
class Request:
    """Any command that arrives from the frontend. Only the fields relevant
    to the command are filled in."""

    cmd: str = ""
    id: int = 0
    path: str = ""
    display: Display = field(default_factory=Display)

    @classmethod
    def from_json(cls, obj):
        """Builds a Request from a decoded JSON object (raises ValueError
        when the shape is wrong)."""
        if not isinstance(obj, dict):
            raise ValueError("request must be an object")
        return cls(
            cmd=_str(obj.get("cmd")),
            id=_int(obj.get("id")),
            path=_str(obj.get("path")),
            display=Display.from_json(obj.get("display")),
        )


@dataclass
class Response:
    """The generic answer envelope sent back to the frontend. type says
    which command answered; message carries the payload (often JSON) or the
    technical error text. code is a stable, translatable error key (e.g.
    "folder_exists") used only when type is "error"; the UI maps it through
    I18n and falls back to message for unknown/technical errors."""

    type: str
    code: str = ""
    message: str = ""

    def to_json(self):
        """Serializes the response, omitting empty code/message like Go's
        omitempty."""
        out = {"type": self.type}
        if self.code:
            out["code"] = self.code
        if self.message:
            out["message"] = self.message
        return json.dumps(out, ensure_ascii=False, separators=(",", ":"))


def error_response(code, message):
    """Builds a type "error" response with a stable code for the UI and a
    technical message for logs/console."""
    return Response(type="error", code=code, message=message)


def technical_error(err):
    """Builds a type "error" response from a raw exception. No code: the
    message is developer-facing and is not translated."""
    return Response(type="error", message=str(err))


def payload(type_, value):
    """Builds a success response whose message is the JSON of value
    (dataclasses are converted to dicts)."""
    if isinstance(value, list):
        value = [asdict(v) if hasattr(v, "__dataclass_fields__") else v for v in value]
    elif hasattr(value, "__dataclass_fields__"):
        value = asdict(value)
    return Response(type=type_, message=json.dumps(value, ensure_ascii=False, separators=(",", ":")))


class Commander:
    """Registers and runs commands, and keeps a reference to the database
    that every handler can use."""

    def __init__(self, database):
        # Imported here: the handler modules import this one for the types.
        from . import display, library, status

        self.database = database
        self.commands = {}
        # Register each command group.
        status.register(self)
        library.register(self)
        display.register(self)

    def register(self, name, fn):
        """Maps a command name (for example "add_library") to the function
        that runs when that command arrives. fn takes (commander, request)."""
        self.commands[name] = fn

    def exec(self, req):
        """Looks up the command name in the request and runs its handler.
        An unknown name returns an error response with a stable code the UI
        can translate (technical message stays for logs). An exception in a
        handler becomes a technical error instead of killing the process."""
        fn = self.commands.get(req.cmd)
        if fn is None:
            return error_response("unknown_command", "unknown command: " + req.cmd)
        try:
            return fn(self, req)
        except Exception as e:  # noqa: BLE001 — same as Go's TechnicalError(err)
            return technical_error(e)

    def start_workers(self):
        """Starts the wallpaper workers when the application boots (see
        worker.start_workers)."""
        from . import worker

        worker.start_workers(self)
