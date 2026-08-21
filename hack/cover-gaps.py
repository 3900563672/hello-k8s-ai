#!/usr/bin/env python3
"""列出指定包未覆盖(<阈值)的函数。用法: python3 hack/cover-gaps.py <cover.out> <包关键字> [阈值]"""
import re, sys

coverfile = sys.argv[1]
keyword = sys.argv[2]
threshold = float(sys.argv[3]) if len(sys.argv) > 3 else 60.0

lines = open(coverfile, encoding="utf-8").read().splitlines()
# go tool cover -func 输出
import subprocess
out = subprocess.run(["go", "tool", "cover", "-func=" + coverfile], capture_output=True, text=True).stdout
rows = []
for line in out.splitlines():
    m = re.match(r"^(.*?\.go):(\d+):\s+(\S+)\s+([0-9.]+)%$", line.strip())
    if not m:
        continue
    path, lineno, fn, pct = m.groups()
    pct = float(pct)
    if keyword in path and pct < threshold:
        rows.append((pct, path, lineno, fn))
rows.sort()
for pct, path, lineno, fn in rows:
    print("%6.1f  %s:%s  %s" % (pct, path.split("/")[-1], lineno, fn))
print("--- total:", len(rows))