package app

import (
	"context"
	"fmt"
	"os"
)

func saveResponseImages(ctx context.Context, c *client, resp *responsePayload, dir string) ([]string, error) {
	return saveResponseImagesWithSaver(newResponseImageSaver(ctx, c.httpClient, dir), resp)
}

func saveResponseImagesWithSaver(saver *responseImageSaver, resp *responsePayload) ([]string, error) {
	if saver == nil {
		return nil, nil
	}
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
