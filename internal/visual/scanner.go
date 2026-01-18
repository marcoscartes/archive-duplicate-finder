package visual

import (
	"archive-duplicate-finder/internal/archive"
	"archive-duplicate-finder/internal/db"
	"archive-duplicate-finder/internal/scanner"
	"fmt"
	"log"
	"sync"
	"time"
)

// ProcessVisualHashes iterates over files and computes visual hashes if they are missing
func ProcessVisualHashes(files []scanner.ArchiveFile, cache *db.Cache, debug bool, onProgress func(float64)) {
	if cache == nil {
		return
	}

	total := len(files)
	var processed int
	var mu sync.Mutex

	// Use a worker pool to avoid resource exhaustion
	workerCount := 4
	jobs := make(chan scanner.ArchiveFile, total)
	var wg sync.WaitGroup

	for w := 1; w <= workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("🔥 CRITICAL RECOVERY: Analysis worker recovered from panic: %v", r)
				}
			}()
			for f := range jobs {
				modTime := f.ModTime.Format(time.RFC3339)

				// Check cache first
				if _, ok := cache.GetVisualHash(f.Path, modTime); ok {
					mu.Lock()
					processed++
					if onProgress != nil {
						onProgress(float64(processed) / float64(total) * 100)
					}
					mu.Unlock()
					continue
				}

				if debug {
					log.Printf("[VISUAL] Processing %s", f.Name)
				}

				// Try to extract preview
				data, _, err := archive.FindPreviewInArchive(f.Path)
				if err != nil {
					if debug {
						log.Printf("[VISUAL] Skipped %s: %v", f.Name, err)
					}
				} else {
					// Generate pHash
					phash, err := archive.GeneratePHash(data)
					if err != nil {
						if debug {
							log.Printf("[VISUAL] Hash error %s: %v", f.Name, err)
						}
					} else {
						// Store in cache
						cache.PutVisualHash(f.Path, phash, modTime)
					}
				}

				mu.Lock()
				processed++
				if onProgress != nil {
					onProgress(float64(processed) / float64(total) * 100)
				}
				mu.Unlock()
			}
		}()
	}

	for _, f := range files {
		jobs <- f
	}
	close(jobs)
	wg.Wait()
}

// FindVisualDuplicates groups files that are visually similar using Hamming distance
func FindVisualDuplicates(files []scanner.ArchiveFile, cache *db.Cache, threshold int) []SimilarityGroup {
	if cache == nil || len(files) < 2 {
		return nil
	}

	// 1. Collect all hashes from cache
	type fileHash struct {
		file scanner.ArchiveFile
		hash uint64
	}
	var hashes []fileHash

	for _, f := range files {
		modTime := f.ModTime.Format(time.RFC3339)
		if h, ok := cache.GetVisualHash(f.Path, modTime); ok {
			hashes = append(hashes, fileHash{file: f, hash: h})
		}
	}

	if len(hashes) < 2 {
		return nil
	}

	// 2. Cluster using Hamming Distance (Simple Greedy Clustering)
	// threshold for Hamming distance (e.g., 5 means highly similar for a 64-bit hash)
	hammingThreshold := 8
	visited := make(map[string]bool)
	var groups []SimilarityGroup

	for i := 0; i < len(hashes); i++ {
		if visited[hashes[i].file.Path] {
			continue
		}

		currentGroup := []scanner.ArchiveFile{hashes[i].file}
		visited[hashes[i].file.Path] = true

		for j := i + 1; j < len(hashes); j++ {
			if visited[hashes[j].file.Path] {
				continue
			}

			dist := archive.CalculateHammingDistance(hashes[i].hash, hashes[j].hash)
			if dist <= hammingThreshold {
				currentGroup = append(currentGroup, hashes[j].file)
				visited[hashes[j].file.Path] = true
			}
		}

		if len(currentGroup) > 1 {
			// Convert to reporting format
			var fileInfos []FileInfo
			for _, f := range currentGroup {
				modTime := f.ModTime.Format(time.RFC3339)
				h, _ := cache.GetVisualHash(f.Path, modTime)
				fileInfos = append(fileInfos, FileInfo{
					Name:    f.Name,
					Path:    f.Path,
					Size:    f.Size,
					Type:    f.Type,
					ModTime: modTime,
					PHash:   h,
				})
			}

			groups = append(groups, SimilarityGroup{
				BaseName: fmt.Sprintf("Visual Match: %s", currentGroup[0].Name),
				Files:    fileInfos,
			})
		}
	}

	return groups
}

// SimilarityGroup and FileInfo aliases to avoid package cycles or use reporter directly
type SimilarityGroup struct {
	BaseName string
	Files    []FileInfo
}

type FileInfo struct {
	Name    string
	Path    string
	Size    int64
	Type    string
	ModTime string
	PHash   uint64
}
