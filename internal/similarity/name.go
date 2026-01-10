package similarity

import (
	"archive-duplicate-finder/internal/scanner"
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
// FindSimilarGroups uses an aggressive normalization strategy to cluster files efficiently (O(N))
// instead of comparing every file with every other file (O(N^2)).
func FindSimilarGroups(files []scanner.ArchiveFile, _ int, _ bool, _ bool, onProgress func(float64)) []SimilarityGroup {
	if len(files) < 2 {
		return nil
	}

	// 1. Group by "Canonical Key"
	// We map: CanonicalKey -> []ArchiveFile
	grouped := make(map[string][]scanner.ArchiveFile)
	var mu sync.Mutex

	totalFiles := len(files)
	batchSize := 1000 // Update progress every N files

	// Parallelize the normalization step if N is huge, but usually simple loop is fine.
	// For 70k files, a single thread map insert is ~50ms.
	for i, f := range files {
		key := generateCanonicalKey(f.Name)
		mu.Lock()
		grouped[key] = append(grouped[key], f)
		mu.Unlock()

		if i%batchSize == 0 && onProgress != nil {
			progress := (float64(i) / float64(totalFiles)) * 100
			onProgress(progress)
		}
	}

	if onProgress != nil {
		onProgress(90.0) // Generating keys done
	}

	// 2. Filter groups
	var results []SimilarityGroup

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

		results = append(results, SimilarityGroup{
			BaseName: key,
			Files:    group,
		})
	}

	if onProgress != nil {
		onProgress(100.0)
	}

	// Sort results by group size (descending) to show biggest clusters first
	sort.Slice(results, func(i, j int) bool {
		return len(results[i].Files) > len(results[j].Files)
	})

	return results
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
	countPart := 0
	for _, f := range files {
		lower := strings.ToLower(f.Name)
		if strings.Contains(lower, ".part") || strings.Contains(lower, ".z0") || strings.Contains(lower, ".00") {
			countPart++
		}
	}
	// If more than 50% are 'parts', it's likely a split archive
	return countPart > 1 && countPart == len(files)
}

// CalculateNameSimilarity is kept for compatibility if needed elsewhere
func CalculateNameSimilarity(name1, name2 string, debug bool) float64 {
	if generateCanonicalKey(name1) == generateCanonicalKey(name2) {
		return 100
	}
	return 0
}
