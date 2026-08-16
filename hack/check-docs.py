#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""检查仓库内 Markdown 的相对链接与图片路径目标是否存在。

- 忽略外部链接（http/https/mailto）、纯锚点（#...）与尖括号链接。
- 链接带锚点（path#anchor）时只校验文件路径部分。
- 目录也算存在；不存在时报错并返回非 0。
"""
import os
import re
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
LINK_RE = re.compile(r"!?\[[^\]]*\]\(([^)]+)\)")
EXCLUDE_DIRS = {
    ".git", "node_modules", "bin", "dist", ".runtime", ".idea", ".verify",
    "output", "tmp", "project-review", ".devcontainer",
}


def iter_md():
    for dirpath, dirnames, filenames in os.walk(ROOT):
        dirnames[:] = [d for d in dirnames if d not in EXCLUDE_DIRS]
        for filename in filenames:
            if filename.endswith(".md"):
                yield os.path.join(dirpath, filename)


def main():
    errors = 0
    for path in iter_md():
        rel = os.path.relpath(path, ROOT)
        with open(path, encoding="utf-8") as handle:
            for lineno, line in enumerate(handle, 1):
                for match in LINK_RE.finditer(line):
                    target = match.group(1).strip()
                    if not target or target.startswith(("http://", "https://", "mailto:", "#", "<")):
                        continue
                    file_part = target.split("#")[0].split("?")[0]
                    if not file_part:
                        continue
                    resolved = os.path.normpath(os.path.join(os.path.dirname(path), file_part))
                    if not os.path.exists(resolved):
                        print(f"{rel}:{lineno}: 链接目标不存在 -> {target}")
                        errors += 1
    if errors:
        print(f"共 {errors} 处链接错误")
        sys.exit(1)
    print("docs-check OK")


if __name__ == "__main__":
    main()