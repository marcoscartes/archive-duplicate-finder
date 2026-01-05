package similarity

import (
	"archive-duplicate-finder/internal/scanner"
	"math"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"
)

// SimilarPair represents a pair of files with similar names
type SimilarPair struct {
	File1      scanner.ArchiveFile
	File2      scanner.ArchiveFile
	Similarity float64
}

// NormalizedFile wraps an ArchiveFile with its pre-normalized name
type NormalizedFile struct {
	File           scanner.ArchiveFile
	NormalizedName string
}

// FindSimilarNames finds pairs of files with similar names but different sizes using parallel processing
func FindSimilarNames(files []scanner.ArchiveFile, threshold int) []SimilarPair {
	if len(files) < 2 {
		return nil
	}

	// 1. Pre-normalize all names
	normalized := make([]NormalizedFile, len(files))
	for i, f := range files {
		normalized[i] = NormalizedFile{
			File:           f,
			NormalizedName: normalizeFilename(f.Name),
		}
	}

	// 2. Setup parallel processing
	numWorkers := runtime.NumCPU()
	var wg sync.WaitGroup
	pairsChan := make(chan SimilarPair, 1000)

	// Work distribution: Split the outer loop among workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Each worker handles a subset of the outer loop
			for i := workerID; i < len(normalized); i += numWorkers {
				f1 := normalized[i]

				for j := i + 1; j < len(normalized); j++ {
					f2 := normalized[j]

					// Skip if same size (likely handled by Step 2)
					if f1.File.Size == f2.File.Size {
						continue
					}

					// Fast path: quick length check
					len1 := utf8.RuneCountInString(f1.NormalizedName)
					len2 := utf8.RuneCountInString(f2.NormalizedName)
					if len1 > 0 && len2 > 0 {
						ratio := float64(len1) / float64(len2)
						if ratio < 0.4 || ratio > 2.5 {
							continue
						}
					}

					// Perform comparison
					similarity := CalculateNormalizedSimilarity(f1.NormalizedName, f2.NormalizedName)

					if similarity >= float64(threshold) {
						pairsChan <- SimilarPair{
							File1:      f1.File,
							File2:      f2.File,
							Similarity: similarity,
						}
					}
				}
			}
		}(w)
	}

	// Collect results in a separate goroutine
	resultsWg := sync.WaitGroup{}
	var pairs []SimilarPair
	resultsWg.Add(1)
	go func() {
		defer resultsWg.Done()
		for p := range pairsChan {
			pairs = append(pairs, p)
		}
	}()

	wg.Wait()
	close(pairsChan)
	resultsWg.Wait()

	return pairs
}

// CalculateNormalizedSimilarity calculates similarity between two already normalized strings
func CalculateNormalizedSimilarity(norm1, norm2 string) float64 {
	if norm1 == norm2 {
		return 100.0
	}

	// Use multiple algorithms and average the results
	lev := levenshteinSimilarity(norm1, norm2)
	jaro := jaroWinklerSimilarity(norm1, norm2)
	ngram := ngramSimilarity(norm1, norm2, 2)

	// Weighted average (Levenshtein is most reliable for filenames)
	similarity := (lev*0.5 + jaro*0.3 + ngram*0.2) * 100

	return math.Round(similarity*10) / 10 // Round to 1 decimal
}

// CalculateNameSimilarity calculates similarity between two raw filenames
func CalculateNameSimilarity(name1, name2 string) float64 {
	return CalculateNormalizedSimilarity(normalizeFilename(name1), normalizeFilename(name2))
}

// normalizeFilename removes extension and converts to lowercase
func normalizeFilename(filename string) string {
	// Remove extension
	name := filename
	if idx := strings.LastIndex(filename, "."); idx != -1 {
		name = filename[:idx]
	}

	// Convert to lowercase
	name = strings.ToLower(name)

	// Remove common version indicators
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	return strings.TrimSpace(name)
}

// levenshteinSimilarity calculates Levenshtein distance-based similarity
func levenshteinSimilarity(s1, s2 string) float64 {
	distance := levenshteinDistance(s1, s2)
	maxLen := math.Max(float64(utf8.RuneCountInString(s1)), float64(utf8.RuneCountInString(s2)))

	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - (float64(distance) / maxLen)
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func levenshteinDistance(s1, s2 string) int {
	r1 := []rune(s1)
	r2 := []rune(s2)

	len1 := len(r1)
	len2 := len(r2)

	// Create matrix
	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}

	// Initialize first row and column
	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len1][len2]
}

// jaroWinklerSimilarity calculates Jaro-Winkler similarity
func jaroWinklerSimilarity(s1, s2 string) float64 {
	jaro := jaroSimilarity(s1, s2)

	// Calculate common prefix length (up to 4 characters)
	prefixLen := 0
	for i := 0; i < min(len(s1), len(s2), 4); i++ {
		if s1[i] == s2[i] {
			prefixLen++
		} else {
			break
		}
	}

	// Jaro-Winkler formula
	return jaro + (float64(prefixLen) * 0.1 * (1.0 - jaro))
}

// jaroSimilarity calculates Jaro similarity
func jaroSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	len1 := len(s1)
	len2 := len(s2)

	if len1 == 0 || len2 == 0 {
		return 0.0
	}

	// Maximum allowed distance
	matchDistance := max(len1, len2)/2 - 1
	if matchDistance < 0 {
		matchDistance = 0
	}

	s1Matches := make([]bool, len1)
	s2Matches := make([]bool, len2)

	matches := 0
	transpositions := 0

	// Find matches
	for i := 0; i < len1; i++ {
		start := max(0, i-matchDistance)
		end := min(i+matchDistance+1, len2)

		for j := start; j < end; j++ {
			if s2Matches[j] || s1[i] != s2[j] {
				continue
			}
			s1Matches[i] = true
			s2Matches[j] = true
			matches++
			break
		}
	}

	if matches == 0 {
		return 0.0
	}

	// Count transpositions
	k := 0
	for i := 0; i < len1; i++ {
		if !s1Matches[i] {
			continue
		}
		for !s2Matches[k] {
			k++
		}
		if s1[i] != s2[k] {
			transpositions++
		}
		k++
	}

	// Jaro formula
	return (float64(matches)/float64(len1) +
		float64(matches)/float64(len2) +
		float64(matches-transpositions/2)/float64(matches)) / 3.0
}

// ngramSimilarity calculates n-gram based similarity
func ngramSimilarity(s1, s2 string, n int) float64 {
	ngrams1 := getNgrams(s1, n)
	ngrams2 := getNgrams(s2, n)

	if len(ngrams1) == 0 && len(ngrams2) == 0 {
		return 1.0
	}

	if len(ngrams1) == 0 || len(ngrams2) == 0 {
		return 0.0
	}

	// Count common n-grams
	common := 0
	for ng := range ngrams1 {
		if ngrams2[ng] {
			common++
		}
	}

	// Jaccard similarity
	total := len(ngrams1) + len(ngrams2) - common
	if total == 0 {
		return 0.0
	}

	return float64(common) / float64(total)
}

// getNgrams generates n-grams from a string
func getNgrams(s string, n int) map[string]bool {
	ngrams := make(map[string]bool)

	if len(s) < n {
		ngrams[s] = true
		return ngrams
	}

	for i := 0; i <= len(s)-n; i++ {
		ngrams[s[i:i+n]] = true
	}

	return ngrams
}

// Helper functions
func min(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func max(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
