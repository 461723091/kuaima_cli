---
name: kuaima-gpt-image-2
description: kuaima_cli image + recharge skill for Linux.
---

# kuaima-gpt-image-2

Linux package.

## Prereqs

- Bash scripts.
- Default image model: `gpt-image-2`.

## Image

```bash
bash "<技能目录>/scripts/kuaima-image.sh" \
  --prompt "a clean product render of a white coffee mug" \
  --output-dir "./outputs"
```

## Balance / Recharge

```bash
"<技能目录>/bin/kuaima_cli-linux-arm64" balance
"<技能目录>/bin/kuaima_cli-linux-amd64" balance
bash "<技能目录>/scripts/kuaima-recharge.sh" --print-url
bash "<技能目录>/scripts/kuaima-recharge.sh" --amount 100
bash "<技能目录>/scripts/kuaima-recharge.sh" --plan-id 2
```
