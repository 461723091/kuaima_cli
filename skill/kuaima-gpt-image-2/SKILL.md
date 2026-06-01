---
name: kuaima-gpt-image-2
description: 一个通过内置 kuaima_cli 调用 gpt-image-2 的生图工具，支持文生图、参考图改图、多参考图融合、蒙版局部编辑，并把生成图片保存到本地。在需要生成或编辑图片时调用。
---

# 快马 GPT Image 2 生图工具

这是一个面向智能体的生图工具。需要画图、改图、根据参考图生成新图、或对蒙版区域做局部编辑时，优先调用本技能内置的包装脚本。

## 使用前提

- Windows 使用 PowerShell 脚本，macOS 使用 bash 脚本。
- 输出图片会保存到 `OutputDir` / `--output-dir` 指定目录。
- 默认固定使用 `gpt-image-2`；如服务端后续支持该模型的新尺寸或新参数，优先透传给 CLI。

## Windows 文生图

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-image.ps1" `
  -Prompt "一张白色咖啡杯的干净产品渲染图，中性摄影棚背景，柔和布光" `
  -OutputDir ".\outputs"
```

## macOS 文生图

```bash
bash "<技能目录>/scripts/kuaima-image.sh" \
  --prompt "一张白色咖啡杯的干净产品渲染图，中性摄影棚背景，柔和布光" \
  --output-dir "./outputs"
```

## 参考图改图

Windows:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-image.ps1" `
  -Prompt "保留主体结构，把材质改成磨砂陶瓷，背景改为浅灰色" `
  -File ".\reference.png" `
  -OutputDir ".\outputs"
```

macOS:

```bash
bash "<技能目录>/scripts/kuaima-image.sh" \
  --prompt "保留主体结构，把材质改成磨砂陶瓷，背景改为浅灰色" \
  --file "./reference.png" \
  --output-dir "./outputs"
```

## 蒙版局部编辑

Windows:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-image.ps1" `
  -Prompt "只替换蒙版区域，补上一个陶瓷手柄，其他区域保持不变" `
  -File ".\reference.png" `
  -Mask ".\mask.png" `
  -OutputDir ".\outputs"
```

macOS:

```bash
bash "<技能目录>/scripts/kuaima-image.sh" \
  --prompt "只替换蒙版区域，补上一个陶瓷手柄，其他区域保持不变" \
  --file "./reference.png" \
  --mask "./mask.png" \
  --output-dir "./outputs"
```

## 常用参数

- 尺寸：Windows `-Size`，macOS `--size`，例如 `auto`、`1024x1024`、`1536x1024`、`1024x1536` 或自定义 `WxH`。
- 质量：Windows `-Quality`，macOS `--quality`，可选 `auto`、`low`、`medium`、`high`。
- 数量：Windows `-Count`，macOS `--count`，范围 `1..10`。
- 格式：Windows `-Format`，macOS `--format`，可选 `png`、`jpeg`、`webp`。
- 压缩：Windows `-Compression`，macOS `--compression`，范围 `0..100`，用于 `jpeg` / `webp`。
- 背景：Windows `-Background`，macOS `--background`，可选 `auto`、`transparent`、`opaque`。
- 审核：Windows `-Moderation`，macOS `--moderation`，可选 `auto`、`low`。

## 给智能体的执行规则

- 把用户需求转换成具体视觉提示词：主体、构图、风格、光线、背景、画幅、材质、文字限制和不要出现的元素。
- 每个任务使用单独输出目录，完成后报告生成图片的本地路径。
- 不要在最终回复里暴露 API key。
- 调试失败时再开启详细日志：Windows 使用 `-VerboseCli`，macOS 使用 `--verbose-cli`。
