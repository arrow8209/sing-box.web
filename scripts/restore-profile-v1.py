#!/usr/bin/env python3
"""Offline rollback of one migrated profile; preserves all post-migration edits."""

import argparse
import datetime
import fcntl
import json
import os
from pathlib import Path
import shutil
import tempfile


def atomic_bytes(path: Path, data: bytes) -> None:
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "wb") as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def encoded(value: dict) -> bytes:
    return (json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode()


def main() -> None:
    parser = argparse.ArgumentParser(description="停止服务后，将指定档案恢复到迁移前的 v1 草稿")
    parser.add_argument("workdir", type=Path)
    parser.add_argument("profile_id")
    args = parser.parse_args()
    profile_id = args.profile_id
    if len(profile_id) != 32 or any(char not in "0123456789abcdef" for char in profile_id):
        parser.error("profile_id 必须是 32 位小写十六进制")
    root = args.workdir.resolve(strict=True)
    data = root / "data"
    drafts = data / "drafts"
    current_path = drafts / f"{profile_id}.json"
    index_path = data / "profiles.json"
    core_path = data / "core.json"
    lock_fd = os.open(data / "app.lock", os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        try:
            fcntl.flock(lock_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            raise SystemExit("服务仍在运行，请先停止后执行回退") from exc
        current_bytes = current_path.read_bytes()
        current = json.loads(current_bytes)
        if current.get("schemaVersion") != 2:
            raise SystemExit("目标档案不是 v2 草稿")
        backups = sorted((drafts / "migration-backups").glob(f"{profile_id}-rev*-v1.json"))
        if len(backups) != 1:
            raise SystemExit("找不到唯一的迁移前备份")
        old_bytes = backups[0].read_bytes()
        old = json.loads(old_bytes)
        if old.get("schemaVersion", 1) >= 2 or old.get("profileId") != profile_id:
            raise SystemExit("迁移前备份无效")
        index_bytes = index_path.read_bytes()
        index = json.loads(index_bytes)
        matches = [item for item in index.get("profiles", []) if item.get("id") == profile_id]
        if len(matches) != 1:
            raise SystemExit("档案索引与目标 ID 不匹配")
        core_bytes = core_path.read_bytes() if core_path.exists() else None
        core = json.loads(core_bytes) if core_bytes else None

        stamp = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%S%fZ")
        archive = drafts / "migration-backups" / f"rollback-{stamp}-{profile_id}"
        archive.mkdir(mode=0o700)
        atomic_bytes(archive / "post-migration-draft.json", current_bytes)
        atomic_bytes(archive / "post-migration-profiles.json", index_bytes)
        if core_bytes:
            atomic_bytes(archive / "post-migration-core.json", core_bytes)
        config = root / "config"
        if config.is_dir():
            shutil.copytree(config, archive / "post-migration-config", symlinks=True)

        meta = matches[0]
        meta["draftRevision"] = old["revision"]
        meta["appliedRevision"] = None
        meta["appliedDigest"] = ""
        meta["appliedResourceDigests"] = {}
        meta["appliedAt"] = None
        meta["configCurrent"] = False
        meta["needsApply"] = True
        meta["lastValidation"] = None
        if core is not None:
            core["expectedRunning"] = False
            core["currentProfileId"] = ""
            core["appliedRevision"] = None
            core["configDirectoryDigest"] = ""
        atomic_bytes(current_path, old_bytes)
        atomic_bytes(index_path, encoded(index))
        if core is not None:
            atomic_bytes(core_path, encoded(core))
        print(f"已恢复 v1 草稿；迁移后的修改和运行配置保存在 {archive}")
        print("启动旧程序前，请核对备份；恢复后重新检测并应用该档案。")
    finally:
        fcntl.flock(lock_fd, fcntl.LOCK_UN)
        os.close(lock_fd)


if __name__ == "__main__":
    main()
