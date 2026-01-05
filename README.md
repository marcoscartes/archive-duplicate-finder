# 📦 Archive Duplicate Finder

<p align="center">
  <img src="archive_finder_hero.png" alt="Archive Duplicate Finder Banner" width="800">
</p>

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)

**Archive Duplicate Finder** is a powerful CLI tool written in Go designed to identify and manage duplicate or highly similar archive files (ZIP, RAR, 7Z). It specialized in 3D modeling workflows (like STL files) but is robust for any archive-heavy system.

> ✨ **Note:** This project was developed with the expert assistance of **Antigravity**, a powerful agentic AI coding assistant by Google Deepmind.

---

## 🚀 Features

- **🔍 Smart Scanning:** Detects ZIP, RAR, and 7Z archives recursively.
- **🌐 Interactive Web Dashboard:** Premium UI built with Next.js and Fiber for visual results management.
- **🖼️ Archive Intelligence Preview:** Hover over archives to see the first image found inside without extraction.
- **⚖️ Size Matching:** Instantly identifies files with identical byte sizes but different names.
- **🧬 Similarity Analysis:** Uses advanced algorithms (Levenshtein, Jaro-Winkler, N-Grams) to find similar filenames.
- **🔬 3D Content Awareness:** Deep-dives into archives to compare STL file geometry.
- **📂 Explorer Integration:** Open files directly with associated apps or reveal them in the system folder from the dashboard.
- **🛡️ Multi-volume Protection:** Automatically protects split archives (part1, part2) from deletion.
- **🗑️ Trash Mode:** Move duplicates to a safe folder instead of permanent deletion.
- **📝 Reference Tracking:** Leave a `.txt` file pointing to the location of the preserved original.
- **🕹️ Interactive Mode:** Take control and decide manually which duplicates to keep or remove.
- **📄 Pro Reporting:** Generates instant PDF reports even while background analysis is running.

---

## 🛠️ Installation

```bash
# Clone the repository
git clone https://github.com/marcoscartes/archive-duplicate-finder.git

# Navigate to the project
cd archive-duplicate-finder

# Install dependencies and build
go build -o archive-finder ./cmd/finder
```

---

## 📖 Usage

### Basic Scan
```bash
./archive-finder -dir "/path/to/archives"
```

### Safe Cleanup (Recommended)
```bash
# Move duplicates to a trash folder and leave a reference note
./archive-finder -dir "/path/to/archives" -delete oldest -trash "./trash" -ref -yes
```

### Interactive Cleanup
```bash
./archive-finder -dir "/path/to/archives" -interactive
```

### Web Dashboard (New!)
```bash
# Launch the scanner with a premium web interface
./archive-finder -dir "/path/to/archives" -web -port 8081
```

### PDF Reporting
```bash
./archive-finder -dir "/path/to/archives" -pdf "Scanned_Results.pdf" -threshold 85
```

### Automatic Cleanup (Be careful!)
```bash
# Deletes the oldest version of identical files
./archive-finder -dir "/path/to/archives" -delete oldest -yes
```

---

## 🧪 Modes

| Mode | Description |
| :--- | :--- |
| `all` | Runs both size and name similarity analysis (Default). |
| `size` | Only looks for files with identical sizes. |
| `name` | Only looks for files with similar names. |

---

## 🤝 Acknowledgement

This software was built and refined with the assistance of **Antigravity**, an AI agent specialized in advanced coding tasks. Antigravity helped implement:
- Parallel string similarity processing.
- Glassmorphic Web Dashboard with Next.js/Three.js.
- Real-time archive preview (On-Hover extraction).
- Multi-platform Explorer/Reveal integration.
- PDF reporting modules.
- Multi-volume archive detection logic.
- Interactive CLI resolution menus.

---

## 📜 License

This project is licensed under the MIT License - see the LICENSE file for details.
