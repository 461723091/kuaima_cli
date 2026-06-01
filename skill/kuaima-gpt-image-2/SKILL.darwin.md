---
name: kuaima-gpt-image-2
description: 一个通过内置 kuaima_cli 调用 gpt-image-2 的生图工具，支持文生图、参考图改图、多参考图融合、蒙版局部编辑，并把生成图片保存到本地。在需要生成或编辑图片时调用。
---

# 快马 GPT Image 2 生图工具

这是 macOS 版技能说明。需要文生图、参考图改图、多参考图融合、蒙版局部编辑，或执行充值/余额查询时，调用这里对应的 bash 封装脚本。

## 使用前提

- 仅适用于 macOS，脚本使用 bash。
- 图片输出到 `--output-dir` 指定目录，默认是 `./outputs`。
- 默认固定使用 `gpt-image-2`。
- 调试失败时再加 `--verbose-cli`。

## 文生图

```bash
bash "<技能目录>/scripts/kuaima-image.sh" \
  --prompt "一张白色咖啡杯的干净产品渲染图，中性摄影棚背景，柔和布光" \
  --output-dir "./outputs"
```

## 参考图改图

```bash
bash "<技能目录>/scripts/kuaima-image.sh" \
  --prompt "保留主体结构，把材质改成磨砂陶瓷，背景改为浅灰色" \
  --file "./reference.png" \
  --output-dir "./outputs"
```

## 多参考图融合

```bash
bash "<技能目录>/scripts/kuaima-image.sh" \
  --prompt "融合两张参考图的造型语言，输出一个更简洁的产品概念图" \
  --file "./ref-a.png" \
  --file "./ref-b.png" \
  --output-dir "./outputs"
```

## 蒙版局部编辑

```bash
bash "<技能目录>/scripts/kuaima-image.sh" \
  --prompt "只替换蒙版区域，补上一个陶瓷手柄，其他区域保持不变" \
  --file "./reference.png" \
  --mask "./mask.png" \
  --output-dir "./outputs"
```

## 常用参数

- 尺寸：`--size`，例如 `auto`、`1024x1024`、`1536x1024`、`1024x1536` 或自定义 `WxH`
- 质量：`--quality`，可选 `auto`、`low`、`medium`、`high`
- 数量：`--count`，范围 `1..10`
- 格式：`--format`，可选 `png`、`jpeg`、`webp`
- 压缩：`--compression`，范围 `0..100`，用于 `jpeg` / `webp`
- 背景：`--background`，可选 `auto`、`transparent`、`opaque`
- 审核：`--moderation`，可选 `auto`、`low`
- 额外服务配置：`--api-key`、`--base-url`

## 余额 / 充值

```bash
"<技能目录>/bin/kuaima_cli-darwin-arm64" balance
"<技能目录>/bin/kuaima_cli-darwin-amd64" balance
bash "<技能目录>/scripts/kuaima-recharge.sh" --print-url
bash "<技能目录>/scripts/kuaima-recharge.sh" --amount 100
bash "<技能目录>/scripts/kuaima-recharge.sh" --plan-id 2
```

## 给智能体的执行规则

- 把用户需求整理成明确视觉提示词：主体、构图、风格、光线、背景、画幅、材质、文字限制和不要出现的元素。
- 每个任务使用单独输出目录，完成后报告生成图片的本地路径。
- 不要在最终回复里暴露 API key。
