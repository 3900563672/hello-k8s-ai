# 从 Markdown 构建 PDF

> 维护层：human | last-reviewed：2026-08-18 | 事实源：docs/MAP.yaml、源码、change-history/

## 单一内容源

`COMPLETE_OVERVIEW.md` 是 `hello-k8s-ai-complete-overview.pdf` 的唯一正文源。不要直接编辑 PDF，也不要在脚本里维护另一套正文。

## 构建

在文档交接目录根运行：

```bash
$CODEX_PRIMARY_RUNTIME_PYTHON tools/build_overview_pdf.py \
  --source docs/whitepaper/COMPLETE_OVERVIEW.md \
  --output .runtime/hello-k8s-ai-complete-overview.pdf
```

若没有 Work Mode runtime，安装 Python 3.12+ 与 ReportLab 4，并将命令中的解释器替换为 `python3`。

脚本：

- 解析 Heading、段落、列表、表格、代码块和 Mermaid block。
- 生成封面、目录、页眉/页脚、章节书签和关系图。
- 写入 PDF metadata。
- 嵌入 `tools/fonts/` 中的 Noto Sans SC Regular/Bold；交付包包含 SIL Open Font License，PDF 不依赖阅读机器的系统字体。

## 验收

```bash
pdfinfo .runtime/hello-k8s-ai-complete-overview.pdf
pdftoppm -png -r 120 .runtime/hello-k8s-ai-complete-overview.pdf /tmp/hello-k8s-ai-overview
```

逐页检查：

- 中文无方框/缺字，代码/英文不乱码。
- 无截断、重叠、表格越界、空白异常页。
- 目录页码、章节书签、页码正确。
- 图的节点/箭头与 Markdown Mermaid 一致。
- 表头重复，长表可以跨页。
- 文档状态和 Cluster Information 的“未验证”措辞保留。

还应随机提取文本，确认 PDF 可搜索。每次正文或生成器改变都重新渲染检查，不只比较文件是否生成成功。

## 发布位置

PDF 是构建产物，输出到 `.runtime/hello-k8s-ai-complete-overview.pdf`（`.runtime/` 已被 `.gitignore` 排除），不提交仓库。根 README 只链接 Markdown 源 `docs/whitepaper/COMPLETE_OVERVIEW.md`；需要交付 PDF 时从 `.runtime/` 单独取走。
