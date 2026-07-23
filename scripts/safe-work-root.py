#!/usr/bin/env python3
"""Create or remove one validation work root without following links."""

from __future__ import annotations

import argparse
import os
import re
import stat
import sys
from pathlib import Path


RUN_ID_PATTERN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,79}$")


class SafetyError(RuntimeError):
    pass


def fail(message: str) -> None:
    raise SafetyError(message)


def validate_run_id(value: str) -> None:
    if not RUN_ID_PATTERN.fullmatch(value):
        fail("FG_RUN_ID does not match the required restricted format")
    if value in {".", ".."} or value.startswith("."):
        fail("FG_RUN_ID is not a permitted direct-child name")


def directory_open_flags() -> int:
    for required in ("O_DIRECTORY", "O_NOFOLLOW"):
        if not hasattr(os, required):
            fail(f"required descriptor flag is unavailable: {required}")
    return (
        os.O_RDONLY
        | os.O_DIRECTORY
        | os.O_NOFOLLOW
        | getattr(os, "O_CLOEXEC", 0)
    )


def same_identity(left: os.stat_result, right: os.stat_result) -> bool:
    return (left.st_dev, left.st_ino, left.st_mode) == (
        right.st_dev,
        right.st_ino,
        right.st_mode,
    )


def open_directory_component(parent_fd: int, name: str) -> int:
    try:
        before = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
    except FileNotFoundError as error:
        raise SafetyError(f"required directory component does not exist: {name}") from error
    if stat.S_ISLNK(before.st_mode):
        fail(f"symbolic-link directory component rejected: {name}")
    if not stat.S_ISDIR(before.st_mode):
        fail(f"non-directory path component rejected: {name}")

    descriptor = os.open(
        name,
        directory_open_flags(),
        dir_fd=parent_fd,
    )
    try:
        opened = os.fstat(descriptor)
        current = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
        if not same_identity(before, opened) or not same_identity(opened, current):
            fail(f"directory component changed while being bound: {name}")
        return descriptor
    except Exception:
        os.close(descriptor)
        raise


def open_bound_base_directory(base: Path) -> int:
    value = os.fspath(base)
    if not base.is_absolute() or base.anchor != os.path.sep:
        fail("validation base must be an absolute POSIX path")
    if os.path.normpath(value) != value:
        fail("validation base must be lexically canonical")
    components = base.parts[1:]
    if not components or any(part in {"", ".", ".."} for part in components):
        fail("validation base must contain safe directory components")
    if value in {os.path.sep, "/home"}:
        fail("validation base must not be a protected directory")
    configured_home = os.environ.get("HOME")
    if configured_home and value == configured_home:
        fail("validation base must not equal HOME")

    current_fd = os.open(os.path.sep, directory_open_flags())
    try:
        for component in components:
            next_fd = open_directory_component(current_fd, component)
            previous_fd = current_fd
            current_fd = next_fd
            os.close(previous_fd)

        metadata = os.fstat(current_fd)
        if not stat.S_ISDIR(metadata.st_mode):
            fail("validation base descriptor is not a directory")
        if metadata.st_uid != os.geteuid():
            fail("validation base must be owned by the current user")
        if metadata.st_mode & 0o022:
            fail("validation base must not be group- or world-writable")
        return current_fd
    except Exception:
        os.close(current_fd)
        raise


def remove_directory_contents(directory_fd: int) -> None:
    entries = list(os.scandir(directory_fd))
    for entry in entries:
        before = entry.stat(follow_symlinks=False)
        if stat.S_ISLNK(before.st_mode):
            fail(f"symbolic link encountered during removal: {entry.name}")
        if stat.S_ISDIR(before.st_mode):
            child_fd = os.open(
                entry.name,
                directory_open_flags(),
                dir_fd=directory_fd,
            )
            try:
                opened = os.fstat(child_fd)
                current = os.stat(
                    entry.name,
                    dir_fd=directory_fd,
                    follow_symlinks=False,
                )
                if not same_identity(before, opened) or not same_identity(opened, current):
                    fail(f"directory component changed during removal: {entry.name}")
                remove_directory_contents(child_fd)
            finally:
                os.close(child_fd)
            current = os.stat(
                entry.name,
                dir_fd=directory_fd,
                follow_symlinks=False,
            )
            if not same_identity(before, current):
                fail(f"directory component changed before deletion: {entry.name}")
            os.rmdir(entry.name, dir_fd=directory_fd)
        elif stat.S_ISREG(before.st_mode):
            current = os.stat(
                entry.name,
                dir_fd=directory_fd,
                follow_symlinks=False,
            )
            if not same_identity(before, current):
                fail(f"file component changed before deletion: {entry.name}")
            os.unlink(entry.name, dir_fd=directory_fd)
        else:
            fail(f"special file rejected during removal: {entry.name}")


def remove_run_directory(base: Path, run_id: str) -> None:
    validate_run_id(run_id)
    base_fd = open_bound_base_directory(base)
    try:
        try:
            lexical = os.stat(run_id, dir_fd=base_fd, follow_symlinks=False)
        except FileNotFoundError:
            return
        if stat.S_ISLNK(lexical.st_mode) or not stat.S_ISDIR(lexical.st_mode):
            fail("work root is not a real directory")

        work_fd = os.open(
            run_id,
            directory_open_flags(),
            dir_fd=base_fd,
        )
        try:
            opened = os.fstat(work_fd)
            current = os.stat(run_id, dir_fd=base_fd, follow_symlinks=False)
            if not same_identity(lexical, opened) or not same_identity(opened, current):
                fail("work-root component changed before removal")
            if opened.st_uid != os.geteuid():
                fail("work root must be owned by the current user")
            remove_directory_contents(work_fd)
        finally:
            os.close(work_fd)

        current = os.stat(run_id, dir_fd=base_fd, follow_symlinks=False)
        if not same_identity(lexical, current):
            fail("work-root component changed before final deletion")
        os.rmdir(run_id, dir_fd=base_fd)
    finally:
        os.close(base_fd)


def create_run_directory(base: Path, run_id: str) -> Path:
    validate_run_id(run_id)
    base_fd = open_bound_base_directory(base)
    try:
        try:
            os.mkdir(run_id, 0o700, dir_fd=base_fd)
        except FileExistsError as error:
            raise SafetyError("work root already exists") from error
        work_fd = os.open(
            run_id,
            directory_open_flags(),
            dir_fd=base_fd,
        )
        try:
            metadata = os.fstat(work_fd)
            current = os.stat(run_id, dir_fd=base_fd, follow_symlinks=False)
            if not same_identity(metadata, current):
                fail("work-root component changed during creation")
            if metadata.st_uid != os.geteuid():
                fail("created work root has an unexpected owner")
        finally:
            os.close(work_fd)
    finally:
        os.close(base_fd)

    return base / run_id


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("operation", choices=("create", "remove"))
    parser.add_argument("--base", required=True)
    parser.add_argument("--run-id", required=True)
    arguments = parser.parse_args()

    try:
        validate_run_id(arguments.run_id)
        base = Path(arguments.base)
        if arguments.operation == "remove":
            remove_run_directory(base, arguments.run_id)
        else:
            print(create_run_directory(base, arguments.run_id))
    except (OSError, SafetyError) as error:
        print(f"safe-work-root: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
