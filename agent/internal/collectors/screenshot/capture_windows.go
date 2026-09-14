//go:build windows

package screenshot

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"time"

	"github.com/kbinani/screenshot"

	"github.com/southcorner/systemcheck/agent/internal/collectors"
	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// capture grabs every active display, encodes JPEG, stores the blob, and emits a
// screenshot metadata event referencing the uploaded object key.
//
// NOTE: MVP encodes JPEG via the standard library. WebP and region blur are
// planned; see docs/ARCHITECTURE.md.
func (c *Collector) capture(ctx context.Context, emit collectors.Emit, emitBlob collectors.EmitBlob) error {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return fmt.Errorf("no active displays")
	}
	quality := c.pol.Quality
	if quality <= 0 || quality > 100 {
		quality = 60
	}
	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return err
		}
		key := fmt.Sprintf("%s.jpg", randID())
		objectKey, err := emitBlob(key, buf.Bytes())
		if err != nil {
			return err
		}
		emit(wire.Event{
			Kind: "screenshot",
			TS:   time.Now().UTC(),
			Data: map[string]interface{}{
				"object_key": objectKey,
				"display":    i,
				"width":      imgW(img),
				"height":     imgH(img),
				"bytes":      buf.Len(),
				"format":     "jpeg",
			},
		})
	}
	return nil
}

func imgW(img image.Image) int { return img.Bounds().Dx() }
func imgH(img image.Image) int { return img.Bounds().Dy() }
