package stl

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// RenderToPNG renders the STL triangles to a PNG image data using a simple software renderer
func RenderToPNG(info *STLInfo, width, height int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Background: dark gradient or solid
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{20, 22, 25, 255})
		}
	}

	if len(info.Triangles) == 0 {
		var buf bytes.Buffer
		err := png.Encode(&buf, img)
		return buf.Bytes(), err
	}

	// Calculate scale and offset to center the model
	dx := info.Bounds.MaxX - info.Bounds.MinX
	dy := info.Bounds.MaxY - info.Bounds.MinY
	dz := info.Bounds.MaxZ - info.Bounds.MinZ
	maxDim := float32(math.Max(float64(dx), math.Max(float64(dy), float64(dz))))

	scale := float32(width) * 0.7 / maxDim
	offsetX := float32(width) / 2
	offsetY := float32(height) / 2

	// Z-buffer for depth testing
	zBuffer := make([]float32, width*height)
	for i := range zBuffer {
		zBuffer[i] = -math.MaxFloat32
	}

	// Basic lighting direction
	lightDir := Vec3{X: 0.5, Y: 0.5, Z: 1.0}
	norm := float32(math.Sqrt(float64(lightDir.X*lightDir.X + lightDir.Y*lightDir.Y + lightDir.Z*lightDir.Z)))
	lightDir.X /= norm
	lightDir.Y /= norm
	lightDir.Z /= norm

	// Project and draw triangles
	for _, tri := range info.Triangles {
		// Simple orthographic projection with a slight rotation
		p1 := project(tri.V1, scale, offsetX, offsetY, info.Bounds)
		p2 := project(tri.V2, scale, offsetX, offsetY, info.Bounds)
		p3 := project(tri.V3, scale, offsetX, offsetY, info.Bounds)

		// Calculate shading based on normal
		dot := tri.Normal.X*lightDir.X + tri.Normal.Y*lightDir.Y + tri.Normal.Z*lightDir.Z
		brightness := float32(0.3) + float32(0.7)*float32(math.Max(0, float64(dot)))
		c := uint8(brightness * 200)
		shade := color.RGBA{uint8(float32(c) * 0.8), uint8(float32(c) * 0.9), c, 255}

		// Rasterize triangle
		rasterize(img, zBuffer, p1, p2, p3, shade)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type point struct {
	x, y int
	z    float32
}

func project(v Vec3, scale, ox, oy float32, b Bounds) point {
	// Center
	cx := (b.MaxX + b.MinX) / 2
	cy := (b.MaxY + b.MinY) / 2
	cz := (b.MaxZ + b.MinZ) / 2

	x := v.X - cx
	y := v.Y - cy
	z := v.Z - cz

	// Slight isometric rotation
	// y' = y*cos(a) - z*sin(a)
	// z' = y*sin(a) + z*cos(a)
	angle := float64(0.5)
	ry := float32(float64(y)*math.Cos(angle) - float64(z)*math.Sin(angle))
	rz := float32(float64(y)*math.Sin(angle) + float64(z)*math.Cos(angle))

	return point{
		x: int(x*scale + ox),
		y: int(-ry*scale + oy),
		z: rz,
	}
}

func rasterize(img *image.RGBA, zBuffer []float32, p1, p2, p3 point, c color.RGBA) {
	// Bounding box of triangle
	minX := minInt(p1.x, minInt(p2.x, p3.x))
	maxX := maxInt(p1.x, maxInt(p2.x, p3.x))
	minY := minInt(p1.y, minInt(p2.y, p3.y))
	maxY := maxInt(p1.y, maxInt(p2.y, p3.y))

	// Clip to image bounds
	bounds := img.Bounds()
	minX = maxInt(minX, bounds.Min.X)
	maxX = minInt(maxX, bounds.Max.X-1)
	minY = maxInt(minY, bounds.Min.Y)
	maxY = minInt(maxY, bounds.Max.Y-1)

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			w1, w2, w3 := barycentric(p1, p2, p3, x, y)
			if w1 >= 0 && w2 >= 0 && w3 >= 0 {
				z := w1*p1.z + w2*p2.z + w3*p3.z
				idx := y*bounds.Dx() + x
				if z > zBuffer[idx] {
					zBuffer[idx] = z
					img.Set(x, y, c)
				}
			}
		}
	}
}

func barycentric(p1, p2, p3 point, x, y int) (float32, float32, float32) {
	det := float32((p2.y-p3.y)*(p1.x-p3.x) + (p3.x-p2.x)*(p1.y-p3.y))
	if math.Abs(float64(det)) < 1e-6 {
		return -1, -1, -1
	}
	w1 := float32((p2.y-p3.y)*(x-p3.x)+(p3.x-p2.x)*(y-p3.y)) / det
	w2 := float32((p3.y-p1.y)*(x-p3.x)+(p1.x-p3.x)*(y-p3.y)) / det
	w3 := 1 - w1 - w2
	return w1, w2, w3
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
