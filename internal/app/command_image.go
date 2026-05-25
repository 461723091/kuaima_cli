package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

func runImage(args []string) error {
	cfg, err := loadAppConfig()
	if err != nil {
		return err
	}
	fs := newFlagSet("image")
	opts := addClientFlagsWithConfig(fs, cfg)
	stream := fs.Bool("stream", configBool(cfg.Stream, false), "stream image generation events")
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
	imageSaver := newResponseImageSaver(context.Background(), c.httpClient, *saveDir)
	if len(images) == 0 {
		req := imageOpts.generationRequest(opts.imageGenerationModel(), promptWithTextFiles(prompt, textFiles))
		if *stream {
			resp, err = c.createImageGenerationStream(context.Background(), req, func(candidate imageCandidate) error {
				return saveStreamImage(imageSaver, candidate)
			})
			if err == nil {
				fmt.Fprintln(os.Stdout)
			}
		} else {
			resp, err = c.createImageGeneration(context.Background(), req)
		}
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
		req := imageOpts.editRequest(opts.imageGenerationModel(), editPrompt, refs, mask)
		if *stream {
			resp, err = c.createImageEditStream(context.Background(), req, func(candidate imageCandidate) error {
				return saveStreamImage(imageSaver, candidate)
			})
			if err == nil {
				fmt.Fprintln(os.Stdout)
			}
		} else {
			resp, err = c.createImageEdit(context.Background(), req)
		}
	}
	if err != nil {
		return err
	}

	saved, err := saveResponseImagesWithSaver(imageSaver, resp)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*saveDir) != "" && len(saved) == 0 {
		return errors.New("响应中没有图片")
	}
	return nil
}
