package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/cmplx"
	"os"
	"sync"
)

type Generator struct {
	params Parameters
}

type Parameters struct {
	ViewPort struct {
		XMin, YMin float64
		XMax, YMax float64
	}
	Size struct {
		Width  int
		Height int
	}
	RenderOpts struct {
		SubPixelSamples int
		MaxIterations   int
		Contrast        int
	}
}

// 不正なパラメータが指定された場合のエラー
var ErrInvalidParameters = errors.New("invalid parameters")

func NewDefaultParameters() Parameters {
	p := Parameters{}
	p.ViewPort.XMin = -2
	p.ViewPort.YMin = -2
	p.ViewPort.XMax = 2
	p.ViewPort.YMax = 2
	p.Size.Width = 1024
	p.Size.Height = 1024
	p.RenderOpts.SubPixelSamples = 4
	p.RenderOpts.MaxIterations = 200
	p.RenderOpts.Contrast = 15
	return p
}

func NewGenerator(params Parameters) (*Generator, error) {
	if err := validateParameters(params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	return &Generator{params: params}, nil
}

func validateParameters(p Parameters) error {
	if p.ViewPort.XMax <= p.ViewPort.XMin || p.ViewPort.YMax <= p.ViewPort.YMin {
		return fmt.Errorf("%w: invalid viewport range", ErrInvalidParameters)
	}
	if p.Size.Width <= 0 || p.Size.Height <= 0 {
		return fmt.Errorf("%w: invalid image size", ErrInvalidParameters)
	}
	if p.RenderOpts.SubPixelSamples <= 0 {
		return fmt.Errorf("%w: invalid subpixel samples", ErrInvalidParameters)
	}
	return nil
}

// point はサンプリングポイントを表す
type point struct {
	x, y float64
}

func (g *Generator) Generate(ctx context.Context) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, g.params.Size.Width, g.params.Size.Height))

	samplingWidth := 0.5 / float64(g.params.Size.Width) * (g.params.ViewPort.XMax - g.params.ViewPort.XMin)
	samplingHeight := 0.5 / float64(g.params.Size.Height) * (g.params.ViewPort.YMax - g.params.ViewPort.YMin)

	var wg sync.WaitGroup
	errChan := make(chan error, g.params.Size.Height)

	// 各行を並列処理
	for py := 0; py < g.params.Size.Height; py++ {
		wg.Add(1)
		go func(py int) {
			defer wg.Done()
			if err := g.processRow(ctx, py, img, samplingWidth, samplingHeight); err != nil {
				errChan <- err
			}
		}(py)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return nil, fmt.Errorf("error processing row: %w", err)
		}
	}

	return img, nil
}

// 1行分のピクセルを処理する
func (g *Generator) processRow(ctx context.Context, py int, img *image.RGBA, samplingWidth, samplingHeight float64) error {
	y := float64(py)/float64(g.params.Size.Height)*(g.params.ViewPort.YMax-g.params.ViewPort.YMin) + g.params.ViewPort.YMin

	for px := 0; px < g.params.Size.Width; px++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			x := float64(px)/float64(g.params.Size.Width)*(g.params.ViewPort.XMax-g.params.ViewPort.XMin) + g.params.ViewPort.XMin
			samples := g.getSamples(x, y, samplingWidth, samplingHeight)
			avgColor := averageColors(samples)
			img.Set(px, py, avgColor)
		}
	}
	return nil
}

// スーパーサンプリング用のカラーサンプルを取得する
func (g *Generator) getSamples(x, y, samplingWidth, samplingHeight float64) []color.Color {
	points := []point{
		{x, y},
		{x + samplingWidth, y},
		{x + samplingWidth, y + samplingHeight},
		{x, y + samplingHeight},
	}

	samples := make([]color.Color, 0, len(points))
	for _, p := range points {
		samples = append(samples, g.mandelbrot(complex(p.x, p.y)))
	}
	return samples
}

func (g *Generator) mandelbrot(z complex128) color.Color {
	var v complex128
	for n := uint8(0); n < uint8(g.params.RenderOpts.MaxIterations); n++ {
		v = v*v + z
		if cmplx.Abs(v) > 2 {
			return color.RGBA{
				R: 64 - uint8(g.params.RenderOpts.Contrast)*n,
				G: 80 - uint8(g.params.RenderOpts.Contrast)*n%128,
				B: 240 + uint8(g.params.RenderOpts.Contrast)*n%64,
				A: 255,
			}
		}
	}
	return color.Black
}

// 複数のサンプルから平均色を計算する
func averageColors(colors []color.Color) color.Color {
	if len(colors) == 0 {
		return color.Black
	}

	var r, g, b, a uint32
	for _, c := range colors {
		cr, cg, cb, ca := c.RGBA()
		r += cr
		g += cg
		b += cb
		a += ca
	}

	n := uint32(len(colors))
	return color.RGBA{
		R: uint8(r / n >> 8),
		G: uint8(g / n >> 8),
		B: uint8(b / n >> 8),
		A: uint8(a / n >> 8),
	}
}

func SaveImage(img *image.RGBA, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}
	return nil
}

func main() {
	params := NewDefaultParameters()
	generator, err := NewGenerator(params)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	img, err := generator.Generate(ctx)
	if err != nil {
		panic(err)
	}

	if err := SaveImage(img, "mandelbrot.png"); err != nil {
		panic(err)
	}
}
