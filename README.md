# kuaima_cli

轻量级 Responses API 命令行客户端。默认 API 地址：

```text
https://ai.szkmjb.com
```

## 构建

```powershell
go build -buildvcs=false -o dist/kuaima_cli.exe .
```

## 配置

```powershell
$env:KUAIMA_API_KEY="your_api_key"
```

也可以使用 `OPENAI_API_KEY`。

也可以使用家目录下的配置文件，路径为 `~/.kuaima/conf.json`。配置文件优先级最低：命令行参数 > 环境变量 > 配置文件 > 内置默认值。

命令运行时会把本次显式传入的可持久化参数自动写回配置文件，包括：

```text
model
image_model
base_url
oss_url
api_key
system
stream
file_format
save_images
```

例如：

```json
{
  "model": "gpt-5.4-mini",
  "image_model": "gpt-image-1",
  "base_url": "https://ai.szkmjb.com",
  "oss_url": "https://oss.szkmjb.com",
  "api_key": "your_api_key",
  "stream": true,
  "file_format": "base64",
  "save_images": "."
}
```

## 使用

直接输入提示词并打印返回结果：

```powershell
.\dist\kuaima_cli.exe "用一句话解释量子纠缠"
```

从标准输入读取提示词：

```powershell
Get-Content prompt.txt | .\dist\kuaima_cli.exe
```

输入本地文本、图片或图片 URL，`-file` 可以重复；提示词里的 `@路径` 也会按文件类型自动处理：

```powershell
.\dist\kuaima_cli.exe -file .\notes.md -file .\photo.png -file "https://example.com/cat.jpg" "结合 @.\extra.txt 描述这些内容"
```

本地图片默认以 base64 data URL 形式放入输入；也可以先上传到 OSS，再用返回的 URL 构造输入：

```powershell
.\dist\kuaima_cli.exe -file-format url -file .\photo.png "描述这张图片"
```

开启流式输出：

```powershell
.\dist\kuaima_cli.exe -stream "写一段产品介绍"
```

如果返回结果里包含图片 URL、data URL 或 base64 图片，默认会保存到当前目录。可以指定保存目录，或传空字符串关闭：

```powershell
.\dist\kuaima_cli.exe -save-images .\outputs "生成一张海报"
.\dist\kuaima_cli.exe -save-images "" "只打印文本"
```

需要启用图片生成工具时：

```powershell
.\dist\kuaima_cli.exe -image-generation -save-images .\outputs "生成一张白色咖啡杯产品图"
```

图片子命令会自动启用图片生成，并用 `-save-images` 保存响应中的所有图片：

```powershell
.\dist\kuaima_cli.exe image -save-images .\outputs "a clean product render of a white coffee mug"
.\dist\kuaima_cli.exe image -file .\reference.png -save-images .\outputs "生成这张参考图的产品海报版本"
```

兼容子命令：

```powershell
.\dist\kuaima_cli.exe ask -stream "你好"
.\dist\kuaima_cli.exe chat
.\dist\kuaima_cli.exe image -save-images .\outputs "a clean product render of a white coffee mug"
```

## 常用环境变量

```text
KUAIMA_API_KEY   API key，未设置时使用 OPENAI_API_KEY
KUAIMA_BASE_URL  默认 https://ai.szkmjb.com
KUAIMA_OSS_URL   默认 https://oss.szkmjb.com
KUAIMA_MODEL     默认 gpt-5.4-mini
KUAIMA_IMAGE_MODEL 图片生成模型；未设置时回退到 KUAIMA_MODEL/model
```
