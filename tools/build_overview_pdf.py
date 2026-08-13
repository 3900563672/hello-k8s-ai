#!/usr/bin/env python3
"""从唯一 Markdown 正文构建 hello-k8s-ai 项目概览 PDF。

渲染器只支持 COMPLETE_OVERVIEW.md 使用的 Markdown 结构，并将 Mermaid 关系转换为
紧凑、可搜索的矢量图，不嵌入截图。
"""

from __future__ import annotations

import argparse
import html
import re
from pathlib import Path
from typing import Iterable

from reportlab.lib import colors
from reportlab.lib.enums import TA_CENTER, TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.cidfonts import UnicodeCIDFont
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen.canvas import Canvas
from reportlab.platypus import (
    BaseDocTemplate,
    Flowable,
    Frame,
    KeepTogether,
    PageBreak,
    PageTemplate,
    Paragraph,
    Preformatted,
    Spacer,
    Table,
    TableStyle,
)
from reportlab.platypus.tableofcontents import TableOfContents


PAGE_WIDTH, PAGE_HEIGHT = A4
MARGIN_X = 17 * mm
MARGIN_TOP = 20 * mm
MARGIN_BOTTOM = 17 * mm
CONTENT_WIDTH = PAGE_WIDTH - 2 * MARGIN_X

INK = colors.HexColor("#122033")
MUTED = colors.HexColor("#5B6B7F")
NAVY = colors.HexColor("#173B63")
BLUE = colors.HexColor("#2F6FED")
CYAN = colors.HexColor("#24A6C8")
LIGHT_BLUE = colors.HexColor("#EAF2FF")
PALE = colors.HexColor("#F5F8FC")
BORDER = colors.HexColor("#C9D5E5")
ORANGE = colors.HexColor("#F59E0B")
WHITE = colors.white

FONT_REGULAR = "NotoSansSC"
FONT_BOLD = "NotoSansSC-Bold"


def register_fonts() -> None:
    global FONT_REGULAR, FONT_BOLD
    font_dir = Path(__file__).resolve().parent / "fonts"
    regular = font_dir / "NotoSansSC-Regular.ttf"
    bold = font_dir / "NotoSansSC-Bold.ttf"
    if regular.exists() and bold.exists():
        pdfmetrics.registerFont(TTFont(FONT_REGULAR, str(regular)))
        pdfmetrics.registerFont(TTFont(FONT_BOLD, str(bold)))
        pdfmetrics.registerFontFamily(
            FONT_REGULAR,
            normal=FONT_REGULAR,
            bold=FONT_BOLD,
            italic=FONT_REGULAR,
            boldItalic=FONT_BOLD,
        )
        return

    # 受限环境中可回退到系统字体；优先使用随仓库提供的 Noto 字体，确保 PDF 自包含。
    pdfmetrics.registerFont(UnicodeCIDFont("STSong-Light"))
    FONT_REGULAR = "STSong-Light"
    FONT_BOLD = "STSong-Light"
    pdfmetrics.registerFontFamily(
        FONT_REGULAR,
        normal="STSong-Light",
        bold="STSong-Light",
        italic="STSong-Light",
        boldItalic="STSong-Light",
    )


register_fonts()


def inline_markup(value: str) -> str:
    """转义 Markdown 文本，仅保留少量安全的行内格式。"""
    escaped = html.escape(value.strip(), quote=True)
    escaped = re.sub(
        r"\[([^\]]+)\]\(([^)]+)\)",
        lambda match: f'<link href="{match.group(2)}" color="#2F6FED">{match.group(1)}</link>',
        escaped,
    )
    escaped = re.sub(r"\*\*([^*]+)\*\*", r"<b>\1</b>", escaped)
    escaped = re.sub(
        r"`([^`]+)`",
        r'<font color="#9A3412">\1</font>',
        escaped,
    )
    return escaped


def clean_label(value: str) -> str:
    value = value.strip().strip('"').strip()
    return re.sub(r"\s+", " ", value)


class MermaidDiagram(Flowable):
    """将 Mermaid 流程、状态和时序关系渲染为矢量关系行。"""

    def __init__(self, source: str, width: float, font_name: str = FONT_REGULAR) -> None:
        super().__init__()
        self.source = source
        self.width = width
        self.font_name = font_name
        self.kind, self.rows = self._parse(source)
        self.row_height = 31
        self.title_height = 26
        self.height = self.title_height + max(1, len(self.rows)) * self.row_height + 12

    @staticmethod
    def _parse(source: str) -> tuple[str, list[tuple[str, str, str]]]:
        lines = [line.strip() for line in source.splitlines() if line.strip()]
        is_sequence = bool(lines and lines[0].startswith("sequenceDiagram"))
        kind = "时序关系" if is_sequence else "组件关系"
        labels: dict[str, str] = {}
        rows: list[tuple[str, str, str]] = []

        if is_sequence:
            for line in lines[1:]:
                participant = re.match(r"participant\s+(\w+)\s+as\s+(.+)", line)
                if participant:
                    labels[participant.group(1)] = clean_label(participant.group(2))
                    continue
                message = re.match(r"(\w+)\s*[-.x>]+\s*(\w+)\s*:\s*(.+)", line)
                if message:
                    rows.append(
                        (
                            labels.get(message.group(1), message.group(1)),
                            labels.get(message.group(2), message.group(2)),
                            clean_label(message.group(3)),
                        )
                    )
            return kind, rows[:16]

        node_pattern = re.compile(
            r"(?:^|\s)([A-Za-z][A-Za-z0-9_]*)\s*(?:\[\"([^\"]+)\"\]|\[([^\]]+)\]|\{\"?([^}\"]+)\"?\})"
        )
        for line in lines[1:]:
            for match in node_pattern.finditer(line):
                labels[match.group(1)] = clean_label(next(value for value in match.groups()[1:] if value))

        edge_pattern = re.compile(
            r"([A-Za-z][A-Za-z0-9_]*)\s*(?:-->|<-->|-.->|--x|--\|[^|]+\|-->)\s*(?:\|([^|]+)\|\s*)?([A-Za-z][A-Za-z0-9_]*)"
        )
        for line in lines[1:]:
            for match in edge_pattern.finditer(line):
                rows.append(
                    (
                        labels.get(match.group(1), match.group(1)),
                        labels.get(match.group(3), match.group(3)),
                        clean_label(match.group(2) or "数据/控制流"),
                    )
                )

        if not rows:
            ordered = list(dict.fromkeys(labels.values()))
            rows = [(ordered[index], ordered[index + 1], "关联") for index in range(len(ordered) - 1)]
        return kind, rows[:16]

    def wrap(self, avail_width: float, avail_height: float) -> tuple[float, float]:
        self.width = min(self.width, avail_width)
        return self.width, self.height

    def draw(self) -> None:
        canvas = self.canv
        canvas.saveState()
        canvas.setFillColor(PALE)
        canvas.setStrokeColor(BORDER)
        canvas.roundRect(0, 0, self.width, self.height, 7, fill=1, stroke=1)

        canvas.setFillColor(NAVY)
        canvas.setFont(self.font_name, 10)
        canvas.drawString(12, self.height - 17, f"图：{self.kind}（由 Mermaid 源自动生成）")

        y = self.height - self.title_height - 24
        box_width = (self.width - 82) * 0.34
        middle_width = self.width - 2 * box_width - 42
        for index, (left, right, message) in enumerate(self.rows, start=1):
            canvas.setFillColor(WHITE)
            canvas.setStrokeColor(colors.HexColor("#9DB6D3"))
            canvas.roundRect(10, y - 5, box_width, 22, 5, fill=1, stroke=1)
            canvas.roundRect(self.width - box_width - 10, y - 5, box_width, 22, 5, fill=1, stroke=1)

            canvas.setFillColor(INK)
            canvas.setFont(self.font_name, 7.4)
            canvas.drawCentredString(10 + box_width / 2, y + 3, self._fit(left, 22))
            canvas.drawCentredString(self.width - 10 - box_width / 2, y + 3, self._fit(right, 22))

            start_x = 14 + box_width
            end_x = self.width - box_width - 14
            canvas.setStrokeColor(BLUE)
            canvas.setFillColor(BLUE)
            canvas.setLineWidth(1.2)
            canvas.line(start_x, y + 6, end_x, y + 6)
            canvas.line(end_x, y + 6, end_x - 5, y + 9)
            canvas.line(end_x, y + 6, end_x - 5, y + 3)
            canvas.setFillColor(MUTED)
            canvas.setFont(self.font_name, 6.6)
            canvas.drawCentredString(
                start_x + middle_width / 2,
                y + 10,
                self._fit(f"{index}. {message}", 26),
            )
            y -= self.row_height

        if not self.rows:
            canvas.setFillColor(MUTED)
            canvas.drawString(12, 12, "该 Mermaid block 未包含可解析关系；请检查源格式。")
        canvas.restoreState()

    @staticmethod
    def _fit(value: str, maximum: int) -> str:
        return value if len(value) <= maximum else value[: maximum - 1] + "…"


class WhitepaperDocTemplate(BaseDocTemplate):
    def __init__(self, filename: str, **kwargs: object) -> None:
        super().__init__(filename, **kwargs)
        frame = Frame(
            MARGIN_X,
            MARGIN_BOTTOM,
            CONTENT_WIDTH,
            PAGE_HEIGHT - MARGIN_TOP - MARGIN_BOTTOM,
            leftPadding=0,
            rightPadding=0,
            topPadding=7 * mm,
            bottomPadding=6 * mm,
            id="content",
        )
        self.addPageTemplates(
            [
                PageTemplate(id="content", frames=[frame], onPage=self._page_decorations),
            ]
        )
        self._heading_counter = 0

    def beforeDocument(self) -> None:
        # multiBuild 会重复排版直到目录稳定，因此每轮 Bookmark ID 必须一致。
        self._heading_counter = 0
        super().beforeDocument()

    def afterFlowable(self, flowable: Flowable) -> None:
        if not isinstance(flowable, Paragraph):
            return
        level = getattr(flowable, "_toc_level", None)
        if level is None:
            return
        self._heading_counter += 1
        key = f"heading-{self._heading_counter}"
        self.canv.bookmarkPage(key)
        # 白皮书在第一章前以 H2 阅读摘要开头。PDF 大纲不能从 -1 级跳到 1 级，
        # 因此仅提升第一项的大纲级别，目录语义级别保持不变。
        outline_level = 0 if flowable.getPlainText() == "阅读摘要" and level > 0 else level
        self.canv.addOutlineEntry(
            flowable.getPlainText(), key, level=outline_level, closed=outline_level > 0
        )
        self.notify("TOCEntry", (level, flowable.getPlainText(), self.page, key))

    @staticmethod
    def _page_decorations(canvas: Canvas, doc: BaseDocTemplate) -> None:
        page = canvas.getPageNumber()
        if page == 1:
            return
        canvas.saveState()
        canvas.setStrokeColor(BORDER)
        canvas.setLineWidth(0.5)
        canvas.line(MARGIN_X, PAGE_HEIGHT - 14 * mm, PAGE_WIDTH - MARGIN_X, PAGE_HEIGHT - 14 * mm)
        canvas.setFont(FONT_REGULAR, 7.5)
        canvas.setFillColor(MUTED)
        canvas.drawString(MARGIN_X, PAGE_HEIGHT - 11 * mm, "hello-k8s-ai · 完整技术总览")
        canvas.drawRightString(PAGE_WIDTH - MARGIN_X, PAGE_HEIGHT - 11 * mm, "文档基线 2026-08-13")
        canvas.line(MARGIN_X, 12 * mm, PAGE_WIDTH - MARGIN_X, 12 * mm)
        canvas.drawString(MARGIN_X, 8 * mm, "Markdown 单一内容源 · 当前集群运行态未在本交接环境验证")
        canvas.drawRightString(PAGE_WIDTH - MARGIN_X, 8 * mm, f"{page}")
        canvas.restoreState()


def build_styles() -> dict[str, ParagraphStyle]:
    sample = getSampleStyleSheet()
    base = ParagraphStyle(
        "BaseCJK",
        parent=sample["BodyText"],
        fontName=FONT_REGULAR,
        fontSize=9.2,
        leading=15.2,
        textColor=INK,
        spaceAfter=5,
        wordWrap="CJK",
        splitLongWords=True,
        allowWidows=0,
        allowOrphans=0,
    )
    return {
        "body": base,
        "h1": ParagraphStyle(
            "H1CJK",
            parent=base,
            fontSize=19,
            leading=25,
            textColor=NAVY,
            spaceBefore=2,
            spaceAfter=10,
            keepWithNext=True,
        ),
        "h2": ParagraphStyle(
            "H2CJK",
            parent=base,
            fontSize=13.5,
            leading=19,
            textColor=BLUE,
            spaceBefore=10,
            spaceAfter=5,
            keepWithNext=True,
        ),
        "h3": ParagraphStyle(
            "H3CJK",
            parent=base,
            fontSize=11,
            leading=16,
            textColor=CYAN,
            spaceBefore=7,
            spaceAfter=4,
            keepWithNext=True,
        ),
        "bullet": ParagraphStyle(
            "BulletCJK",
            parent=base,
            leftIndent=13,
            firstLineIndent=-8,
            spaceAfter=3,
        ),
        "quote": ParagraphStyle(
            "QuoteCJK",
            parent=base,
            textColor=NAVY,
            leftIndent=4,
            rightIndent=4,
            spaceAfter=0,
        ),
        "code": ParagraphStyle(
            "CodeCJK",
            parent=base,
            fontSize=7.4,
            leading=10.2,
            textColor=colors.HexColor("#233043"),
            leftIndent=5,
            rightIndent=5,
        ),
        "caption": ParagraphStyle(
            "CaptionCJK",
            parent=base,
            fontSize=7.5,
            leading=10,
            textColor=MUTED,
            alignment=TA_CENTER,
        ),
        "toc1": ParagraphStyle(
            "TOC1CJK",
            parent=base,
            fontSize=10.5,
            leading=15,
            textColor=NAVY,
            leftIndent=0,
            firstLineIndent=0,
            spaceBefore=4,
        ),
        "toc2": ParagraphStyle(
            "TOC2CJK",
            parent=base,
            fontSize=8.6,
            leading=12.5,
            textColor=MUTED,
            leftIndent=14,
            firstLineIndent=0,
        ),
    }


def cover_story(styles: dict[str, ParagraphStyle]) -> list[Flowable]:
    title = ParagraphStyle(
        "CoverTitle",
        parent=styles["body"],
        fontSize=28,
        leading=36,
        textColor=WHITE,
        alignment=TA_LEFT,
        fontName=FONT_BOLD,
    )
    subtitle = ParagraphStyle(
        "CoverSubtitle",
        parent=styles["body"],
        fontSize=13,
        leading=21,
        textColor=colors.HexColor("#DCEAFF"),
    )
    small = ParagraphStyle(
        "CoverSmall",
        parent=styles["body"],
        fontSize=9,
        leading=15,
        textColor=WHITE,
    )

    hero = Table(
        [
            [
                Paragraph("hello-k8s-ai", title),
            ],
            [Paragraph("Kubernetes 原生 AI 推理调度与仿真平台", subtitle)],
            [Paragraph("完整技术总览 · 工程交接白皮书", subtitle)],
            [Spacer(1, 12 * mm)],
            [
                Paragraph(
                    "从用户操作、React、Dashboard Backend、Kubernetes CRD/Controller、Simulator，"
                    "到 Prometheus、OpenTelemetry、Jaeger、PostgreSQL 与可视化的完整闭环。",
                    small,
                )
            ],
            [Spacer(1, 19 * mm)],
            [Paragraph("文档基线：2026-08-13", small)],
            [Paragraph("正文源：docs/whitepaper/COMPLETE_OVERVIEW.md", small)],
        ],
        colWidths=[CONTENT_WIDTH],
        rowHeights=[None] * 8,
        style=TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, -1), NAVY),
                ("BOX", (0, 0), (-1, -1), 0, NAVY),
                ("LEFTPADDING", (0, 0), (-1, -1), 18 * mm),
                ("RIGHTPADDING", (0, 0), (-1, -1), 18 * mm),
                ("TOPPADDING", (0, 0), (-1, 0), 25 * mm),
                ("BOTTOMPADDING", (0, -1), (-1, -1), 25 * mm),
            ]
        ),
    )

    pipeline_cells = []
    for label in ["React", "Backend", "Kubernetes", "Controllers", "Simulator", "Observability"]:
        pipeline_cells.append(Paragraph(label, styles["caption"]))
    pipeline = Table(
        [pipeline_cells],
        colWidths=[CONTENT_WIDTH / 6] * 6,
        style=TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, -1), LIGHT_BLUE),
                ("BOX", (0, 0), (-1, -1), 0.6, BORDER),
                ("INNERGRID", (0, 0), (-1, -1), 0.4, BORDER),
                ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
                ("TOPPADDING", (0, 0), (-1, -1), 6),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 6),
            ]
        ),
    )
    return [Spacer(1, 14 * mm), hero, Spacer(1, 12 * mm), pipeline, PageBreak()]


def parse_table(lines: list[str], index: int, styles: dict[str, ParagraphStyle]) -> tuple[Table, int]:
    raw_rows: list[list[str]] = []
    while index < len(lines) and lines[index].strip().startswith("|"):
        raw_rows.append([cell.strip() for cell in lines[index].strip().strip("|").split("|")])
        index += 1
    if len(raw_rows) >= 2 and all(re.fullmatch(r":?-{3,}:?", cell.replace(" ", "")) for cell in raw_rows[1]):
        del raw_rows[1]
    columns = max(len(row) for row in raw_rows)
    raw_rows = [row + [""] * (columns - len(row)) for row in raw_rows]

    font_size = 7.1 if columns >= 5 else 7.7
    leading = 9.6 if columns >= 5 else 10.5
    cell_style = ParagraphStyle(
        "TableCell",
        parent=styles["body"],
        fontSize=font_size,
        leading=leading,
        spaceAfter=0,
        wordWrap="CJK",
    )
    head_style = ParagraphStyle(
        "TableHead",
        parent=cell_style,
        textColor=WHITE,
        alignment=TA_LEFT,
        fontName=FONT_BOLD,
    )
    data = [
        [Paragraph(inline_markup(cell), head_style if row_index == 0 else cell_style) for cell in row]
        for row_index, row in enumerate(raw_rows)
    ]

    scores: list[float] = []
    for column in range(columns):
        maximum = max(len(re.sub(r"[`*\[\]()]", "", row[column])) for row in raw_rows)
        scores.append(min(34.0, max(7.5, maximum ** 0.72)))
    total = sum(scores)
    widths = [CONTENT_WIDTH * score / total for score in scores]

    table = Table(data, colWidths=widths, repeatRows=1, hAlign="LEFT")
    table.setStyle(
        TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, 0), NAVY),
                ("TEXTCOLOR", (0, 0), (-1, 0), WHITE),
                ("BACKGROUND", (0, 1), (-1, -1), WHITE),
                ("ROWBACKGROUNDS", (0, 1), (-1, -1), [WHITE, PALE]),
                ("GRID", (0, 0), (-1, -1), 0.35, BORDER),
                ("VALIGN", (0, 0), (-1, -1), "TOP"),
                ("LEFTPADDING", (0, 0), (-1, -1), 4),
                ("RIGHTPADDING", (0, 0), (-1, -1), 4),
                ("TOPPADDING", (0, 0), (-1, -1), 4),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 4),
            ]
        )
    )
    return table, index


def code_panel(source: str, language: str, styles: dict[str, ParagraphStyle]) -> Table:
    # Preformatted 按字面绘制文本，不解析 ReportLab 段落标记；此处转义会显示 ``-&gt;`` 等实体。
    pre = Preformatted(source.rstrip(), styles["code"], maxLineLength=105)
    label = language or "text"
    return Table(
        [[Paragraph(f"<b>{html.escape(label)}</b>", styles["caption"])], [pre]],
        colWidths=[CONTENT_WIDTH],
        style=TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#DCE7F5")),
                ("BACKGROUND", (0, 1), (-1, -1), colors.HexColor("#F7F9FC")),
                ("BOX", (0, 0), (-1, -1), 0.5, BORDER),
                ("LEFTPADDING", (0, 0), (-1, -1), 7),
                ("RIGHTPADDING", (0, 0), (-1, -1), 7),
                ("TOPPADDING", (0, 0), (-1, -1), 4),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 4),
            ]
        ),
    )


def quote_panel(text: str, styles: dict[str, ParagraphStyle]) -> Table:
    return Table(
        [[Paragraph(inline_markup(text), styles["quote"])]],
        colWidths=[CONTENT_WIDTH],
        style=TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, -1), LIGHT_BLUE),
                ("LINEBEFORE", (0, 0), (0, -1), 4, BLUE),
                ("BOX", (0, 0), (-1, -1), 0.3, BORDER),
                ("LEFTPADDING", (0, 0), (-1, -1), 10),
                ("RIGHTPADDING", (0, 0), (-1, -1), 9),
                ("TOPPADDING", (0, 0), (-1, -1), 7),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 7),
            ]
        ),
    )


def markdown_story(source: str, styles: dict[str, ParagraphStyle]) -> list[Flowable]:
    lines = source.splitlines()
    story: list[Flowable] = []
    index = 0
    skipped_document_title = False

    while index < len(lines):
        line = lines[index]
        stripped = line.strip()

        if not stripped:
            index += 1
            continue

        if stripped.startswith("```"):
            language = stripped[3:].strip()
            index += 1
            block: list[str] = []
            while index < len(lines) and not lines[index].strip().startswith("```"):
                block.append(lines[index])
                index += 1
            index += 1
            text = "\n".join(block)
            story.append(Spacer(1, 3))
            if language == "mermaid":
                story.append(MermaidDiagram(text, CONTENT_WIDTH))
            else:
                # 语言 Label 与代码正文是一个视觉单元，保持同页可避免 Label 单独落在页尾。
                story.append(KeepTogether([code_panel(text, language, styles)]))
            story.append(Spacer(1, 6))
            continue

        heading = re.match(r"^(#{1,3})\s+(.+)$", stripped)
        if heading:
            level = len(heading.group(1))
            text = heading.group(2).strip()
            if level == 1 and not skipped_document_title:
                skipped_document_title = True
                index += 1
                continue
            if level == 1:
                if story and not isinstance(story[-1], PageBreak):
                    story.append(PageBreak())
                style_key = "h1"
                toc_level = 0
            elif level == 2:
                style_key = "h2"
                toc_level = 1
            else:
                style_key = "h3"
                toc_level = 2
            paragraph = Paragraph(inline_markup(text), styles[style_key])
            paragraph._toc_level = toc_level  # type: ignore[attr-defined]
            story.append(paragraph)
            index += 1
            continue

        if stripped.startswith("|") and index + 1 < len(lines) and lines[index + 1].strip().startswith("|"):
            table, index = parse_table(lines, index, styles)
            story.extend([Spacer(1, 3), table, Spacer(1, 7)])
            continue

        if stripped.startswith(">"):
            quote_lines: list[str] = []
            while index < len(lines) and lines[index].strip().startswith(">"):
                quote_lines.append(lines[index].strip().lstrip(">").strip())
                index += 1
            story.extend([quote_panel(" ".join(quote_lines), styles), Spacer(1, 7)])
            continue

        bullet = re.match(r"^[-*]\s+(.+)$", stripped)
        number = re.match(r"^(\d+)\.\s+(.+)$", stripped)
        if bullet or number:
            items: list[Flowable] = []
            ordered = bool(number)
            counter = 1
            while index < len(lines):
                current = lines[index].strip()
                match = re.match(r"^(\d+)\.\s+(.+)$", current) if ordered else re.match(r"^[-*]\s+(.+)$", current)
                if not match:
                    break
                content = match.group(2) if ordered else match.group(1)
                marker = f"{match.group(1)}." if ordered else "•"
                items.append(Paragraph(f"{marker} {inline_markup(content)}", styles["bullet"]))
                index += 1
                counter += 1
            story.append(KeepTogether(items) if len(items) <= 4 else items[0])
            if len(items) > 4:
                story.extend(items[1:])
            story.append(Spacer(1, 3))
            continue

        paragraph_lines = [stripped]
        index += 1
        while index < len(lines):
            nxt = lines[index].strip()
            if not nxt:
                index += 1
                break
            if (
                nxt.startswith("#")
                or nxt.startswith("```")
                or nxt.startswith(">")
                or nxt.startswith("|")
                or re.match(r"^[-*]\s+", nxt)
                or re.match(r"^\d+\.\s+", nxt)
            ):
                break
            paragraph_lines.append(nxt)
            index += 1
        story.append(Paragraph(inline_markup(" ".join(paragraph_lines)), styles["body"]))

    return story


def make_toc(styles: dict[str, ParagraphStyle]) -> list[Flowable]:
    heading = Paragraph("目录", styles["h1"])
    heading._toc_level = None  # type: ignore[attr-defined]
    toc = TableOfContents()
    toc.levelStyles = [styles["toc1"], styles["toc2"], styles["toc2"]]
    toc.dotsMinLevel = 0
    return [heading, Spacer(1, 4), toc, PageBreak()]


def build_pdf(source_path: Path, output_path: Path) -> None:
    source = source_path.read_text(encoding="utf-8")
    styles = build_styles()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    doc = WhitepaperDocTemplate(
        str(output_path),
        pagesize=A4,
        leftMargin=MARGIN_X,
        rightMargin=MARGIN_X,
        topMargin=MARGIN_TOP,
        bottomMargin=MARGIN_BOTTOM,
        title="hello-k8s-ai 完整技术总览",
        author="hello-k8s-ai 项目文档工程",
        subject="Kubernetes 原生 AI 推理调度与仿真平台技术白皮书",
        creator="Markdown -> ReportLab 单一内容源文档管线",
    )
    story = cover_story(styles) + make_toc(styles) + markdown_story(source, styles)
    doc.multiBuild(story)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    build_pdf(args.source.resolve(), args.output.resolve())


if __name__ == "__main__":
    main()
