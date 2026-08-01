package similarity

import (
	"archive-duplicate-finder/internal/scanner"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// SimilarityGroup represents a cluster of files that share a similar canonical name
type SimilarityGroup struct {
	BaseName string
	Files    []scanner.ArchiveFile
}

// FindSimilarGroups uses an aggressive normalization strategy to cluster files efficiently (O(N))
// instead of comparing every file with every other file (O(N^2)).
func FindSimilarGroups(files []scanner.ArchiveFile, _ int, _ bool, onProgress func(float64)) []SimilarityGroup {
	if len(files) < 2 {
		return nil
	}

	// 1. Group by "Canonical Key" - Parallel version
	// We map: CanonicalKey -> []ArchiveFile
	grouped := make(map[string][]scanner.ArchiveFile)
	var mu sync.Mutex

	totalFiles := len(files)

	// Parallel grouping using worker pool
	numWorkers := 8
	if totalFiles < 1000 {
		numWorkers = 2
	}

	jobs := make(chan scanner.ArchiveFile, len(files))
	results := make(chan struct {
		key  string
		file scanner.ArchiveFile
	}, len(files))
	var wg sync.WaitGroup

	// Workers to generate keys in parallel
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				key := generateCanonicalKey(f.Name)
				results <- struct {
					key  string
					file scanner.ArchiveFile
				}{key, f}
			}
		}()
	}

	// Collector goroutine
	go func() {
		wg.Wait()
		close(results)
	}()

	// Send jobs
	go func() {
		for _, f := range files {
			jobs <- f
		}
		close(jobs)
	}()

	// Collect results and group
	processedCount := 0
	for result := range results {
		mu.Lock()
		grouped[result.key] = append(grouped[result.key], result.file)
		mu.Unlock()

		processedCount++
		if processedCount%1000 == 0 && onProgress != nil {
			progress := (float64(processedCount) / float64(totalFiles)) * 100
			onProgress(progress)
		}
	}

	if onProgress != nil {
		onProgress(90.0) // Generating keys done
	}

	// 2. Filter groups
	var results2 []SimilarityGroup

	totalGroups := len(grouped)
	processedGroups := 0

	for key, group := range grouped {
		processedGroups++
		// Simple progress check for filtering phase
		if processedGroups%100 == 0 && onProgress != nil {
			// Map remaining 10% to filtering phase
			baseProgress := 90.0
			phaseProgress := (float64(processedGroups) / float64(totalGroups)) * 10.0
			onProgress(baseProgress + phaseProgress)
		}

		if len(group) < 2 {
			continue
		}

		// Sort by name for consistency
		sort.Slice(group, func(i, j int) bool {
			return group[i].Name < group[j].Name
		})

		// Check if they are just multi-volume parts of the SAME archive
		if areAllMultiVolumePartsOfSameSet(group) {
			continue
		}

		results2 = append(results2, SimilarityGroup{
			BaseName: key,
			Files:    group,
		})
	}

	if onProgress != nil {
		onProgress(100.0)
	}

	// Sort results by group size (descending) to show biggest clusters first
	sort.Slice(results2, func(i, j int) bool {
		return len(results2[i].Files) > len(results2[j].Files)
	})

	return results2
}

// generateCanonicalKey reduces a filename to its "essence" to find matches.
func generateCanonicalKey(name string) string {
	// 1. Lowercase
	s := strings.ToLower(name)

	// 2. Remove extension
	if idx := strings.LastIndex(s, "."); idx != -1 {
		s = s[:idx]
	}

	// 3. Replace common separators with space
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, ".", " ")
	s = strings.ReplaceAll(s, "+", " ")
	s = strings.ReplaceAll(s, "[", " ")
	s = strings.ReplaceAll(s, "]", " ")
	s = strings.ReplaceAll(s, "(", " ")
	s = strings.ReplaceAll(s, ")", " ")

	// 4. Remove common "noise" words using Regex
	// We want to remove version numbers (v1, 1.0, etc), "copy", "backup", date stamps somewhat.
	// Regex: Remove "v" followed by digits
	reVersion := regexp.MustCompile(`\bv\d+(\.\d+)*\b`)
	s = reVersion.ReplaceAllString(s, "")

	// Remove isolated numbers
	reNumbers := regexp.MustCompile(`\b\d+\b`)
	s = reNumbers.ReplaceAllString(s, "")

	// Remove specific keywords
	keywords := []string{"copy", "backup", "old", "new", "final", "temp", "tmp", "archive", "rar", "zip"}
	words := strings.Fields(s)
	var cleanWords []string

	for _, w := range words {
		isKeyword := false
		for _, k := range keywords {
			if w == k {
				isKeyword = true
				break
			}
		}
		if !isKeyword {
			cleanWords = append(cleanWords, w)
		}
	}

	return strings.Join(cleanWords, " ")
}

func areAllMultiVolumePartsOfSameSet(files []scanner.ArchiveFile) bool {
	if len(files) < 2 {
		return false
	}

	sets := make(map[string]int)
	for _, f := range files {
		isPart, base, _ := f.IsMultiVolumePart()
		if !isPart {
			return false // At least one is not a part, so the group might be valid
		}
		// Set ID includes directory to distinguish parts of the same archive
		setID := filepath.Join(filepath.Dir(f.Path), base)
		sets[setID]++
	}

	// If there's only one set represented, they are just parts of that archive
	// If there are multiple sets, it means we found parts with similar names across different sets/folders
	return len(sets) == 1
}

// CalculateNameSimilarity is kept for compatibility if needed elsewhere
func CalculateNameSimilarity(name1, name2 string, debug bool) float64 {
	if generateCanonicalKey(name1) == generateCanonicalKey(name2) {
		return 100
	}
	return 0
}
