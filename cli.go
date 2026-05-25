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
	"time"
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
	model      *string
	imageModel *string
	baseURL    *string
	ossURL     *string
	apiKey     *string
	username   *string
	password   *string
	verbose    *bool
	logFile    *string
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

type imageOptions struct {
	size              *string
	quality           *string
	count             *int
	outputFormat      *string
	outputCompression *int
	background        *string
	moderation        *string
	action            *string
	mask              *string
}

func addClientFlags(fs *flag.FlagSet) clientOptions {
	return addClientFlagsWithConfig(fs, appConfig{})
}

func addClientFlagsWithConfig(fs *flag.FlagSet, cfg appConfig) clientOptions {
	modelDefault := envOr("KUAIMA_MODEL", configNonEmptyString(cfg.Model, defaultModel))
	return clientOptions{
		model:      fs.String("model", modelDefault, "模型名称"),
		imageModel: fs.String("image-model", envOr("KUAIMA_IMAGE_MODEL", configNonEmptyString(cfg.ImageModel, defaultImageModel)), "图片生成模型名称"),
		baseURL:    fs.String("base-url", envOr("KUAIMA_BASE_URL", configNonEmptyString(cfg.BaseURL, defaultBaseURL)), "API 基础地址"),
		ossURL:     fs.String("oss-url", envOr("KUAIMA_OSS_URL", configNonEmptyString(cfg.OssURL, defaultOssURL)), "OSS API 基础地址"),
		apiKey:     fs.String("api-key", envOr("KUAIMA_API_KEY", envOr("OPENAI_API_KEY", configString(cfg.APIKey, ""))), "API key；覆盖 KUAIMA_API_KEY/OPENAI_API_KEY"),
		username:   fs.String("username", configString(cfg.Username, ""), "快马账号；未提供时自动生成并保存"),
		password:   fs.String("password", configString(cfg.Password, ""), "快马密码；未提供时自动生成并保存"),
		verbose:    fs.Bool("v", false, "打印完整请求和响应到 stderr"),
		logFile:    fs.String("log", "", "将请求、响应和耗时日志写入文件"),
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

func addImageFlags(fs *flag.FlagSet, cfg appConfig) imageOptions {
	return imageOptions{
		size:              fs.String("image-size", configNonEmptyString(cfg.ImageSize, "auto"), "image size, for example auto, 1024x1024, 1536x1024, 1024x1536, or gpt-image-2 custom WxH"),
		quality:           fs.String("image-quality", configNonEmptyString(cfg.ImageQuality, "auto"), "image quality, for example auto, low, medium, high"),
		count:             fs.Int("image-count", configInt(cfg.ImageCount, 1), "number of images for Images API generations"),
		outputFormat:      fs.String("image-output-format", configString(cfg.ImageOutputFormat, ""), "output image format: png, jpeg, or webp"),
		outputCompression: fs.Int("image-output-compression", configInt(cfg.ImageOutputCompression, -1), "output compression 0-100 for jpeg/webp; -1 leaves it unset"),
		background:        fs.String("image-background", configNonEmptyString(cfg.ImageBackground, "auto"), "background: auto, transparent, or opaque"),
		moderation:        fs.String("image-moderation", configString(cfg.ImageModeration, ""), "image moderation setting, for example auto or low"),
		action:            fs.String("image-action", configNonEmptyString(cfg.ImageAction, "auto"), "Responses image tool action: auto, generate, or edit"),
		mask:              fs.String("image-mask", "", "mask image path, data URL, or image URL for Images Edit"),
	}
}

func addAttachmentFlags(fs *flag.FlagSet) *stringsFlag {
	var attachments stringsFlag
	fs.Var(&attachments, "file", "输入文本/图片路径或图片 URL；可重复传入")
	fs.Var(&attachments, "image", "已废弃，等同于 -file")
	return &attachments
}

func addSaveImagesFlag(fs *flag.FlagSet, defaultDir string) *string {
	return fs.String("save-images", defaultDir, "保存响应中图片的目录或文件，空则不保存")
}

func addSaveImagesFlagWithConfig(fs *flag.FlagSet, cfg appConfig, defaultDir string) *string {
	return addSaveImagesFlag(fs, configString(cfg.SaveImages, defaultDir))
}

func (opts clientOptions) newClient() (*client, error) {
	return newClient(*opts.baseURL, *opts.ossURL, *opts.apiKey, *opts.username, *opts.password, *opts.verbose, *opts.logFile)
}

func (opts clientOptions) imageGenerationModel() string {
	if opts.imageModel != nil && strings.TrimSpace(*opts.imageModel) != "" {
		return *opts.imageModel
	}
	if opts.model != nil {
		return *opts.model
	}
	return ""
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

func (opts imageOptions) validate() error {
	if opts.count != nil && (*opts.count < 1 || *opts.count > 10) {
		return fmt.Errorf("invalid -image-count %d: expected 1-10", *opts.count)
	}
	if opts.outputCompression != nil && *opts.outputCompression != -1 && (*opts.outputCompression < 0 || *opts.outputCompression > 100) {
		return fmt.Errorf("invalid -image-output-compression %d: expected -1 or 0-100", *opts.outputCompression)
	}
	return nil
}

func (opts imageOptions) responseTool() map[string]any {
	tool := map[string]any{"type": "image_generation"}
	addAutoString(tool, "size", opts.size)
	addAutoString(tool, "quality", opts.quality)
	addString(tool, "output_format", opts.outputFormat)
	if opts.outputCompression != nil && *opts.outputCompression >= 0 {
		tool["output_compression"] = *opts.outputCompression
	}
	addAutoString(tool, "background", opts.background)
	addAutoString(tool, "action", opts.action)

	return tool
}

func (opts imageOptions) responseRunCount() int {
	if opts.count != nil && *opts.count > 1 {
		return *opts.count
	}
	return 1
}

func (opts imageOptions) generationRequest(model, prompt string) imageGenerationRequest {
	req := imageGenerationRequest{
		Model:  model,
		Prompt: prompt,
	}
	if opts.count != nil && *opts.count > 1 {
		req.N = *opts.count
	}
	if opts.size != nil && strings.TrimSpace(*opts.size) != "" && !strings.EqualFold(strings.TrimSpace(*opts.size), "auto") {
		req.Size = strings.TrimSpace(*opts.size)
	}
	if opts.quality != nil && strings.TrimSpace(*opts.quality) != "" && !strings.EqualFold(strings.TrimSpace(*opts.quality), "auto") {
		req.Quality = strings.TrimSpace(*opts.quality)
	}
	if opts.outputFormat != nil {
		req.OutputFormat = strings.TrimSpace(*opts.outputFormat)
	}
	if opts.outputCompression != nil && *opts.outputCompression >= 0 {
		req.OutputCompression = opts.outputCompression
	}
	if opts.background != nil && strings.TrimSpace(*opts.background) != "" && !strings.EqualFold(strings.TrimSpace(*opts.background), "auto") {
		req.Background = strings.TrimSpace(*opts.background)
	}
	if opts.moderation != nil {
		req.Moderation = strings.TrimSpace(*opts.moderation)
	}
	return req
}

func (opts imageOptions) editRequest(model, prompt string, images []imageRef, mask *imageRef) imageEditRequest {
	req := imageEditRequest{
		Model:  model,
		Prompt: prompt,
		Images: images,
		Mask:   mask,
	}
	if opts.count != nil && *opts.count > 1 {
		req.N = *opts.count
	}
	if opts.size != nil && strings.TrimSpace(*opts.size) != "" && !strings.EqualFold(strings.TrimSpace(*opts.size), "auto") {
		req.Size = strings.TrimSpace(*opts.size)
	}
	if opts.quality != nil && strings.TrimSpace(*opts.quality) != "" && !strings.EqualFold(strings.TrimSpace(*opts.quality), "auto") {
		req.Quality = strings.TrimSpace(*opts.quality)
	}
	if opts.outputFormat != nil {
		req.OutputFormat = strings.TrimSpace(*opts.outputFormat)
	}
	if opts.outputCompression != nil && *opts.outputCompression >= 0 {
		req.OutputCompression = opts.outputCompression
	}
	if opts.background != nil && strings.TrimSpace(*opts.background) != "" && !strings.EqualFold(strings.TrimSpace(*opts.background), "auto") {
		req.Background = strings.TrimSpace(*opts.background)
	}
	if opts.moderation != nil {
		req.Moderation = strings.TrimSpace(*opts.moderation)
	}
	return req
}

func imageRefs(ctx context.Context, c *client, images []string, fileFormat string) ([]imageRef, error) {
	if len(images) > 16 {
		return nil, fmt.Errorf("too many image inputs: got %d, expected at most 16", len(images))
	}
	refs := make([]imageRef, 0, len(images))
	for _, image := range images {
		ref, err := imageRefFromInput(ctx, c, image, fileFormat)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func imageRefFromInput(ctx context.Context, c *client, value, fileFormat string) (imageRef, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return imageRef{}, errors.New("empty image input")
	}
	imageURL, err := imageInputURL(ctx, c, value, fileFormat)
	if err != nil {
		return imageRef{}, err
	}
	return imageRef{ImageURL: imageURL}, nil
}

func addString(values map[string]any, key string, value *string) {
	if value != nil && strings.TrimSpace(*value) != "" {
		values[key] = strings.TrimSpace(*value)
	}
}

func addAutoString(values map[string]any, key string, value *string) {
	if value != nil && strings.TrimSpace(*value) != "" && !strings.EqualFold(strings.TrimSpace(*value), "auto") {
		values[key] = strings.TrimSpace(*value)
	}
}

func promptWithTextFiles(prompt string, files []textFileContent) string {
	parts := []string{}
	if strings.TrimSpace(prompt) != "" {
		parts = append(parts, strings.TrimSpace(prompt))
	}
	for _, file := range files {
		parts = append(parts, formatTextFileContent(file))
	}
	return strings.Join(parts, "\n\n")
}

func saveResponseImages(ctx context.Context, c *client, resp *responsePayload, dir string) ([]string, error) {
	return saveResponseImagesWithSaver(newResponseImageSaver(ctx, c.httpClient, dir), resp)
}

func saveResponseImagesWithSaver(saver *responseImageSaver, resp *responsePayload) ([]string, error) {
	saved, err := saver.saveResponse(resp)
	if err != nil {
		return nil, err
	}
	printSavedImages(saved)
	return saved, nil
}

func saveStreamImage(saver *responseImageSaver, candidate imageCandidate) error {
	path, err := saver.saveCandidate(candidate)
	if err != nil {
		return err
	}
	if path != "" {
		printSavedImages([]string{path})
	}
	return nil
}

func printSavedImages(paths []string) {
	for _, path := range paths {
		fmt.Fprintf(os.Stderr, "已保存图片: %s\n", path)
	}
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
	imageOpts := addImageFlags(fs, cfg)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := imageOpts.validate(); err != nil {
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
	defer c.Close()

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
		tool := imageOpts.responseTool()
		//指定模型
		addAutoString(tool, "model", opts.imageModel)
		//mask图片
		if imageOpts.mask != nil && *imageOpts.mask != "" {
			maskURL, err := imageInputURL(context.Background(), c, *imageOpts.mask, *inputOpts.fileFormat)
			if err != nil {
				return err
			}
			tool["input_image_mask"] = map[string]any{
				"image_url": maskURL,
			}
		}

		req.Tools = []map[string]any{tool}
	}

	runCount := 1
	if *imageGeneration {
		runCount = imageOpts.responseRunCount()
	}
	for i := 0; i < runCount; i++ {
		var resp *responsePayload
		imageSaver := newResponseImageSaver(context.Background(), c.httpClient, *saveDir)
		if *opts.stream {
			resp, err = c.createResponseStreamWithImages(context.Background(), req, os.Stdout, func(candidate imageCandidate) error {
				return saveStreamImage(imageSaver, candidate)
			})
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
		if _, err := saveResponseImagesWithSaver(imageSaver, resp); err != nil {
			return err
		}
	}
	return nil
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
	defer c.Close()

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
	imageOpts := addImageFlags(fs, cfg)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := imageOpts.validate(); err != nil {
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
	defer c.Close()
	images, textFiles, err := collectInputAttachments(prompt, *inputOpts.attachments)
	if err != nil {
		return err
	}
	var resp *responsePayload
	if len(images) == 0 {
		resp, err = c.createImageGeneration(context.Background(), imageOpts.generationRequest(opts.imageGenerationModel(), promptWithTextFiles(prompt, textFiles)))
	} else {
		editPrompt := promptWithTextFiles(prompt, textFiles)
		if strings.TrimSpace(editPrompt) == "" {
			return errors.New("must provide an image edit prompt")
		}
		refs, refsErr := imageRefs(context.Background(), c, images, *inputOpts.fileFormat)
		if refsErr != nil {
			return refsErr
		}
		var mask *imageRef
		if imageOpts.mask != nil && strings.TrimSpace(*imageOpts.mask) != "" {
			maskRef, maskErr := imageRefFromInput(context.Background(), c, *imageOpts.mask, *inputOpts.fileFormat)
			if maskErr != nil {
				return maskErr
			}
			mask = &maskRef
		}
		resp, err = c.createImageEdit(context.Background(), imageOpts.editRequest(opts.imageGenerationModel(), editPrompt, refs, mask))
	}
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

func runLogin(args []string) error {
	cfg, err := loadAppConfig()
	if err != nil {
		return err
	}
	fs := newFlagSet("login")
	opts := addClientFlagsWithConfig(fs, cfg)
	force := fs.Bool("force", false, "重新登录并刷新 API key")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := persistConfigFlags(fs, cfg); err != nil {
		return err
	}

	apiKey := strings.TrimSpace(*opts.apiKey)
	if apiKey == "" || *force {
		logFile, err := openLogFile(*opts.logFile)
		if err != nil {
			return err
		}
		var logWriter io.Writer
		if logFile != nil {
			defer logFile.Close()
			logWriter = logFile
		}
		apiKey, err = ensureAPIKey(context.Background(), *opts.baseURL, *opts.username, *opts.password, *opts.verbose, logWriter)
		if err != nil {
			return err
		}
	}
	fmt.Println(apiKey)
	return nil
}

func runBalance(args []string) error {
	cfg, err := loadAppConfig()
	if err != nil {
		return err
	}
	fs := newFlagSet("balance")
	opts := addClientFlagsWithConfig(fs, cfg)
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
	defer c.Close()
	usage, err := c.getTokenUsage(context.Background())
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", usage.Name)
	fmt.Printf("可用额度: %s\n", formatQuotaAmount(usage.TotalAvailable))
	fmt.Printf("已消耗: %s\n", formatQuotaAmount(usage.TotalUsed))
	if len(usage.Subscriptions) > 0 {
		fmt.Println("订阅套餐:")
		for _, item := range usage.Subscriptions {
			sub := item.Subscription
			fmt.Printf("    状态: %s\n", sub.Status)
			fmt.Printf("    可用额度: %s\n", formatQuotaAmount(sub.AmountTotal-sub.AmountUsed))
			fmt.Printf("    已消耗: %s\n", formatQuotaAmount(sub.AmountUsed))
			fmt.Printf("    订阅时间: %s\n", formatTime(sub.StartTime))
			fmt.Printf("    到期时间: %s\n", formatTime(sub.EndTime))
			//fmt.Printf("    last_reset_time: %d\n", sub.LastResetTime)
			//fmt.Printf("    next_reset_time: %d\n", sub.NextResetTime)
		}
	}
	return nil
}

func formatQuotaAmount(quota int64) string {
	return fmt.Sprintf("%d (￥%.2f)", quota, float64(quota)/500000)
}
func formatTime(ts int64) string {
	if ts <= 0 {
		return "-"
	}
	tm := time.Unix(ts, 0)
	return tm.Format("2006-01-02 15:04")
}

func runRecharge(args []string) error {
	cfg, err := loadAppConfig()
	if err != nil {
		return err
	}
	fs := newFlagSet("recharge")
	opts := addClientFlagsWithConfig(fs, cfg)
	printOnly := fs.Bool("print-url", false, "只打印充值页面 URL，不打开浏览器")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := persistConfigFlags(fs, cfg); err != nil {
		return err
	}
	logFile, err := openLogFile(*opts.logFile)
	if err != nil {
		return err
	}
	var logWriter io.Writer
	if logFile != nil {
		defer logFile.Close()
		logWriter = logFile
	}
	username, password, err := resolveRechargeCredentials(context.Background(), *opts.baseURL, *opts.username, *opts.password, *opts.verbose, logWriter)
	if err != nil {
		return err
	}
	rawURL, err := rechargeURL(*opts.baseURL, username, password, time.Now())
	if err != nil {
		return err
	}
	if *printOnly {
		fmt.Println(rawURL)
		return nil
	}
	if err := openBrowser(rawURL); err != nil {
		return err
	}
	fmt.Println(rawURL)
	return nil
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}
	return nil
}
