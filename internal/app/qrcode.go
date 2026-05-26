package app

import (
	"fmt"
	"io"
	"os"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

func printQRCode(w io.Writer, value string) error {
	qr, err := qrcode.New(value, qrcode.Medium)
	if err != nil {
		return err
	}
	bitmap := qr.Bitmap()
	const quiet = 2
	for y := -quiet; y < len(bitmap)+quiet; y++ {
		var line strings.Builder
		for x := -quiet; x < len(bitmap)+quiet; x++ {
			if y >= 0 && y < len(bitmap) && x >= 0 && x < len(bitmap[y]) && bitmap[y][x] {
				line.WriteString("##")
			} else {
				line.WriteString("  ")
			}
		}
		fmt.Fprintln(w, line.String())
	}
	return nil
}

func writeQRCodePNG(path, value string, size int) error {
	png, err := qrcodePNG(value, size)
	if err != nil {
		return err
	}
	return os.WriteFile(path, png, 0600)
}

func qrcodePNG(value string, size int) ([]byte, error) {
	if size <= 0 {
		size = 512
	}
	return qrcode.Encode(value, qrcode.Medium, size)
}
