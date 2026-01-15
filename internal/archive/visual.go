package archive

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/corona10/goimagehash"
	_ "golang.org/x/image/webp"
)

// GeneratePHash generates a perceptual hash for the given image data
func GeneratePHash(data []byte) (uint64, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, fmt.Errorf("failed to decode image: %w", err)
	}

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return 0, fmt.Errorf("failed to generate pHash: %w", err)
	}

	return hash.GetHash(), nil
}

// GenerateDHash generates a difference hash for the given image data
func GenerateDHash(data []byte) (uint64, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, fmt.Errorf("failed to decode image: %w", err)
	}

	hash, err := goimagehash.DifferenceHash(img)
	if err != nil {
		return 0, fmt.Errorf("failed to generate dHash: %w", err)
	}

	return hash.GetHash(), nil
}

// CalculateHammingDistance returns the Hamming distance between two hashes
func CalculateHammingDistance(hash1, hash2 uint64) int {
	h1 := goimagehash.NewImageHash(hash1, goimagehash.PHash)
	h2 := goimagehash.NewImageHash(hash2, goimagehash.PHash)
	distance, _ := h1.Distance(h2)
	return distance
}
