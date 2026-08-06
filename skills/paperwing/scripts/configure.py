#!/usr/bin/env python3
"""Securely save the connection settings used by the Paperwing skill."""

from __future__ import annotations

import getpass
import os
from pathlib import Path
import shlex
import tempfile
from urllib.parse import urlsplit


def config_path() -> Path:
    override = os.environ.get("PAPERWING_CONFIG_FILE")
    if override:
        return Path(override).expanduser()
    config_home = os.environ.get("XDG_CONFIG_HOME")
    root = Path(config_home).expanduser() if config_home else Path.home() / ".config"
    return root / "paperwing" / "config.env"


def read_config(path: Path) -> dict[str, str]:
    if not path.is_file():
        return {}
    values: dict[str, str] = {}
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, raw_value = line.split("=", 1)
        if key not in {"PAPERWING_URL", "PAPERWING_API_TOKEN"}:
            continue
        parsed = shlex.split(raw_value, posix=True)
        if len(parsed) == 1:
            values[key] = parsed[0]
    return values


def normalize_url(value: str) -> str:
    value = value.strip().rstrip("/")
    parsed = urlsplit(value)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        raise ValueError("Paperwing URL must be an absolute http:// or https:// URL")
    if parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ValueError("Paperwing URL must not contain credentials, a query, or a fragment")
    return value


def validate_token(value: str) -> str:
    value = value.strip()
    if not value.startswith("pw_") or len(value) < 32:
        raise ValueError("API token must be a Paperwing token beginning with pw_")
    return value


def write_config(path: Path, url: str, token: str) -> None:
    parent_existed = path.parent.exists()
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    if not parent_existed:
        os.chmod(path.parent, 0o700)
    descriptor, temporary_name = tempfile.mkstemp(prefix=".config.env.", dir=path.parent)
    temporary_path = Path(temporary_name)
    try:
        os.fchmod(descriptor, 0o600)
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            handle.write(f"PAPERWING_URL={shlex.quote(url)}\n")
            handle.write(f"PAPERWING_API_TOKEN={shlex.quote(token)}\n")
        os.replace(temporary_path, path)
        os.chmod(path, 0o600)
    except BaseException:
        temporary_path.unlink(missing_ok=True)
        raise


def main() -> None:
    path = config_path()
    existing = read_config(path)
    current_url = os.environ.get("PAPERWING_URL") or existing.get("PAPERWING_URL", "")
    current_token = os.environ.get("PAPERWING_API_TOKEN") or existing.get("PAPERWING_API_TOKEN", "")

    prompt = f"Paperwing URL [{current_url}]: " if current_url else "Paperwing URL: "
    entered_url = input(prompt).strip()
    url = normalize_url(entered_url or current_url)

    token_prompt = "API token (leave blank to keep the saved token): " if current_token else "API token: "
    entered_token = getpass.getpass(token_prompt)
    token = validate_token(entered_token or current_token)

    write_config(path, url, token)
    print(f"Saved Paperwing configuration to {path}")


if __name__ == "__main__":
    try:
        main()
    except (EOFError, KeyboardInterrupt):
        raise SystemExit("Configuration cancelled") from None
    except ValueError as error:
        raise SystemExit(str(error)) from None
