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

- **⚡ Lightning Fast Caching:** SQLite-backed persistence remembers your duplicates to skip re-scanning.
- **🧠 Intelligent Clustering:** New O(N) algorithm groups similar filenames instantly, handling 100,000+ files in seconds.
- **🚀 Turbo Mode:** Experimental high-performance Bit-Parallel engine for extreme concurrent throughput.
- **📊 Smart Pagination:** Smoothly handles massive datasets with configurable page sizes (10, 20, 50, 100).
- **🔔 Live Notifications:** Receive browser alerts when background analysis finishes.
- **🌐 Interactive Web Dashboard:** Premium UI built with Next.js and Fiber for visual results management.
- **⏳ Real-time Progress:** Visual progress bars for long-running tasks in both CLI and Web Dashboard.
- **🖼️ Archive Intelligence Preview:** Hover over archives to see the first image found inside without extraction.
- **⚖️ Size Matching:** Instantly identifies files with identical byte sizes but different names.
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

# Build the dashboard (Requires Node.js)
cd ui
npm install
npm run build
cd ..
```

---

## 📖 Usage

### Basic Scan
```bash
./archive-finder -dir "D:/Archives"
```

### Web Dashboard (Recommended)
```bash
# Launch the premium web interface
./archive-finder -dir "D:/Archives" -web
```
*Step 3 (Similarity Analysis) is now on-demand from the dashboard for massive performance gains.*

### Safe Cleanup
```bash
# Move duplicates to a trash folder and leave a reference note
./archive-finder -dir "D:/Archives" -delete oldest -trash "./trash" -ref -yes
```

### Check Similar Names (CLI)
```bash
# Run clustering analysis immediately without dashboard
./archive-finder -dir "D:/Archives" -check-similar
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
- **Optimization V3:** Replaced O(N²) pairwise comparison with O(N) Canonical Clustering for massive datasets.
- **Dynamic Dashboard:** React/Next.js dashboard with real-time progress bars and on-demand analysis triggers.
- Parallel string similarity processing with Bit-Parallel Myers Algorithm.
- Glassmorphic Web Dashboard with Next.js/Three.js.
- Real-time archive preview (On-Hover extraction).
- Multi-platform Explorer/Reveal integration.
- Smart pagination and data virtualization.
- PDF reporting modules.
- Multi-volume archive detection logic.

---

## 📜 License

This project is licensed under the MIT License - see the LICENSE file for details.
