package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

type stringsFlag []string

func (f *stringsFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringsFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("值不能为空")
	}
	*f = append(*f, value)
	return nil
}

type clientOptions struct {
	model   *string
	baseURL *string
	ossURL  *string
	apiKey  *string
	verbose *bool
}

type responseOptions struct {
	clientOptions
	system *string
	stream *bool
}

type inputOptions struct {
	fileFormat  *string
	attachments *stringsFlag
}

func addClientFlags(fs *flag.FlagSet) clientOptions {
	return addClientFlagsWithConfig(fs, appConfig{})
}

func addClientFlagsWithConfig(fs *flag.FlagSet, cfg appConfig) clientOptions {
	return clientOptions{
		model:   fs.String("model", envOr("KUAIMA_MODEL", configNonEmptyString(cfg.Model, defaultModel)), "模型名称"),
		baseURL: fs.String("base-url", envOr("KUAIMA_BASE_URL", configNonEmptyString(cfg.BaseURL, defaultBaseURL)), "API 基础地址"),
		ossURL:  fs.String("oss-url", envOr("KUAIMA_OSS_URL", configNonEmptyString(cfg.OssURL, defaultOssURL)), "OSS API 基础地址"),
		apiKey:  fs.String("api-key", envOr("KUAIMA_API_KEY", envOr("OPENAI_API_KEY", configString(cfg.APIKey, ""))), "API key；覆盖 KUAIMA_API_KEY/OPENAI_API_KEY"),
		verbose: fs.Bool("v", false, "打印完整请求和响应到 stderr"),
	}
}

func addResponseFlags(fs *flag.FlagSet) responseOptions {
	return addResponseFlagsWithConfig(fs, appConfig{})
}

func addResponseFlagsWithConfig(fs *flag.FlagSet, cfg appConfig) responseOptions {
	return responseOptions{
		clientOptions: addClientFlagsWithConfig(fs, cfg),
		system:        fs.String("system", configString(cfg.System, ""), "可选的系统/开发者指令"),
		stream:        fs.Bool("stream", configBool(cfg.Stream, false), "实时打印服务端返回的文本增量"),
	}
}

func addInputFlags(fs *flag.FlagSet) inputOptions {
	return addInputFlagsWithConfig(fs, appConfig{})
}

func addInputFlagsWithConfig(fs *flag.FlagSet, cfg appConfig) inputOptions {
	return inputOptions{
		fileFormat:  fs.String("file-format", configNonEmptyString(cfg.FileFormat, "base64"), "本地图片输入格式：base64 或 url"),
		attachments: addAttachmentFlags(fs),
	}
}

func addAttachmentFlags(fs *flag.FlagSet) *stringsFlag {
	var attachments stringsFlag
	fs.Var(&attachments, "file", "输入文本/图片路径或图片 URL；可重复传入")
	fs.Var(&attachments, "image", "已废弃，等同于 -file")
	return &attachments
}

func addSaveImagesFlag(fs *flag.FlagSet, defaultDir string) *string {
	return fs.String("save-images", defaultDir, "保存响应中图片的目录；留空则不保存")
}

func addSaveImagesFlagWithConfig(fs *flag.FlagSet, cfg appConfig, defaultDir string) *string {
	return addSaveImagesFlag(fs, configString(cfg.SaveImages, defaultDir))
}

func (opts clientOptions) newClient() (*client, error) {
	return newClient(*opts.baseURL, *opts.ossURL, *opts.apiKey, *opts.verbose)
}

func (opts inputOptions) buildInput(ctx context.Context, c *client, prompt, system string) ([]inputMessage, error) {
	images, textFiles, err := collectInputAttachments(prompt, *opts.attachments)
	if err != nil {
		return nil, err
	}
	return buildInputWithFileFormat(ctx, c, prompt, system, images, textFiles, *opts.fileFormat)
}

func (opts inputOptions) hasPromptOrAttachments(prompt string) bool {
	return strings.TrimSpace(prompt) != "" || len(*opts.attachments) > 0
}

func saveResponseImages(ctx context.Context, c *client, resp *responsePayload, dir string) ([]string, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil
	}
	saved, err := saveImagesFromResponse(ctx, c.httpClient, resp, dir)
	if err != nil {
		return nil, err
	}
	for _, path := range saved {
		fmt.Fprintf(os.Stderr, "已保存图片: %s\n", path)
	}
	return saved, nil
}

func runAsk(args []string) error {
	cfg, err := loadAppConfig()
	if err != nil {
		return err
	}
	fs := newFlagSet("ask")
	opts := addResponseFlagsWithConfig(fs, cfg)
	imageGeneration := fs.Bool("image-generation", false, "启用图片生成工具")
	saveDir := addSaveImagesFlagWithConfig(fs, cfg, ".")
	inputOpts := addInputFlagsWithConfig(fs, cfg)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := persistConfigFlags(fs, cfg); err != nil {
		return err
	}

	prompt := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if prompt == "" {
		var err error
		prompt, err = readPromptFromStdin()
		if err != nil {
			return err
		}
	}
	if !inputOpts.hasPromptOrAttachments(prompt) {
		return errors.New("必须提供提示词或附件")
	}

	c, err := opts.newClient()
	if err != nil {
		return err
	}

	input, err := inputOpts.buildInput(context.Background(), c, prompt, strings.TrimSpace(*opts.system))
	if err != nil {
		return err
	}
	req := responseRequest{
		Model:  *opts.model,
		Input:  input,
		Stream: *opts.stream,
	}
	if *imageGeneration {
		req.Tools = []map[string]string{{"type": "image_generation"}}
	}

	var resp *responsePayload
	if *opts.stream {
		resp, err = c.createResponseStream(context.Background(), req, os.Stdout)
		if err == nil {
			fmt.Fprintln(os.Stdout)
		}
	} else {
		resp, err = c.createResponse(context.Background(), req)
		if err == nil {
			text := strings.TrimSpace(resp.text())
			if text != "" {
				fmt.Println(text)
			}
		}
	}
	if err != nil {
		return err
	}

	_, err = saveResponseImages(context.Background(), c, resp, *saveDir)
	return err
}

func readPromptFromStdin() (string, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}

	fmt.Fprint(os.Stderr, "> ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", scanner.Err()
	}
	return strings.TrimSpace(scanner.Text()), nil
}

func runChat(args []string) error {
	cfg, err := loadAppConfig()
	if err != nil {
		return err
	}
	fs := newFlagSet("chat")
	opts := addResponseFlagsWithConfig(fs, cfg)
	inputOpts := addInputFlagsWithConfig(fs, cfg)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := persistConfigFlags(fs, cfg); err != nil {
		return err
	}

	c, err := opts.newClient()
	if err != nil {
		return err
	}

	fmt.Println("简单对话模式。输入 /exit 或 /quit 退出。")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		if text == "/exit" || text == "/quit" {
			break
		}
		input, err := inputOpts.buildInput(context.Background(), c, text, strings.TrimSpace(*opts.system))
		if err != nil {
			return err
		}
		req := responseRequest{Model: *opts.model, Input: input, Stream: *opts.stream}
		if *opts.stream {
			if _, err := c.createResponseStream(context.Background(), req, os.Stdout); err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout)
			continue
		}
		resp, err := c.createResponse(context.Background(), req)
		if err != nil {
			return err
		}
		answer := strings.TrimSpace(resp.text())
		if answer == "" {
			answer = "(空响应)"
		}
		fmt.Println(answer)
	}
	return scanner.Err()
}

func runImage(args []string) error {
	cfg, err := loadAppConfig()
	if err != nil {
		return err
	}
	fs := newFlagSet("image")
	opts := addClientFlagsWithConfig(fs, cfg)
	saveDir := addSaveImagesFlagWithConfig(fs, cfg, ".")
	inputOpts := addInputFlagsWithConfig(fs, cfg)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := persistConfigFlags(fs, cfg); err != nil {
		return err
	}

	prompt := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if !inputOpts.hasPromptOrAttachments(prompt) {
		return errors.New("必须提供图片提示词或附件")
	}

	c, err := opts.newClient()
	if err != nil {
		return err
	}
	input, err := inputOpts.buildInput(context.Background(), c, prompt, "")
	if err != nil {
		return err
	}
	resp, err := c.createResponse(context.Background(), responseRequest{
		Model: *opts.model,
		Input: input,
		Tools: []map[string]string{{"type": "image_generation"}},
	})
	if err != nil {
		return err
	}

	saved, err := saveResponseImages(context.Background(), c, resp, *saveDir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*saveDir) != "" && len(saved) == 0 {
		return errors.New("响应中没有图片")
	}
	return nil
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}
	return nil
}
