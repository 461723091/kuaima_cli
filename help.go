package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		commandUsage(fs.Output(), name, fs)
	}
	return fs
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "用法:")
	fmt.Fprintln(w, "  kuaima_cli [ask] [选项] 提示词")
	fmt.Fprintln(w, "  kuaima_cli [ask] [选项] < prompt.txt")
	fmt.Fprintln(w, "  kuaima_cli chat [选项]")
	fmt.Fprintln(w, "  kuaima_cli image [选项] 提示词")
	//fmt.Fprintln(w, "  kuaima_cli login [选项]")
	fmt.Fprintln(w, "  kuaima_cli balance [选项]")
	fmt.Fprintln(w, "  kuaima_cli recharge [选项]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "命令:")
	fmt.Fprintln(w, "  ask      发送一次提示词并打印回复；省略命令时默认执行 ask")
	fmt.Fprintln(w, "  chat     进入交互式对话模式")
	fmt.Fprintln(w, "  image    启用图片生成并保存响应中的所有图片")
	//fmt.Fprintln(w, "  login    自动登录/注册账号并保存 API key")
	fmt.Fprintln(w, "  balance  查询当前 API key 的余额")
	fmt.Fprintln(w, "  recharge 打开充值页面")
	fmt.Fprintln(w, "  help     打印帮助信息")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "常用选项:")
	fmt.Fprintln(w, "  -file PATH_OR_URL       输入文本/图片路径或图片 URL；可重复传入")
	fmt.Fprintln(w, "  -file-format FORMAT     本地图片输入格式：base64 或 url；默认 base64")
	fmt.Fprintln(w, "  -image-generation       启用图片生成工具")
	fmt.Fprintln(w, "  -stream                 实时打印返回的文本增量")
	fmt.Fprintln(w, "  -save-images DIR        保存响应中的图片；留空则不保存")
	fmt.Fprintln(w, "  -model MODEL            默认使用 KUAIMA_MODEL 或 gpt-5.4-mini")
	fmt.Fprintln(w, "  -image-model MODEL      图片生成优先使用 KUAIMA_IMAGE_MODEL 或 image_model")
	fmt.Fprintln(w, "  -base-url URL           默认使用 KUAIMA_BASE_URL 或 https://ai.szkmjb.com")
	fmt.Fprintln(w, "  -system TEXT            可选的系统/开发者指令")
	fmt.Fprintln(w, "  -api-key KEY            API key；覆盖 KUAIMA_API_KEY/OPENAI_API_KEY")
	fmt.Fprintln(w, "  -username USER          快马账号；未提供时自动生成")
	fmt.Fprintln(w, "  -password PASS          快马密码；未提供时自动生成")
	fmt.Fprintln(w, "  -v                      打印调试信息到 stderr")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "环境变量:")
	fmt.Fprintln(w, "  KUAIMA_API_KEY          API key；未设置时使用 OPENAI_API_KEY")
	fmt.Fprintln(w, "  KUAIMA_BASE_URL         默认 https://ai.szkmjb.com")
	fmt.Fprintln(w, "  KUAIMA_OSS_URL          默认 https://oss.szkmjb.com")
	fmt.Fprintln(w, "  KUAIMA_MODEL            默认 gpt-5.4-mini")
	fmt.Fprintln(w, "  KUAIMA_IMAGE_MODEL      图片生成模型；未设置时回退到 KUAIMA_MODEL/model")
	//fmt.Fprintln(w)
	//fmt.Fprintln(w, "配置文件:")
	//fmt.Fprintln(w, "  ~/.kuaima/config.json   优先级低于环境变量和命令行参数")
}

func commandUsage(w io.Writer, name string, fs *flag.FlagSet) {
	switch name {
	case "ask":
		fmt.Fprintln(w, "用法:")
		fmt.Fprintln(w, "  kuaima_cli [ask] [选项] 提示词")
		fmt.Fprintln(w, "  kuaima_cli [ask] [选项] < prompt.txt")
	case "chat":
		fmt.Fprintln(w, "用法:")
		fmt.Fprintln(w, "  kuaima_cli chat [选项]")
	case "image":
		fmt.Fprintln(w, "用法:")
		fmt.Fprintln(w, "  kuaima_cli image [选项] 提示词")
	case "login":
		fmt.Fprintln(w, "用法:")
		fmt.Fprintln(w, "  kuaima_cli login [选项]")
	case "balance":
		fmt.Fprintln(w, "用法:")
		fmt.Fprintln(w, "  kuaima_cli balance [选项]")
	case "recharge":
		fmt.Fprintln(w, "用法:")
		fmt.Fprintln(w, "  kuaima_cli recharge [选项]")
	default:
		fmt.Fprintf(w, "用法:\n  kuaima_cli %s [选项]\n", name)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "选项:")
	fs.PrintDefaults()
}
