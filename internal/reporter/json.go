package reporter

import (
	"encoding/json"
	"fmt"
	"os"
)

// Report represents the analysis results
type Report struct {
	TotalFiles       int               `json:"total_files"`
	SizeGroups       []SizeGroup       `json:"size_groups"`
	SimilarGroups    []SimilarityGroup `json:"similar_groups"`
	SimilarCount     int               `json:"similar_count"`
	VisualGroups     []SimilarityGroup `json:"visual_groups"`
	VisualCount      int               `json:"visual_count"`
	AnalysisDuration float64           `json:"analysis_duration_seconds"`
	Timestamp        string            `json:"timestamp"`
	Status           string            `json:"status"`   // "analyzing", "finished"
	Progress         float64           `json:"progress"` // 0.0 to 100.0
}

// SizeGroup represents files with identical size
type SizeGroup struct {
	Size  int64      `json:"size"`
	Files []FileInfo `json:"files"`
}

// SimilarityGroup represents a cluster of similar files
type SimilarityGroup struct {
	BaseName string     `json:"base_name"`
	Files    []FileInfo `json:"files"`
}

// FileInfo represents basic file information
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Type    string `json:"type"`
	ModTime string `json:"mod_time"`
	PHash   uint64 `json:"p_hash,omitempty"`
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
	fmt.Printf("📝 Similar groups found: %d\n", len(report.SimilarGroups))
	fmt.Printf("⏱️  Analysis duration: %.2fs\n", report.AnalysisDuration)
	fmt.Println()
}
