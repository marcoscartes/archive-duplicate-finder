package reporter

import (
	"encoding/json"
	"fmt"
	"os"
)

// Report represents the analysis results
type Report struct {
	TotalFiles       int           `json:"total_files"`
	SizeGroups       []SizeGroup   `json:"size_groups"`
	SimilarPairs     []SimilarPair `json:"similar_pairs"`
	AnalysisDuration float64       `json:"analysis_duration_seconds"`
	Timestamp        string        `json:"timestamp"`
}

// SizeGroup represents files with identical size
type SizeGroup struct {
	Size  int64      `json:"size"`
	Files []FileInfo `json:"files"`
}

// SimilarPair represents files with similar names
type SimilarPair struct {
	File1      FileInfo `json:"file1"`
	File2      FileInfo `json:"file2"`
	Similarity float64  `json:"similarity"`
}

// FileInfo represents basic file information
type FileInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	Type string `json:"type"`
}

// ExportJSON exports the report to a JSON file
func ExportJSON(report Report, filename string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// PrintSummary prints a summary of the analysis
func PrintSummary(report Report) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📈 ANALYSIS SUMMARY")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📦 Total files analyzed: %d\n", report.TotalFiles)
	fmt.Printf("🔄 Size groups found: %d\n", len(report.SizeGroups))
	fmt.Printf("📝 Similar name pairs: %d\n", len(report.SimilarPairs))
	fmt.Printf("⏱️  Analysis duration: %.2fs\n", report.AnalysisDuration)
	fmt.Println()
}
