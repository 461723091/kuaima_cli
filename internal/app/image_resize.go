package app

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
)

func parseImageDimensions(value string) (int, int) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "auto" {
		return 0, 0
	}
	left, right, ok := strings.Cut(value, "x")
	if !ok {
		return 0, 0
	}
	width, err := strconv.Atoi(strings.TrimSpace(left))
	if err != nil || width <= 0 {
		return 0, 0
	}
	height, err := strconv.Atoi(strings.TrimSpace(right))
	if err != nil || height <= 0 {
		return 0, 0
	}
	return width, height
}

func resizeBilinear(src image.Image, dstWidth, dstHeight int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))
	if dstWidth <= 0 || dstHeight <= 0 {
		return dst
	}
	bounds := src.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	if srcWidth <= 0 || srcHeight <= 0 {
		return dst
	}
	if dstWidth == 1 || dstHeight == 1 {
		for y := 0; y < dstHeight; y++ {
			for x := 0; x < dstWidth; x++ {
				dst.Set(x, y, src.At(bounds.Min.X, bounds.Min.Y))
			}
		}
		return dst
	}

	xScale := float64(srcWidth-1) / float64(dstWidth-1)
	yScale := float64(srcHeight-1) / float64(dstHeight-1)
	for y := 0; y < dstHeight; y++ {
		sy := float64(y) * yScale
		y0 := int(math.Floor(sy))
		y1 := minInt(y0+1, srcHeight-1)
		ty := sy - float64(y0)
		for x := 0; x < dstWidth; x++ {
			sx := float64(x) * xScale
			x0 := int(math.Floor(sx))
			x1 := minInt(x0+1, srcWidth-1)
			tx := sx - float64(x0)

			c00 := toNRGBA(src.At(bounds.Min.X+x0, bounds.Min.Y+y0))
			c10 := toNRGBA(src.At(bounds.Min.X+x1, bounds.Min.Y+y0))
			c01 := toNRGBA(src.At(bounds.Min.X+x0, bounds.Min.Y+y1))
			c11 := toNRGBA(src.At(bounds.Min.X+x1, bounds.Min.Y+y1))

			dst.Set(x, y, color.NRGBA{
				R: lerpColor(lerpColor(c00.R, c10.R, tx), lerpColor(c01.R, c11.R, tx), ty),
				G: lerpColor(lerpColor(c00.G, c10.G, tx), lerpColor(c01.G, c11.G, tx), ty),
				B: lerpColor(lerpColor(c00.B, c10.B, tx), lerpColor(c01.B, c11.B, tx), ty),
				A: lerpColor(lerpColor(c00.A, c10.A, tx), lerpColor(c01.A, c11.A, tx), ty),
			})
		}
	}
	return dst
}

func toNRGBA(c color.Color) color.NRGBA {
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

func lerpColor(a, b uint8, t float64) uint8 {
	return uint8(math.Round(float64(a)*(1-t) + float64(b)*t))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
