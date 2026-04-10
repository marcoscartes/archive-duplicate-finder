package stl

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// STLDiff represents differences between two STL files
type STLDiff struct {
	Vertices1   int
	Vertices2   int
	Triangles1  int
	Triangles2  int
	Description string
}

// IsSTLFile checks if a filename is an STL file
func IsSTLFile(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".stl")
}

// CompareSTL compares two STL files and returns if they're identical and their differences
func CompareSTL(data1, data2 []byte) (identical bool, diff *STLDiff) {
	// Quick check: if data is identical, files are identical
	if bytes.Equal(data1, data2) {
		return true, nil
	}

	// Parse both STL files
	info1, err1 := ParseSTL(data1)
	info2, err2 := ParseSTL(data2)

	if err1 != nil || err2 != nil {
		// If we can't parse, just compare bytes
		return false, &STLDiff{
			Description: "Unable to parse STL format",
		}
	}

	// Create diff
	diff = &STLDiff{
		Vertices1:  info1.VertexCount,
		Vertices2:  info2.VertexCount,
		Triangles1: info1.TriangleCount,
		Triangles2: info2.TriangleCount,
	}

	// Analyze differences
	if info1.TriangleCount != info2.TriangleCount {
		triangleDiff := info2.TriangleCount - info1.TriangleCount

		if triangleDiff > 0 {
			diff.Description = fmt.Sprintf("Geometry expanded (+%d triangles)", triangleDiff)
		} else {
			diff.Description = fmt.Sprintf("Geometry simplified (%d triangles)", triangleDiff)
		}

		// Check if bounds changed
		if !boundsEqual(info1.Bounds, info2.Bounds) {
			diff.Description += ", dimensions changed"
		}
	} else if !boundsEqual(info1.Bounds, info2.Bounds) {
		diff.Description = "Geometry transformed (same triangle count, different dimensions)"
	} else {
		diff.Description = "Minor modifications (same structure, different vertex data)"
	}

	return false, diff
}

type Vec3 struct {
	X, Y, Z float32
}

type Triangle struct {
	V1, V2, V3 Vec3
	Normal     Vec3
}

// STLInfo contains information about an STL file
type STLInfo struct {
	TriangleCount int
	VertexCount   int
	Bounds        Bounds
	IsBinary      bool
	Triangles     []Triangle
}

// Bounds represents the bounding box of an STL model
type Bounds struct {
	MinX, MaxX float32
	MinY, MaxY float32
	MinZ, MaxZ float32
}

// ParseSTL parses an STL file and extracts information
func ParseSTL(data []byte) (*STLInfo, error) {
	// Determine if binary or ASCII
	if isBinarySTL(data) {
		return parseBinarySTL(data)
	}
	return parseASCIISTL(data)
}

// isBinarySTL checks if STL file is in binary format
func isBinarySTL(data []byte) bool {
	// Binary STL starts with 80-byte header, then 4-byte triangle count
	if len(data) < 84 {
		return false
	}

	// Check if it starts with "solid" (ASCII format)
	if bytes.HasPrefix(data, []byte("solid")) {
		return false
	}

	return true
}

// parseBinarySTL parses a binary STL file
func parseBinarySTL(data []byte) (*STLInfo, error) {
	if len(data) < 84 {
		return nil, fmt.Errorf("file too small for binary STL")
	}

	// Read triangle count (bytes 80-83)
	triangleCount := binary.LittleEndian.Uint32(data[80:84])

	// Each triangle is 50 bytes (12 floats + 2 bytes attribute)
	expectedSize := 84 + int(triangleCount)*50
	if len(data) < expectedSize {
		return nil, fmt.Errorf("invalid binary STL: expected %d bytes, got %d", expectedSize, len(data))
	}

	info := &STLInfo{
		TriangleCount: int(triangleCount),
		VertexCount:   int(triangleCount) * 3,
		IsBinary:      true,
		Bounds: Bounds{
			MinX: math.MaxFloat32,
			MaxX: -math.MaxFloat32,
			MinY: math.MaxFloat32,
			MaxY: -math.MaxFloat32,
			MinZ: math.MaxFloat32,
			MaxZ: -math.MaxFloat32,
		},
	}

	// Parse triangles to get bounds
	offset := 84
	info.Triangles = make([]Triangle, 0, triangleCount)
	for i := 0; i < int(triangleCount); i++ {
		// Read normal vector (12 bytes)
		nx := math.Float32frombits(binary.LittleEndian.Uint32(data[offset : offset+4]))
		ny := math.Float32frombits(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		nz := math.Float32frombits(binary.LittleEndian.Uint32(data[offset+8 : offset+12]))
		offset += 12

		var tri Triangle
		tri.Normal = Vec3{nx, ny, nz}

		// Read 3 vertices (9 floats = 36 bytes)
		vertices := [3]*Vec3{&tri.V1, &tri.V2, &tri.V3}
		for v := 0; v < 3; v++ {
			x := math.Float32frombits(binary.LittleEndian.Uint32(data[offset : offset+4]))
			y := math.Float32frombits(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
			z := math.Float32frombits(binary.LittleEndian.Uint32(data[offset+8 : offset+12]))

			vertices[v].X = x
			vertices[v].Y = y
			vertices[v].Z = z

			info.Bounds.MinX = min(info.Bounds.MinX, x)
			info.Bounds.MaxX = max(info.Bounds.MaxX, x)
			info.Bounds.MinY = min(info.Bounds.MinY, y)
			info.Bounds.MaxY = max(info.Bounds.MaxY, y)
			info.Bounds.MinZ = min(info.Bounds.MinZ, z)
			info.Bounds.MaxZ = max(info.Bounds.MaxZ, z)

			offset += 12
		}
		info.Triangles = append(info.Triangles, tri)

		// Skip attribute byte count (2 bytes)
		offset += 2
	}

	return info, nil
}

// parseASCIISTL parses an ASCII STL file
func parseASCIISTL(data []byte) (*STLInfo, error) {
	lines := bytes.Split(data, []byte("\n"))

	info := &STLInfo{
		IsBinary: false,
		Bounds: Bounds{
			MinX: math.MaxFloat32,
			MaxX: -math.MaxFloat32,
			MinY: math.MaxFloat32,
			MaxY: -math.MaxFloat32,
			MinZ: math.MaxFloat32,
			MaxZ: -math.MaxFloat32,
		},
	}

	triangleCount := 0
	vertexCount := 0

	var currentTriangle Triangle
	vCount := 0
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)

		if bytes.HasPrefix(trimmed, []byte("facet normal")) {
			triangleCount++
			var nx, ny, nz float32
			_, err := fmt.Sscanf(string(trimmed), "facet normal %f %f %f", &nx, &ny, &nz)
			if err == nil {
				currentTriangle.Normal = Vec3{nx, ny, nz}
			}
		} else if bytes.HasPrefix(trimmed, []byte("vertex")) {
			vertexCount++
			vCount++

			// Parse vertex coordinates
			var x, y, z float32
			_, err := fmt.Sscanf(string(trimmed), "vertex %f %f %f", &x, &y, &z)
			if err == nil {
				v := Vec3{x, y, z}
				switch vCount {
				case 1:
					currentTriangle.V1 = v
				case 2:
					currentTriangle.V2 = v
				case 3:
					currentTriangle.V3 = v
				}

				info.Bounds.MinX = min(info.Bounds.MinX, x)
				info.Bounds.MaxX = max(info.Bounds.MaxX, x)
				info.Bounds.MinY = min(info.Bounds.MinY, y)
				info.Bounds.MaxY = max(info.Bounds.MaxY, y)
				info.Bounds.MinZ = min(info.Bounds.MinZ, z)
				info.Bounds.MaxZ = max(info.Bounds.MaxZ, z)
			}
		} else if bytes.HasPrefix(trimmed, []byte("endfacet")) {
			info.Triangles = append(info.Triangles, currentTriangle)
			vCount = 0
		}
	}

	info.TriangleCount = triangleCount
	info.VertexCount = vertexCount

	return info, nil
}

// boundsEqual checks if two bounds are approximately equal
func boundsEqual(b1, b2 Bounds) bool {
	epsilon := float32(0.001)

	return abs(b1.MinX-b2.MinX) < epsilon &&
		abs(b1.MaxX-b2.MaxX) < epsilon &&
		abs(b1.MinY-b2.MinY) < epsilon &&
		abs(b1.MaxY-b2.MaxY) < epsilon &&
		abs(b1.MinZ-b2.MinZ) < epsilon &&
		abs(b1.MaxZ-b2.MaxZ) < epsilon
}

// Helper functions
func min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
