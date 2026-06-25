package app

import (
	"errors"
	"flag"
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
	upscale           *bool
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

func addResponseFlagsWithConfig(fs *flag.FlagSet, cfg appConfig) responseOptions {
	return responseOptions{
		clientOptions: addClientFlagsWithConfig(fs, cfg),
		system:        fs.String("system", configString(cfg.System, ""), "可选的系统/开发者指令"),
		stream:        fs.Bool("stream", configBool(cfg.Stream, true), "实时打印服务端返回的文本增量"),
	}
}

func addInputFlags(fs *flag.FlagSet) inputOptions {
	return addInputFlagsWithConfig(fs, appConfig{})
}

func addInputFlagsWithConfig(fs *flag.FlagSet, cfg appConfig) inputOptions {
	return inputOptions{
		fileFormat:  fs.String("file-format", configNonEmptyString(cfg.FileFormat, "url"), "本地图片输入格式：base64 或 url"),
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
		upscale:           fs.Bool("image-upscale", configBool(cfg.ImageUpscale, true), "save images at the target size when they are smaller; keeps aspect ratio"),
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

func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}
	return nil
}
