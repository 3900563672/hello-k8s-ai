# 变更总览：项目采用 Apache-2.0 许可证

> 日期：2026-08-18 ｜ 级别：P2 ｜ 对应 Issue：无（仓库健康度治理的收尾项）

## 为什么做

- 公开仓库此前没有 LICENSE，第三方无法合法复用代码，也不利于开源协作履历。
- 用户授权选择推荐方案：Apache-2.0（Kubernetes 生态惯例，含专利授权，对企业合规友好）。

## 改成什么

- 新增根目录 `LICENSE`（Apache-2.0 标准文本，来自 apache.org 官方）。
- README 增加许可证引用。
- GitHub 会自动识别 LICENSE 并在仓库页展示许可证徽章。

## 关键行为

- Apache-2.0 允许商用、修改、分发，附带专利授权与署名要求。
- 后续如修改许可证需用户明确决策，不得自行更换。

## 验证

- `make docs-check`：全绿（LICENSE 非 Markdown，不涉及白名单）。
- GitHub 仓库页 license 徽章以推送后实际识别结果为准。

## 回滚

- 删除 LICENSE 并 revert README 引用行即可。
