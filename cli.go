package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type stringsFlag []string

func (f *stringsFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringsFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("value cannot be empty")
	}
	*f = append(*f, value)
	return nil
}

type clientOptions struct {
	model   *string
	baseURL *string
	apiKey  *string
	verbose *bool
}

type responseOptions struct {
	clientOptions
	system *string
	stream *bool
}

func addClientFlags(fs *flag.FlagSet) clientOptions {
	return clientOptions{
		model:   fs.String("model", envOr("KUAIMA_MODEL", defaultModel), "模型名称"),
		baseURL: fs.String("base-url", envOr("KUAIMA_BASE_URL", defaultBaseURL), "API 基础地址"),
		apiKey:  fs.String("api-key", "", "API key; overrides KUAIMA_API_KEY/OPENAI_API_KEY"),
		verbose: fs.Bool("v", false, "print full request and response to stderr"),
	}
}

func addResponseFlags(fs *flag.FlagSet) responseOptions {
	return responseOptions{
		clientOptions: addClientFlags(fs),
		system:        fs.String("system", "", "可选的系统/开发者指令"),
		stream:        fs.Bool("stream", false, "实时打印服务端返回的文本增量"),
	}
}

func addAttachmentFlags(fs *flag.FlagSet) *stringsFlag {
	var attachments stringsFlag
	fs.Var(&attachments, "file", "file path or image URL to include; can be repeated")
	fs.Var(&attachments, "image", "deprecated alias for -file")
	return &attachments
}

func (opts clientOptions) newClient() (*client, error) {
	return newClient(*opts.baseURL, *opts.apiKey, *opts.verbose)
}

func runAsk(args []string) error {
	fs := newFlagSet("ask")
	opts := addResponseFlags(fs)
	imageGeneration := fs.Bool("image-generation", false, "启用图片生成工具")
	saveDir := fs.String("save-images", ".", "保存响应中图片的目录")
	attachments := addAttachmentFlags(fs)
	if err := parseFlags(fs, args); err != nil {
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
	if prompt == "" && len(*attachments) == 0 {
		return errors.New("prompt is required")
	}
	images, textFiles, err := collectInputAttachments(prompt, *attachments)
	if err != nil {
		return err
	}

	c, err := opts.newClient()
	if err != nil {
		return err
	}

	input, err := buildInput(prompt, strings.TrimSpace(*opts.system), images, textFiles)
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

	if strings.TrimSpace(*saveDir) != "" {
		saved, err := saveImagesFromResponse(context.Background(), c.httpClient, resp, *saveDir)
		if err != nil {
			return err
		}
		for _, path := range saved {
			fmt.Fprintf(os.Stderr, "saved image: %s\n", path)
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
	fs := newFlagSet("chat")
	opts := addResponseFlags(fs)
	files := addAttachmentFlags(fs)
	if err := parseFlags(fs, args); err != nil {
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
		images, textFiles, err := collectInputAttachments(text, *files)
		if err != nil {
			return err
		}

		input, err := buildInput(text, strings.TrimSpace(*opts.system), images, textFiles)
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
	fs := newFlagSet("image")
	opts := addClientFlags(fs)
	output := fs.String("o", "image.png", "输出图片路径")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	prompt := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if prompt == "" {
		return errors.New("image prompt is required")
	}

	c, err := opts.newClient()
	if err != nil {
		return err
	}
	input, err := buildInput(prompt, "", nil, nil)
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

	candidates := imageCandidates(resp)
	if len(candidates) == 0 {
		return errors.New("no image found in response")
	}
	data, mediaType, err := loadImageCandidate(context.Background(), c.httpClient, candidates[0])
	if err != nil {
		return err
	}
	if mediaType != "" && filepath.Ext(*output) == "" {
		*output += extensionForMediaType(mediaType, "")
	}
	savedPath, err := writeFileUnique(*output, data, 0644)
	if err != nil {
		return err
	}
	fmt.Printf("已保存 %s\n", savedPath)
	return nil
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}
	return nil
}
