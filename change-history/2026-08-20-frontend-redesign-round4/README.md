# 前端重构第四轮：设计打磨 + 配置样例数据 + 模板加载真实 bug（#101）

> 日期：2026-08-20 ｜ 关联：issue #101、project-review/issue-11-config-form-template-load-reset.md、独立重构副本 /root/frontend-redesign

## 为什么做

- 前端功能主体闭环后进入设计打磨：全局字号、信息密度与页面美感需要整体提升；用户要求"独立副本先行、验证后合入"，避免大改直接动主仓库。
- 本轮在 fixtures 录制 + mock-server 预览链路上完成第 4 轮重构（副本 4 个本地 commit），并在打磨中发现一个真实功能 bug：#101。

## 改成什么

1. 字号与监控页（cc32a86）：全局字号 10px 起步 → 12px 起步，小标签统一 12/13px；Monitor 页改为指标墙（5 张指标卡 + 状态筛选 + Grafana 折叠区）；CollapsibleSection 抽为公共组件，Trace 工作台复用 metricStats / aggregateMetricPoints 共享工具。
2. 配置样例数据（613b2a1）：mock-server 对 /configuration 做"录制快照打底 + dev-fixtures/configuration-dev.json 补齐空数组"（3 模型 / 4 节点 / 3 租户 / 2 编排 / 5 策略），meta.devSamples=true 标注非录制，仅用于前端预览。
3. Config 实时影响（2b45d9c）：新增 LiveImpactSummary，四个资源表单顶部实时展示当前参数变更影响；节点表新增 GPU 用量 mini 条（usedGPU / 总量 + 阈值绿黄红变色）；dev-fixtures 节点样例补 status 运行时数据。
4. Guide 双栏（597c892）：左侧 sticky 目录 + 锚点滚动 + 激活态 + 章节编号徽章（01–09）；ConfigFormSection 支持可选 id 与 scroll-mt。

## 关键行为

- 重构只在独立副本 /root/frontend-redesign 进行，主仓库 dashboard/frontend/my-app 零改动；副本 4 个 commit 均为本地提交，未推送。
- #101（模板加载被重置）：useConfigForm 内 useEffect(() => form.reset(defaultValues), [defaultValues, form]) 会覆盖"模板库 → 加载模板"的 form.reset(template.data)。defaultValues 每次父重渲染都是新引用：加载模板 → 表单变脏 → 父重渲染 → effect 后执行，把表单打回已保存值。表单已由 key={selectedItem.name} 保证重挂载，该监听冗余且有害。修复：删除该 effect（保留 setSubmitError('')）并移除 defaultValues 参数；副本实测 gpuUnits 800→8 生效。
- 修复代码目前只存在于重构副本，随重构 PR 合入主仓库时一并落地；主仓库合入前仍带该 bug，由 issue #101 跟踪。
- mock-server 的 dev-fixtures 合并逻辑属于副本开发基础设施，是否并入主仓库待用户拍板。

## 验证

- 每个 commit 后 npm run check 通过；浏览器实测：模板加载 gpuUnits 800→8、LiveImpactSummary 实时联动、Guide 锚点滚动、Monitor 指标墙渲染正常。
- 本轮纯前端 + 预览基建，未触碰控制面 / API / 集群；主仓库无代码改动，无 CI 影响。
- 未附 before/after 截图：重构未合入主仓库，视觉验收以浏览器实测为准。

## 回滚

- 主仓库无改动，无回滚面；删除副本 /root/frontend-redesign 即整体撤销，4 个 commit 保留在副本 git 历史，可供后续 PR 拣选。
- #101 审查记录已入库（commit 0bec1ef），PR 合入时作为修复依据。
