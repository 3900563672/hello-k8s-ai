#!/usr/bin/env python3
"""覆盖率门禁：跑主 module + dashboard/backend module，输出每包覆盖率表格，
核心包低于阈值退出非零。阈值目标见 issue #142。

用法:
  python3 hack/coverage-check.py            # 全量跑 + 检查
  python3 hack/coverage-check.py --func     # 只看表格（跑测试）
  TEST_DATABASE_URL=... python3 hack/coverage-check.py   # 启用 store 集成测试 gate

说明: store 依赖真实 PostgreSQL 集成测试；未设置 TEST_DATABASE_URL 时该包标 SKIP（警告不红），
CI 中配置 postgres service 后即为硬 gate。
"""
import argparse
import os
import re
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BACKEND = os.path.join(ROOT, "dashboard", "backend")

# 包前缀 -> 阈值(%)。目标值见 issue #142。
THRESHOLDS = {
    "github.com/3900563672/hello-k8s-ai/internal/controller": 60.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops": 80.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops/prompts": 80.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/clock": 80.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/readmodel": 80.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/segment": 80.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/api": 50.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config": 50.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store": 40.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/kubernetes": 40.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/prometheus": 40.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/jaeger": 40.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/httputil": 30.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/cmd/server": 30.0,
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/app": 30.0,
}

# 需要 TEST_DATABASE_URL 的包：未设置时警告不红。
DB_GATED = {
    "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store",
}


def run(cmd, cwd=None, env=None):
    merged = dict(os.environ)
    if env:
        merged.update(env)
    return subprocess.run(cmd, cwd=cwd, env=merged, capture_output=True, text=True)


def collect_cover(pkg_prefix, cwd):
    """返回 {包名: 覆盖率}。用 go test -cover 标准输出解析（每包一行 ok ... coverage: X%）。

    返回 (result, ok)；测试失败时 ok=False。
    """
    pkgs = run(["go", "list", "./..."], cwd=cwd)
    pkgs_filtered = [p for p in pkgs.stdout.splitlines() if "/e2e" not in p]
    r = run(["go", "test", "-cover"] + pkgs_filtered, cwd=cwd)
    result = {}
    for line in r.stdout.splitlines():
        m = re.match(r"^ok\s+(\S+)\s+.*coverage:\s+([0-9.]+)%", line)
        if m:
            pkg, pct = m.group(1), float(m.group(2))
            if pkg.startswith(pkg_prefix):
                result[pkg] = pct
    return result, r.returncode == 0


def main():
    parser = argparse.ArgumentParser()
    args = parser.parse_args()

    print("[coverage] 跑主 module 测试（跳过 e2e）...")
    covers_root, ok_root = collect_cover("github.com/3900563672/hello-k8s-ai", ROOT)
    if not ok_root:
        print("[coverage] 主 module 测试失败", file=sys.stderr)
        sys.exit(1)
    print("[coverage] 跑 dashboard/backend module 测试...")
    covers_be, ok_be = collect_cover(
        "github.com/3900563672/hello-k8s-ai/dashboard/backend", BACKEND)
    if not ok_be:
        print("[coverage] backend module 测试失败", file=sys.stderr)
        sys.exit(1)
    covers = {}
    covers.update(covers_root)
    covers.update(covers_be)

    rows = []
    failed = []
    skipped = []
    for pkg, threshold in sorted(THRESHOLDS.items(), key=lambda x: (x[0] != "github.com/3900563672/hello-k8s-ai/internal/controller", x[0])):
        pct = covers.get(pkg)
        if pct is None:
            if pkg in DB_GATED and os.environ.get("TEST_DATABASE_URL", "") == "":
                rows.append((pkg, "N/A", threshold, "SKIP-DB"))
                skipped.append(pkg)
                continue
            # 无测试文件/未产出覆盖率 → 视为 0%，低于任何阈值（2026-08-22 起不再豁免）
            rows.append((pkg, 0.0, threshold, "FAIL"))
            failed.append((pkg, 0.0, threshold))
            continue
        if pkg in DB_GATED and os.environ.get("TEST_DATABASE_URL", "") == "":
            rows.append((pkg, pct, threshold, "SKIP-DB"))
            skipped.append(pkg)
            continue
        ok = pct >= threshold
        rows.append((pkg, pct, threshold, "OK" if ok else "FAIL"))
        if not ok:
            failed.append((pkg, pct, threshold))

    print("\n%-90s %8s %8s %6s" % ("包", "覆盖%", "阈值%", "状态"))
    print("-" * 120)
    for pkg, pct, threshold, status in rows:
        pct_s = "%.1f" % pct if isinstance(pct, float) else pct
        print("%-90s %8s %8s %6s" % (pkg, pct_s, "%.1f" % threshold, status))

    print("\n汇总: %d 包 gate, %d 通过, %d 失败, %d 跳过(缺 DB)" % (
        len(rows), len(rows) - len(failed) - len(skipped), len(failed), len(skipped)))
    if failed:
        print("\nFAIL: 以下包未达阈值（issue #142 目标）:")
        for pkg, pct, threshold in failed:
            print("  %s: %.1f%% < %.1f%%" % (pkg, pct, threshold))
        sys.exit(1)
    print("OK: 全部核心包达到阈值")


if __name__ == "__main__":
    main()