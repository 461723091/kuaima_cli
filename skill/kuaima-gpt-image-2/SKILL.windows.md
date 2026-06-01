---
name: kuaima-gpt-image-2
description: 一个通过内置 kuaima_cli 调用 gpt-image-2 的生图工具，支持文生图、参考图改图、多参考图融合、蒙版局部编辑，并把生成图片保存到本地。在需要生成或编辑图片时调用。
---

# 快马 GPT Image 2 生图工具

这是 Windows 版技能说明。需要文生图、参考图改图、多参考图融合、蒙版局部编辑，或执行充值/余额查询时，调用这里对应的 PowerShell 封装脚本。

## 使用前提

- 仅适用于 Windows，脚本使用 PowerShell。
- 图片输出到 `-OutputDir` 指定目录，默认是 `.\outputs`。
- 默认固定使用 `gpt-image-2`。
- 调试失败时再加 `-VerboseCli`。

## 文生图

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-image.ps1" `
  -Prompt "一张白色咖啡杯的干净产品渲染图，中性摄影棚背景，柔和布光" `
  -OutputDir ".\outputs"
```

## 参考图改图

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-image.ps1" `
  -Prompt "保留主体结构，把材质改成磨砂陶瓷，背景改为浅灰色" `
  -File ".\reference.png" `
  -OutputDir ".\outputs"
```

## 多参考图融合

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-image.ps1" `
  -Prompt "融合两张参考图的造型语言，输出一个更简洁的产品概念图" `
  -File ".\ref-a.png" `
  -File ".\ref-b.png" `
  -OutputDir ".\outputs"
```

## 蒙版局部编辑

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-image.ps1" `
  -Prompt "只替换蒙版区域，补上一个陶瓷手柄，其他区域保持不变" `
  -File ".\reference.png" `
  -Mask ".\mask.png" `
  -OutputDir ".\outputs"
```

## 常用参数

- 尺寸：`-Size`，例如 `auto`、`1024x1024`、`1536x1024`、`1024x1536` 或自定义 `WxH`
- 质量：`-Quality`，可选 `auto`、`low`、`medium`、`high`
- 数量：`-Count`，范围 `1..10`
- 格式：`-Format`，可选 `png`、`jpeg`、`webp`
- 压缩：`-Compression`，范围 `0..100`，用于 `jpeg` / `webp`
- 背景：`-Background`，可选 `auto`、`transparent`、`opaque`
- 审核：`-Moderation`，可选 `auto`、`low`
- 额外服务配置：`-ApiKey`、`-BaseUrl`

## 余额 / 充值

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -Command "<技能目录>\bin\kuaima_cli.exe balance"
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-recharge.ps1" -PrintOnly
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-recharge.ps1" -Amount 100
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-recharge.ps1" -PlanId 2
```

## 给智能体的执行规则

- 把用户需求整理成明确视觉提示词：主体、构图、风格、光线、背景、画幅、材质、文字限制和不要出现的元素。
- 每个任务使用单独输出目录，完成后报告生成图片的本地路径。
- 不要在最终回复里暴露 API key。
