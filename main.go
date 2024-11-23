package main

import (
	"image"
	"image/color"
	"image/png"
	"math/cmplx"
	"os"
	"runtime"
	"sync"
)

type Parameters struct {
	XMin, YMin, XMax, YMax float64
	Width, Height          int
	SubPixelsSamples       int
	MaxIterations          int
	Contrast               int
}

func NewDefaultParameters() Parameters {
	return Parameters{
		XMin: -2, YMin: -2,
		XMax: 2, YMax: 2,
		Width: 1024, Height: 1024,
		SubPixelsSamples: 4,
		MaxIterations:    200,
		Contrast:         15,
	}
}

// Point はサンプリングポイント
type Point struct {
	X, Y float64
}

func main() {
	params := NewDefaultParameters()
	img := GenerateMandelbrot(params)

	if err := saveImage(img, "mandelbrot.png"); err != nil {
		panic(err)
	}
}

func GenerateMandelbrot(params Parameters) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, params.Width, params.Height))
	samplingWidth := 0.5 / float64(params.Width) * (params.XMax - params.XMin)
	samplingHeight := 0.5 / float64(params.Height) * (params.YMax - params.YMin)

	numWorkers := runtime.NumCPU()
	runtime.GOMAXPROCS(numWorkers)
	var wg sync.WaitGroup
	wg.Add(params.Height)

	for py := 0; py < params.Height; py++ {
		go func(py int) {
			defer wg.Done()
			processRow(py, img, params, samplingWidth, samplingHeight)
		}(py)
	}

	wg.Wait()
	return img
}

// Pixel1行分の計算
func processRow(py int, img *image.RGBA, params Parameters, samplingWidth, samplingHeight float64) {
	y := float64(py)/float64(params.Height)*(params.YMax-params.YMin) + params.YMin

	for px := 0; px < params.Width; px++ {
		x := float64(px)/float64(params.Width)*(params.XMax-params.XMin) + params.XMin
		samples := getSamples(x, y, samplingWidth, samplingHeight, params)
		avgColor := averageColors(samples)
		img.Set(px, py, avgColor)
	}
}

// スーパーサンプリング用のカラーサンプル
func getSamples(x, y, samplingWidth, samplingHeight float64, params Parameters) []color.Color {
	points := []Point{
		{x, y},
		{x + samplingWidth, y},
		{x + samplingWidth, y + samplingHeight},
		{x, y + samplingHeight},
	}

	samples := make([]color.Color, len(points))
	for i, p := range points {
		samples[i] = mandelbrot(complex(p.X, p.Y), params)
	}
	return samples
}

// 色の計算
func mandelbrot(z complex128, params Parameters) color.Color {
	var v complex128
	for n := uint8(0); n < uint8(params.MaxIterations); n++ {
		v = v*v + z
		if cmplx.Abs(v) > 2 {
			return color.RGBA{
				R: 64 - uint8(params.Contrast)*n,
				G: 80 - uint8(params.Contrast)*n%128,
				B: 240 + uint8(params.Contrast)*n%64,
				A: 255,
			}
		}
	}
	return color.Black
}

// 複数のサンプルから平均色を計算
func averageColors(colors []color.Color) color.Color {
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

func saveImage(img *image.RGBA, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
