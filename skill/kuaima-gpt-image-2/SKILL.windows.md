---
name: kuaima-gpt-image-2
description: kuaima_cli image + recharge skill for Windows.
---

# kuaima-gpt-image-2

Windows package.

## Prereqs

- PowerShell scripts.
- Default image model: `gpt-image-2`.

## Image

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-image.ps1" `
  -Prompt "a clean product render of a white coffee mug" `
  -OutputDir ".\outputs"
```

## Balance / Recharge

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -Command "<技能目录>\bin\kuaima_cli.exe balance"
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-recharge.ps1" -PrintOnly
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-recharge.ps1" -Amount 100
powershell -NoProfile -ExecutionPolicy Bypass -File "<技能目录>\scripts\kuaima-recharge.ps1" -PlanId 2
```
