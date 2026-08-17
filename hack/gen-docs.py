#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""生成文档体系中的派生文件（make docs-sync）。

幂等：输出只依赖已提交内容（change-history 目录、docs/MAP.yaml、docs 文档标题），
重复运行结果一致，`make docs-sync-check` 依赖这一点。

生成内容：
1. README.md 时间线段（<!-- docs-sync:timeline-start/end --> 之间，最近 5 条）
2. docs/status.md（最新变更摘要；部署实况不在此承载，见 make cluster-status）
3. docs/remote-ai/llms.txt（docs/ 各专题 H1 索引）
4. docs/README.md 所有权表（<!-- docs-sync:ownership-start/end --> 之间，由 MAP.yaml 反向渲染）
"""
import os
import re
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
CH_DIR = os.path.join(ROOT, "change-history")
DOCS_DIR = os.path.join(ROOT, "docs")
TIMELINE_START = "<!-- docs-sync:timeline-start -->"
TIMELINE_END = "<!-- docs-sync:timeline-end -->"
OWNERSHIP_START = "<!-- docs-sync:ownership-start -->"
OWNERSHIP_END = "<!-- docs-sync:ownership-end -->"
LIMIT = 5


def read(path):
    with open(path, encoding="utf-8") as handle:
        return handle.read()


def write(path, text):
    with open(path, "w", encoding="utf-8") as handle:
        handle.write(text)


def latest_entries(limit=LIMIT):
    entries = []
    for name in os.listdir(CH_DIR):
        if not re.match(r"^\d{4}-\d{2}-\d{2}-", name):
            continue
        readme = os.path.join(CH_DIR, name, "README.md")
        if not os.path.isfile(readme):
            continue
        title = ""
        for line in read(readme).splitlines():
            if line.startswith("# "):
                title = line[2:].strip()
                break
        entries.append((name[:10], name, title or name))
    entries.sort(reverse=True)
    return entries[:limit]


def timeline_block(entries):
    lines = [
        TIMELINE_START,
        "最近 %d 条变更（完整时间线见 [change-history/README.md](change-history/README.md)）：" % len(entries),
        "",
    ]
    for date, slug, title in entries:
        lines.append("- %s [%s](change-history/%s/README.md)" % (date, title, slug))
    lines += ["", TIMELINE_END]
    return "\n".join(lines)


def status_block(entries):
    stamp = entries[0][0] if entries else "无"
    lines = [
        "# 项目状态（生成）",
        "",
        "> 维护层：generated | last-reviewed：%s | 事实源：make docs-sync 生成" % stamp,
        "> generated: %s（UTC，对应最新 change-history 日期；仅随变更归档更新）" % stamp,
        "> 本文件由 `make docs-sync` 自动生成，禁止手改。",
        "> 部署实况（Pod / CR / PVC / API 健康）请运行 `make cluster-status`；本页不承载环境快照，避免漂移。",
        "",
        "## 最近变更",
        "",
    ]
    for date, slug, title in entries:
        lines.append("- %s [%s](../change-history/%s/README.md)" % (date, title, slug))
    lines.append("")
    return "\n".join(lines)


def h1_of(path):
    for line in read(path).splitlines():
        if line.startswith("# "):
            return line[2:].strip()
    return os.path.basename(path)


def llms_block():
    skip_dirs = {"agents", "remote-ai", "journal", "lessons"}
    lines = [
        "# hello-k8s-ai",
        "",
        "> hello-k8s-ai 是一个以 Kubernetes API 为唯一事实源的 AI 推理调度与仿真平台：React Frontend → Dashboard Backend → CRD → Controller → Simulator → Prometheus / OpenTelemetry / Jaeger → Backend 聚合展示。",
        "> 阅读顺序：先本索引，再按任务读源码与 change-history/；事实以源码、生成清单和可执行测试为准。",
        "",
    ]
    docs = []
    for dirpath, dirnames, filenames in os.walk(DOCS_DIR):
        dirnames[:] = sorted(d for d in dirnames if d not in skip_dirs and not d.startswith("."))
        for filename in sorted(filenames):
            if filename.endswith(".md"):
                rel = os.path.relpath(os.path.join(dirpath, filename), ROOT).replace("\\", "/")
                docs.append(rel)
    for rel in sorted(docs):
        lines.append("- [%s](%s): %s" % (rel, rel, h1_of(os.path.join(ROOT, rel))))
    lines.append("")
    return "\n".join(lines)


def parse_map(path):
    """MAP.yaml 极简解析（结构固定：key 行 + '  - doc' 行），避免 PyYAML 依赖。"""
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


def ownership_block():
    mapping = parse_map(os.path.join(DOCS_DIR, "MAP.yaml"))
    reverse = {}
    for src, docs in mapping.items():
        for doc in docs:
            reverse.setdefault(doc, []).append(src)
    lines = [OWNERSHIP_START, "", "| 文档 | 映射源码路径 |", "| --- | --- |"]
    for doc in sorted(reverse):
        sources = "、".join("`%s`" % s for s in sorted(reverse[doc]))
        lines.append("| `%s` | %s |" % (doc, sources))
    lines += ["", OWNERSHIP_END]
    return "\n".join(lines)


def replace_block(text, start, end, block):
    start_idx = text.find(start)
    end_idx = text.find(end)
    if start_idx == -1 or end_idx == -1:
        raise SystemExit("缺少生成标记：%s / %s" % (start, end))
    return text[:start_idx] + block + text[end_idx + len(end):]


def main():
    entries = latest_entries()

    readme_path = os.path.join(ROOT, "README.md")
    readme = read(readme_path)
    readme = replace_block(readme, TIMELINE_START, TIMELINE_END, timeline_block(entries))
    write(readme_path, readme)

    write(os.path.join(DOCS_DIR, "status.md"), status_block(entries))
    write(os.path.join(DOCS_DIR, "remote-ai", "llms.txt"), llms_block())

    docs_readme_path = os.path.join(DOCS_DIR, "README.md")
    docs_readme = read(docs_readme_path)
    docs_readme = replace_block(docs_readme, OWNERSHIP_START, OWNERSHIP_END, ownership_block())
    write(docs_readme_path, docs_readme)

    print("docs-sync OK：README 时间线段 / docs/status.md / docs/remote-ai/llms.txt / docs/README 所有权表")


if __name__ == "__main__":
    main()