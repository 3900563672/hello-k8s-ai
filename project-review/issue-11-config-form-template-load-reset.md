# 1. 问题标题

配置详情表单内“模板库 → 加载模板”不生效：表单值被立即重置回已保存值

## 2. 当前状态描述

在任一配置资源（模型/节点/租户/编排策略）的详情表单中，点击“模板库”并选择一个模板点“加载模板”，弹窗会正常关闭，但表单各字段不会变成模板值——仍然显示选中资源的已保存值。用预置模板“轻量在线推理”（gpuUnits=8）实测：加载后 `gpuUnits` 仍为 800。

在 `/root/frontend-redesign`（前端重构独立副本，代码与主仓库一致）中已复现，并定位到根因（见第 5 节）。主仓库 `dashboard/frontend/my-app` 代码结构相同，`ConfigFormParts.tsx` 的 `useConfigForm` 与 `ConfigTabPanel.tsx` 的 `key={selectedItem.name}` 均一致，判定主仓库同样存在该问题。

## 3. 问题定位

`ConfigTabPanel.tsx` 渲染 `<FormComponent key={selectedItem.name} defaultValues={getFormValues(selectedItem)} ...>`——选中项切换时表单整体重挂载，`defaultValues` 天然带最新值。

但 `ConfigFormParts.tsx` 的 `useConfigForm` 里仍有：

```ts
useEffect(() => {
    form.reset(defaultValues)
    setSubmitError('')
}, [defaultValues, form])
```

`defaultValues` 是父组件每次渲染新构造的对象。加载模板 → `form.reset(template.data)` → 表单变脏 → `onDirtyChange(true)` → 父组件 `setFormDirty` 重渲染 → 新 `defaultValues` 引用 → 该 effect 触发 → `form.reset(已保存值)`，模板值被覆盖。

## 4. 影响范围

- Config 页全部四类详情表单（模型/节点/租户/编排策略）的“模板库 → 加载模板”功能不可用。
- 用户只能通过“从模板新建”流程（创建弹窗）使用模板，已保存资源的“套用模板快速修改”能力失效。
- 属于静默失败：弹窗关闭、无报错，用户以为加载成功，实际表单未变。

## 5. 根本原因分析

表单切换已由 `key={selectedItem.name}` 保证重挂载，`useEffect` 对 `defaultValues` 的监听是历史冗余逻辑，且与“表单内 reset（模板加载/模板文件加载）”互相竞争，后执行的 effect 总是把表单打回已保存值。

## 6. 修改方向建议

删除 `useConfigForm` 中对 `defaultValues` 的 reset 监听（保留 `setSubmitError('')` 初始化即可）。已在重构副本验证：删除后模板加载正常（gpuUnits 800→8、摘要实时更新），`npm run check` 通过，无回归。