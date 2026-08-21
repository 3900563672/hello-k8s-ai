#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""文档一致性检查（make docs-check）。

检查项（error 为失败，warning 仅提示）：
1. Markdown 相对链接 / 图片路径目标存在
2. 根目录 markdown 白名单：只允许 README.md、AGENTS.md、PROJECT_OVERVIEW_NEW.md、CONTRIBUTING.md、SECURITY.md、AI_COORDINATION.md
3. 入口行数上限：README<=150、AGENTS.md<=200、docs/remote-ai/README.md<=60、docs/remote-ai/llms.txt<=80
4. MAP 门禁：diff 命中的源码路径，其映射文档必须同时出现在 diff 中（最长匹配）
5. 生成物新鲜度：README 时间线段与 docs/status.md 必须包含最新 change-history 目录
6. 文档时间戳：docs/ 专题缺 last-reviewed 头部告警（迁移过渡期）；落后最新变更超 30 天报错
7. 白皮书新鲜度：whitepaper/COMPLETE_OVERVIEW.md 落后最新变更超 14 天告警
8. change-history 格式：目录名 YYYY-MM-DD-*、README.md 存在且带日期元信息
9. journal 格式：文件名 YYYY-MM-DD-*.md 且带日期字段
10. 孤儿文档：docs/ 下 .md 未被任何文档或 llms.txt 链接（告警）

MAP 门禁的 diff 范围：环境变量 DOCS_CHECK_BASE（CI 的 PR 用 base ref）或默认 HEAD~1。
"""
import os
import re
import subprocess
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
LINK_RE = re.compile(r"!?\[[^\]]*\]\(([^)]+)\)")
FM_RE = re.compile(r">\s*维护层：[^|｜]+[|｜]\s*last-reviewed：(\d{4}-\d{2}-\d{2})")
EXCLUDE_DIRS = {
    ".git", "node_modules", "bin", "dist", ".runtime", ".idea", ".verify",
    "output", "tmp", "project-review", ".devcontainer",
}
ROOT_MD_WHITELIST = {"README.md", "AGENTS.md", "PROJECT_OVERVIEW_NEW.md", "CONTRIBUTING.md", "SECURITY.md", "AI_COORDINATION.md"}
LINE_LIMITS = {
    "README.md": 150,
    "AGENTS.md": 200,
    os.path.join("docs", "remote-ai", "README.md"): 60,
    os.path.join("docs", "remote-ai", "llms.txt"): 80,
}
FMT_HEADER_DAYS_ERROR = 30
FMT_HEADER_DAYS_WARN = 14


def read(path):
    with open(path, encoding="utf-8") as handle:
        return handle.read()


def iter_md():
    for dirpath, dirnames, filenames in os.walk(ROOT):
        dirnames[:] = [d for d in dirnames if d not in EXCLUDE_DIRS]
        for filename in filenames:
            if filename.endswith(".md"):
                yield os.path.join(dirpath, filename)


def days_between(latest, older):
    from datetime import date
    try:
        latest_d = date(*[int(x) for x in latest.split("-")])
        older_d = date(*[int(x) for x in older.split("-")])
        return (latest_d - older_d).days
    except (ValueError, TypeError):
        return None


def latest_ch_date():
    dates = []
    for name in os.listdir(os.path.join(ROOT, "change-history")):
        m = re.match(r"^(\d{4}-\d{2}-\d{2})-", name)
        if m:
            dates.append(m.group(1))
    return max(dates) if dates else None


def check_links():
    errors = 0
    for path in iter_md():
        rel = os.path.relpath(path, ROOT)
        for lineno, line in enumerate(read(path).splitlines(), 1):
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
    return errors


def check_root_whitelist():
    errors = 0
    for name in os.listdir(ROOT):
        if name.endswith(".md") and name not in ROOT_MD_WHITELIST:
            print(f"根目录 markdown 白名单违规：{name}（只允许 README.md / AGENTS.md / PROJECT_OVERVIEW_NEW.md / CONTRIBUTING.md / SECURITY.md / AI_COORDINATION.md）")
            errors += 1
    return errors


def check_line_limits():
    errors = 0
    for rel, limit in LINE_LIMITS.items():
        path = os.path.join(ROOT, rel)
        if not os.path.isfile(path):
            print(f"入口文件缺失：{rel}")
            errors += 1
            continue
        count = len(read(path).splitlines())
        if count > limit:
            print(f"入口行数超限：{rel} = {count} 行 > {limit}")
            errors += 1
    return errors


def parse_map(path):
    data = {}
    current = None
    with open(path, encoding="utf-8") as handle:
        for raw in handle:
            line = raw.rstrip()
            if not line or line.startswith("#"):
                continue
            if not line.startswith(" "):
                current = line.rstrip(":").strip()
                data[current] = []
            elif line.startswith("  - "):
                data[current].append(line[4:].strip())
    return data


def changed_files():
    base = os.environ.get("DOCS_CHECK_BASE")
    if base:
        merged = subprocess.run(
            ["git", "merge-base", base, "HEAD"], cwd=ROOT,
            capture_output=True, text=True, check=False,
        )
        if merged.returncode != 0:
            print(f"warning: 无法计算 {base} 与 HEAD 的 merge-base，MAP 门禁跳过")
            return []
        left = merged.stdout.strip()
    else:
        left = "HEAD~1"
    diff = subprocess.run(
        ["git", "diff", "--name-only", left, "HEAD"], cwd=ROOT,
        capture_output=True, text=True, check=False,
    )
    if diff.returncode != 0:
        empty_tree = subprocess.run(
            ["git", "hash-object", "-t", "tree", os.devnull], cwd=ROOT,
            capture_output=True, text=True, check=False,
        )
        if empty_tree.returncode != 0:
            return []
        left = empty_tree.stdout.strip()
        diff = subprocess.run(
            ["git", "diff", "--name-only", left, "HEAD"], cwd=ROOT,
            capture_output=True, text=True, check=False,
        )
        if diff.returncode != 0:
            return []
    return [line for line in diff.stdout.splitlines() if line]


def is_test_file(path):
    """测试文件不改变行为契约，MAP 门禁豁免；行为变更必然同时触碰非测试代码，门禁仍生效。"""
    if path.endswith("_test.go"):
        return True
    return re.search(r"\.(test|spec)\.(ts|tsx|js|jsx)$", path) is not None


# 文档类路径前缀：这些路径的改动视为「文档交付」，不强制配 change-history 条目。
CH_DOC_PREFIXES = ("docs/", "change-history/", "README.md", "AGENTS.md",
                   "CONTRIBUTING.md", "SECURITY.md", "PROJECT_OVERVIEW_NEW.md")


def diff_base():
    """门禁共用的 diff 左边界：DOCS_CHECK_BASE（CI 的 PR base）或默认 HEAD~1。"""
    base = os.environ.get("DOCS_CHECK_BASE")
    if base:
        merged = subprocess.run(
            ["git", "merge-base", base, "HEAD"], cwd=ROOT,
            capture_output=True, text=True, check=False,
        )
        if merged.returncode != 0:
            return None
        return merged.stdout.strip()
    return "HEAD~1"


def check_change_history_gate():
    """change-history 门禁：非文档源码改动必须配条目（2026-08-21 起）。

    通过条件（任一）：
    ① diff 新增 change-history/YYYY-MM-DD-*/README.md；
    ② 提交信息显式引用条目（change-history: <条目名> 或 change-history/<条目名>/），
       且条目目录存在（允许小改动并入既有条目）。
    豁免：纯文档改动（docs/、change-history/、根文档）。测试/CI/脚本/后端/前端均视为交付。
    """
    changed = changed_files()
    if not changed:
        return 0
    source_files = [path for path in changed if not path.startswith(CH_DOC_PREFIXES)]
    if not source_files:
        return 0
    left = diff_base()
    if left is None:
        return 0
    # ① diff 新增条目
    diff = subprocess.run(
        ["git", "diff", "--name-status", left, "HEAD"], cwd=ROOT,
        capture_output=True, text=True, check=False,
    )
    if diff.returncode == 0:
        for line in diff.stdout.splitlines():
            parts = line.split("\t")
            if len(parts) >= 2 and parts[0].startswith("A") \
                    and parts[1].startswith("change-history/") and parts[1].endswith("/README.md"):
                return 0
    # ② 提交信息引用已有条目
    log = subprocess.run(
        ["git", "log", "--format=%B", f"{left}..HEAD"], cwd=ROOT,
        capture_output=True, text=True, check=False,
    )
    if log.returncode == 0:
        messages = log.stdout
        for entry in re.findall(r"change-history[:：]\s*([A-Za-z0-9_-]+)", messages):
            if os.path.isfile(os.path.join(ROOT, "change-history", entry, "README.md")):
                return 0
        for entry in re.findall(r"change-history/([A-Za-z0-9_-]+)/", messages):
            if os.path.isfile(os.path.join(ROOT, "change-history", entry, "README.md")):
                return 0
    preview = "、".join(source_files[:3]) + ("…" if len(source_files) > 3 else "")
    print(f"CHANGE_HISTORY 门禁：本次 diff 含非文档源码改动（{preview}），"
          "但未新增 change-history 条目，提交信息也未引用（格式：change-history: <条目名>）。")
    return 1


def check_map_gate():
    map_path = os.path.join(ROOT, "docs", "MAP.yaml")
    if not os.path.isfile(map_path):
        print("docs/MAP.yaml 缺失，MAP 门禁跳过")
        return 0
    mapping = parse_map(map_path)
    changed = changed_files()
    if not changed:
        return 0
    errors = 0
    for path in changed:
        if path.startswith("docs/") or path.startswith("change-history/"):
            continue
        if is_test_file(path):
            continue
        best = None
        for src in mapping:
            if path == src or (src.endswith("/") and path.startswith(src)):
                if best is None or len(src) > len(best):
                    best = src
        if best is None:
            continue
        missing = [doc for doc in mapping[best] if doc not in changed]
        if missing:
            print(f"MAP 门禁：{path} 命中映射 {best}，但以下文档未在本提交中更新：{', '.join(missing)}")
            errors += 1
    return errors


def check_freshness():
    latest = latest_ch_date()
    if not latest:
        return 0
    errors = 0
    readme = read(os.path.join(ROOT, "README.md"))
    if latest not in readme:
        print(f"README 时间线段未包含最新 change-history：{latest}（运行 make docs-sync）")
        errors += 1
    status_path = os.path.join(ROOT, "docs", "status.md")
    if not os.path.isfile(status_path):
        print("docs/status.md 缺失（运行 make docs-sync）")
        errors += 1
    elif latest not in read(status_path):
        print(f"docs/status.md 未包含最新 change-history：{latest}（运行 make docs-sync）")
        errors += 1
    return errors


def check_front_matter():
    latest = latest_ch_date()
    errors = 0
    warnings = 0
    for dirpath, dirnames, filenames in os.walk(os.path.join(ROOT, "docs")):
        dirnames[:] = [d for d in dirnames if d not in ("journal", "lessons")]
        for filename in filenames:
            if not filename.endswith(".md"):
                continue
            path = os.path.join(dirpath, filename)
            rel = os.path.relpath(path, ROOT)
            head = read(path)[:400]
            match = FM_RE.search(head)
            if not match:
                print(f"warning: {rel} 缺 front-matter（维护层 | last-reviewed | 事实源），迁移过渡期放行")
                warnings += 1
                continue
            reviewed = match.group(1)
            days = days_between(latest, reviewed)
            if days is None:
                continue
            if days > FMT_HEADER_DAYS_ERROR:
                print(f"{rel}: last-reviewed {reviewed} 落后最新变更 {days} 天（> {FMT_HEADER_DAYS_ERROR} 天）")
                errors += 1
            elif days > FMT_HEADER_DAYS_WARN:
                print(f"warning: {rel}: last-reviewed {reviewed} 落后最新变更 {days} 天")
                warnings += 1
    print(f"front-matter：{errors} error / {warnings} warning")
    return errors


def check_whitepaper_freshness():
    latest = latest_ch_date()
    if not latest:
        return 0
    path = os.path.join(ROOT, "docs", "whitepaper", "COMPLETE_OVERVIEW.md")
    if not os.path.isfile(path):
        return 0
    head = read(path)[:400]
    match = FM_RE.search(head)
    if not match:
        print("warning: whitepaper/COMPLETE_OVERVIEW.md 缺 front-matter")
        return 0
    days = days_between(latest, match.group(1))
    if days is not None and days > FMT_HEADER_DAYS_WARN:
        print(f"warning: 白皮书 last-reviewed 落后最新变更 {days} 天，建议同步")
    return 0


def check_change_history():
    errors = 0
    ch_root = os.path.join(ROOT, "change-history")
    for name in os.listdir(ch_root):
        if name == "README.md":
            continue
        path = os.path.join(ch_root, name)
        if not os.path.isdir(path):
            print(f"change-history: 非目录条目 {name}")
            errors += 1
            continue
        if not re.match(r"^\d{4}-\d{2}-\d{2}-", name):
            print(f"change-history: 目录名不符合 YYYY-MM-DD-*：{name}")
            errors += 1
            continue
        readme = os.path.join(path, "README.md")
        if not os.path.isfile(readme):
            print(f"change-history: {name} 缺 README.md")
            errors += 1
            continue
        text = read(readme)
        if not re.search(r"变更日期|日期：", text, re.M):
            print(f"change-history: {name}/README.md 缺日期元信息（变更日期 / > 日期：）")
            errors += 1
    return errors


def check_journal():
    errors = 0
    journal_root = os.path.join(ROOT, "docs", "journal")
    if not os.path.isdir(journal_root):
        return 0
    for name in os.listdir(journal_root):
        if name == "README.md":
            continue
        path = os.path.join(journal_root, name)
        if not os.path.isfile(path):
            print(f"journal: 非文件条目 {name}")
            errors += 1
            continue
        if not re.match(r"^\d{4}-\d{2}-\d{2}-.+\.md$", name):
            print(f"journal: 文件名不符合 YYYY-MM-DD-*.md：{name}")
            errors += 1
            continue
        if "> 日期：" not in read(path):
            print(f"journal: {name} 缺 > 日期： 字段")
            errors += 1
    return errors


def collect_links():
    """收集全部 Markdown 链接目标（相对路径解析到仓库根）。"""
    targets = set()
    sources = []
    for path in iter_md():
        sources.append(path)
    llms = os.path.join(ROOT, "docs", "remote-ai", "llms.txt")
    if os.path.isfile(llms):
        sources.append(llms)
    for path in sources:
        rel_dir = os.path.dirname(os.path.relpath(path, ROOT))
        for line in read(path).splitlines():
            for match in LINK_RE.finditer(line):
                target = match.group(1).strip()
                if not target or target.startswith(("http://", "https://", "mailto:", "#", "<")):
                    continue
                file_part = target.split("#")[0].split("?")[0]
                if not file_part:
                    continue
                # docs/、change-history/ 开头的目标按仓库根相对路径解析（llms.txt 约定）
                if file_part.startswith(("docs/", "change-history/")):
                    resolved = os.path.normpath(file_part)
                else:
                    resolved = os.path.normpath(os.path.join(rel_dir, file_part))
                if resolved.endswith((".md", ".txt", ".yaml")):
                    targets.add(resolved)
    return targets


def check_orphan_docs():
    linked = collect_links()
    warnings = 0
    skip_dirs = {os.path.join(ROOT, "docs", "journal"), os.path.join(ROOT, "docs", "lessons")}
    for dirpath, dirnames, filenames in os.walk(os.path.join(ROOT, "docs")):
        if dirpath in skip_dirs:
            continue
        for filename in filenames:
            if not filename.endswith(".md"):
                continue
            rel = os.path.relpath(os.path.join(dirpath, filename), ROOT)
            if rel not in linked:
                print(f"warning: 孤儿文档（未被任何文档/llms.txt 链接）：{rel}")
                warnings += 1
    return 0


def main():
    total = 0
    total += check_links()
    total += check_root_whitelist()
    total += check_line_limits()
    total += check_map_gate()
    total += check_change_history_gate()
    total += check_freshness()
    total += check_front_matter()
    total += check_whitepaper_freshness()
    total += check_change_history()
    total += check_journal()
    check_orphan_docs()
    if total:
        print(f"共 {total} 处 error")
        sys.exit(1)
    print("docs-check OK")


if __name__ == "__main__":
    main()