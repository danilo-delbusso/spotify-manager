package generator

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"io"
	"math"
	"math/rand"
	"spotify/internal/processor"

	"github.com/fogleman/gg"
	"hash/fnv"
)

const (
	imgWidth  = 640
	imgHeight = 640
)

type imageGenerator struct{}

// NewImageGenerator creates a new generator.
func NewImageGenerator() processor.ImageGenerator {
	return &imageGenerator{}
}

// GenerateForPlaylist creates an image with flowing, multi-colored waves.
func (g *imageGenerator) GenerateForPlaylist(name string) (io.Reader, error) {
	// 1. Create a deterministic seed from the playlist name.
	h := fnv.New64a()
	h.Write([]byte(name))
	seed := h.Sum64()
	rng := rand.New(rand.NewSource(int64(seed)))

	// 2. Generate a harmonious color palette from the seed.
	palette := generateAnalogousPalette(rng)

	// 3. Setup the drawing context and a dark background.
	dc := gg.NewContext(imgWidth, imgHeight)
	dc.SetRGB(0.1, 0.1, 0.15)
	dc.Clear()

	// 4. Draw several layers of waves.
	numWaves := 7
	for i := 0; i < numWaves; i++ {
		color := palette[rng.Intn(len(palette))]
		dc.SetRGB(color[0], color[1], color[2])

		// Randomize wave properties for variety.
		lineWidth := 2 + rng.Float64()*15
		amplitude := 50 + rng.Float64()*100
		frequency := 0.5 + rng.Float64()*2
		yOffset := float64(imgHeight/2) + (rng.Float64()-0.5)*300

		dc.SetLineWidth(lineWidth)

		// Draw a single sine wave across the canvas.
		for x := 0.0; x < float64(imgWidth); x++ {
			y := yOffset + math.Sin(x/float64(imgWidth)*math.Pi*2*frequency)*amplitude
			if x == 0 {
				dc.MoveTo(x, y)
			} else {
				dc.LineTo(x, y)
			}
		}
		dc.Stroke()
	}

	// 5. Encode the final image to a JPEG.
	img := dc.Image()
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("failed to encode image to jpeg: %w", err)
	}

	return buf, nil
}

// generateAnalogousPalette creates a set of 3 harmonious colors.
func generateAnalogousPalette(rng *rand.Rand) [][3]float64 {
	// Start with a random base hue, with good saturation and brightness.
	baseHue := rng.Float64() * 360
	saturation := 0.6
	value := 0.9

	// Create a palette with colors near each other on the color wheel.
	palette := make([][3]float64, 3)
	palette[0] = hsvToRgb(baseHue, saturation, value)
	palette[1] = hsvToRgb(math.Mod(baseHue+25, 360), saturation, value)
	palette[2] = hsvToRgb(math.Mod(baseHue-25, 360), saturation, value)

	return palette
}

// hsvToRgb converts HSV color values to RGB. h is [0-360], s and v are [0-1].
func hsvToRgb(h, s, v float64) [3]float64 {
	if s == 0 {
		return [3]float64{v, v, v}
	}
	h /= 60
	i := math.Floor(h)
	f := h - i
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))

	switch int(i) % 6 {
	case 0:
		return [3]float64{v, t, p}
	case 1:
		return [3]float64{q, v, p}
	case 2:
		return [3]float64{p, v, t}
	case 3:
		return [3]float64{p, q, v}
	case 4:
		return [3]float64{t, p, v}
	default: // case 5
		return [3]float64{v, p, q}
	}
}
