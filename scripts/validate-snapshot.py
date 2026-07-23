#!/usr/bin/env python3
"""Validate and safely extract a FlashGate Git snapshot."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import posixpath
import shutil
import stat
import sys
import tarfile
from pathlib import Path, PurePosixPath


class SnapshotError(RuntimeError):
    pass


def fail(message: str) -> None:
    raise SnapshotError(message)


def normalize_path(value: str) -> str:
    if not value or "\x00" in value or "\\" in value or value.startswith("/"):
        fail(f"unsafe snapshot path: {value!r}")
    pure = PurePosixPath(value)
    if any(part in {"", ".", ".."} for part in pure.parts):
        fail(f"noncanonical snapshot path: {value!r}")
    normalized = posixpath.normpath(value)
    if normalized != value or normalized.startswith("../"):
        fail(f"noncanonical snapshot path: {value!r}")
    return normalized


def load_manifest(path: Path) -> dict[str, tuple[int, str]]:
    with path.open("r", encoding="utf-8") as stream:
        manifest = json.load(stream)
    if set(manifest) != {"schema", "files"}:
        fail("snapshot manifest has unexpected fields")
    if manifest["schema"] != "flashgate-git-snapshot-manifest/v1":
        fail("snapshot manifest has an unexpected schema")

    result: dict[str, tuple[int, str]] = {}
    for record in manifest["files"]:
        if set(record) != {"Path", "Type", "Length", "SHA256"}:
            fail("snapshot manifest record has unexpected fields")
        name = normalize_path(record["Path"])
        if name in result:
            fail(f"duplicate manifest path: {name}")
        if record["Type"] != "file":
            fail(f"unsupported manifest type: {record['Type']}")
        length = record["Length"]
        digest = record["SHA256"]
        if not isinstance(length, int) or length < 0:
            fail(f"invalid manifest length: {name}")
        if (
            not isinstance(digest, str)
            or len(digest) != 64
            or any(character not in "0123456789abcdef" for character in digest)
        ):
            fail(f"invalid manifest SHA-256: {name}")
        result[name] = (length, digest)
    if not result:
        fail("snapshot manifest is empty")
    return result


def required_directories(files: set[str]) -> set[str]:
    directories: set[str] = set()
    for name in files:
        parent = PurePosixPath(name).parent
        while str(parent) != ".":
            directories.add(str(parent))
            parent = parent.parent
    return directories


def validate_archive(
    archive: tarfile.TarFile,
    manifest: dict[str, tuple[int, str]],
) -> dict[str, tarfile.TarInfo]:
    members: dict[str, tarfile.TarInfo] = {}
    allowed_directories = required_directories(set(manifest))
    for member in archive.getmembers():
        name = normalize_path(member.name)
        if name in members:
            fail(f"duplicate normalized TAR path: {name}")
        members[name] = member
        if member.isfile():
            if name not in manifest:
                fail(f"unexpected snapshot file: {name}")
            if member.size != manifest[name][0]:
                fail(f"snapshot TAR length mismatch: {name}")
        elif member.isdir():
            if name not in allowed_directories:
                fail(f"unexpected snapshot directory: {name}")
        else:
            fail(f"unsupported TAR entry type for {name}")

    archive_files = {name for name, member in members.items() if member.isfile()}
    missing = set(manifest) - archive_files
    if missing:
        fail(f"snapshot TAR is missing files: {sorted(missing)!r}")
    return members


def open_directory_child(parent_fd: int, name: str) -> int:
    try:
        os.mkdir(name, 0o700, dir_fd=parent_fd)
    except FileExistsError:
        metadata = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
        if stat.S_ISLNK(metadata.st_mode) or not stat.S_ISDIR(metadata.st_mode):
            fail(f"non-directory extraction component: {name}")
    return os.open(
        name,
        os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW,
        dir_fd=parent_fd,
    )


def open_parent(root_fd: int, path: PurePosixPath) -> tuple[int, str]:
    current_fd = os.dup(root_fd)
    try:
        for part in path.parts[:-1]:
            next_fd = open_directory_child(current_fd, part)
            os.close(current_fd)
            current_fd = next_fd
        return current_fd, path.name
    except Exception:
        os.close(current_fd)
        raise


def extract_regular_files(
    archive: tarfile.TarFile,
    members: dict[str, tarfile.TarInfo],
    extraction_root: Path,
) -> None:
    if extraction_root.exists() or extraction_root.is_symlink():
        fail("extraction root must not already exist")
    extraction_root.mkdir(mode=0o700, parents=False)
    root_fd = os.open(
        extraction_root,
        os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW,
    )
    try:
        for name in sorted(members):
            member = members[name]
            if not member.isfile():
                continue
            parent_fd, leaf = open_parent(root_fd, PurePosixPath(name))
            try:
                descriptor = os.open(
                    leaf,
                    os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                    0o600,
                    dir_fd=parent_fd,
                )
                source = archive.extractfile(member)
                if source is None:
                    fail(f"unable to read snapshot member: {name}")
                with source, os.fdopen(descriptor, "wb", closefd=True) as destination:
                    shutil.copyfileobj(source, destination, length=1024 * 1024)
            finally:
                os.close(parent_fd)
    finally:
        os.close(root_fd)


def verify_extracted(
    extraction_root: Path,
    manifest: dict[str, tuple[int, str]],
) -> None:
    actual_files: set[str] = set()
    for current_root, directory_names, file_names in os.walk(
        extraction_root,
        topdown=True,
        followlinks=False,
    ):
        root = Path(current_root)
        for name in directory_names:
            path = root / name
            metadata = os.stat(path, follow_symlinks=False)
            if stat.S_ISLNK(metadata.st_mode) or not stat.S_ISDIR(metadata.st_mode):
                fail(f"unsafe extracted directory: {path}")
        for name in file_names:
            path = root / name
            relative = path.relative_to(extraction_root).as_posix()
            metadata = os.stat(path, follow_symlinks=False)
            if stat.S_ISLNK(metadata.st_mode) or not stat.S_ISREG(metadata.st_mode):
                fail(f"unsafe extracted file: {relative}")
            actual_files.add(relative)
            expected_length, expected_hash = manifest.get(relative, (-1, ""))
            digest = hashlib.sha256()
            with path.open("rb") as stream:
                for block in iter(lambda: stream.read(1024 * 1024), b""):
                    digest.update(block)
            if metadata.st_size != expected_length:
                fail(f"extracted length mismatch: {relative}")
            if digest.hexdigest() != expected_hash:
                fail(f"extracted SHA-256 mismatch: {relative}")
    if actual_files != set(manifest):
        fail("extracted inventory does not match the snapshot manifest")


def overlay_verified_files(
    extraction_root: Path,
    overlay_root: Path,
    manifest: dict[str, tuple[int, str]],
) -> None:
    if not overlay_root.is_dir() or overlay_root.is_symlink():
        fail("overlay root must be a real existing directory")
    root_fd = os.open(
        overlay_root,
        os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW,
    )
    try:
        for name in sorted(manifest):
            parent_fd, leaf = open_parent(root_fd, PurePosixPath(name))
            temporary = f".flashgate-snapshot-{os.getpid()}-{leaf}"
            try:
                descriptor = os.open(
                    temporary,
                    os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                    0o600,
                    dir_fd=parent_fd,
                )
                with (
                    (extraction_root / name).open("rb") as source,
                    os.fdopen(descriptor, "wb", closefd=True) as destination,
                ):
                    shutil.copyfileobj(source, destination, length=1024 * 1024)
                existing = None
                try:
                    existing = os.stat(
                        leaf,
                        dir_fd=parent_fd,
                        follow_symlinks=False,
                    )
                except FileNotFoundError:
                    pass
                if existing is not None and not stat.S_ISREG(existing.st_mode):
                    fail(f"unsafe overlay target: {name}")
                os.replace(
                    temporary,
                    leaf,
                    src_dir_fd=parent_fd,
                    dst_dir_fd=parent_fd,
                )
            finally:
                try:
                    os.unlink(temporary, dir_fd=parent_fd)
                except FileNotFoundError:
                    pass
                os.close(parent_fd)
    finally:
        os.close(root_fd)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--archive", required=True)
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--extract-root", required=True)
    parser.add_argument("--overlay-root")
    arguments = parser.parse_args()

    extraction_root = Path(arguments.extract_root)
    try:
        manifest = load_manifest(Path(arguments.manifest))
        with tarfile.open(arguments.archive, "r:") as archive:
            members = validate_archive(archive, manifest)
            extract_regular_files(archive, members, extraction_root)
        verify_extracted(extraction_root, manifest)
        if arguments.overlay_root:
            overlay_verified_files(
                extraction_root,
                Path(arguments.overlay_root),
                manifest,
            )
    except (OSError, tarfile.TarError, ValueError, SnapshotError) as error:
        print(f"validate-snapshot: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
