#!/usr/bin/env python3
"""Repository hygiene checks for duplicates and orphaned files."""
from __future__ import annotations

import hashlib
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ALLOWED_TOP_LEVEL_DIRS = {
    '.github',
    'cmd',
    'docs',
    'hack',
    'internal',
    'testcase',
}
ALLOWED_TOP_LEVEL_FILES = {
    '.env.example',
    '.gitignore',
    'AGENTS.md',
    'Dockerfile',
    'LICENSE',
    'Makefile',
    'README.md',
    'docker-compose.yml',
    'go.mod',
    'go.sum',
    'package-lock.json',
    'package.json',
}
IGNORE_PATH_PREFIXES = {
    '.git/',
    'docs/.vitepress/cache/',
    'docs/.vitepress/dist/',
    'dist/',
    'logs/',
}


def _git_files() -> list[Path]:
    try:
        output = subprocess.check_output(['git', 'ls-files'], cwd=ROOT, text=True)
    except subprocess.CalledProcessError as exc:  # pragma: no cover - critical failure
        print('Failed to list tracked files', file=sys.stderr)
        raise SystemExit(exc.returncode)
    return [Path(line.strip()) for line in output.splitlines() if line.strip()]


def _is_ignored(path: Path) -> bool:
    as_posix = path.as_posix()
    return any(as_posix.startswith(prefix) for prefix in IGNORE_PATH_PREFIXES)


def check_duplicates(paths: list[Path]) -> int:
    print('Checking for duplicate files...')
    hashes: dict[str, list[Path]] = {}
    for path in paths:
        if _is_ignored(path):
            continue
        # Skip empty files to reduce noise.
        full_path = ROOT / path
        if full_path.stat().st_size == 0:
            continue
        digest = hashlib.sha256(full_path.read_bytes()).hexdigest()
        hashes.setdefault(digest, []).append(path)
    duplicates = [group for group in hashes.values() if len(group) > 1]
    if duplicates:
        print('Found duplicate file contents:')
        for group in duplicates:
            for item in group:
                print(f'  - {item}')
            print('')
        return 1
    print('No duplicate files detected.')
    return 0


def check_orphans(paths: list[Path]) -> int:
    print('Checking for orphan files...')
    orphans: list[Path] = []
    for path in paths:
        if _is_ignored(path):
            continue
        parts = path.parts
        top = parts[0]
        if top in ALLOWED_TOP_LEVEL_DIRS:
            continue
        if path.name in ALLOWED_TOP_LEVEL_FILES and len(parts) == 1:
            continue
        orphans.append(path)
    if orphans:
        print('Files outside the allowed layout detected:')
        for orphan in orphans:
            print(f'  - {orphan}')
        return 1
    print('No orphan files detected.')
    return 0


def main() -> int:
    paths = _git_files()
    dup_rc = check_duplicates(paths)
    orphan_rc = check_orphans(paths)
    if dup_rc or orphan_rc:
        print('Repository hygiene checks failed.')
        return 1
    print('Repository hygiene checks passed.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
