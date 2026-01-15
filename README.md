# 📦 Archive Duplicate Finder

<p align="center">
  <img src="assets/archive_finder_hero.png" alt="Archive Duplicate Finder Banner" width="800">
</p>

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)

**Archive Duplicate Finder** is a powerful CLI tool written in Go designed to identify and manage duplicate or highly similar archive files (ZIP, RAR, 7Z). It specialized in 3D modeling workflows (like STL files) but is robust for any archive-heavy system.

> ✨ **Note:** This project was developed with the expert assistance of **Antigravity**, a powerful agentic AI coding assistant by Google Deepmind.

---

## 🚀 Features

- **⚡ Lightning Fast Caching:** SQLite-backed persistence remembers your duplicates to skip re-scanning.
- **🧠 Intelligent Clustering:** New O(N) algorithm groups similar filenames instantly, handling 100,000+ files in seconds.
- **� Smart Pagination:** Smoothly handles massive datasets with configurable page sizes (10, 20, 50, 100).
- **🔔 Live Notifications:** Receive browser alerts when background analysis finishes.
- **🖼️ Archive Intelligence 3.0:** Deep-recursive extraction with **internal browsing**. View ALL images and STL models inside an archive without extracting them. Now supporting **25+ formats** including `.fbx`, `.blend`, `.gltf`, `.glb`, `.3mf`, `.step`, `.iso` and more.
- **🎨 Cinematic Gallery Experience:** 3x3 adaptive layout with a new **Global Viewer** featuring fluid navigation, keyboard controls, and an **Internal File Selector**. Now including **advanced sorting** and **extension filtering**.
- **�🔬 Advanced 3D Geometry Studio:** Integrated professional CAD-viewer using Three.js. Features:
  - **Smart Comparison:** Stacks multiple models vertically for structural analysis.
  - **Immersive Mode:** Professional Fullscreen view with "Stage" lighting, contact shadows, and realistic PBR materials.
  - **Auto-Normalization:** Intelligent scaling to compare models of vastly different units (mm vs inches) side-by-side.
  - **Deep Archive Dive:** Extracts and renders `.stl` files directly from ZIP/RAR previews without unzipping.
- **📂 Explorer Integration:** Open files directly with associated apps or reveal them in the system folder from the dashboard.
- **🛡️ Multi-volume Protection:** Automatically protects split archives (part1, part2, .001) from deletion.
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
- **Cinematic Viewer:** Implemented a global modal system with ergonomic navigation, backdrop-blur effects, and keyboard support.
- **Robust Extractor:** Refactored extraction logic to handle nested subfolders, system folders (MACOSX) filtering, and intelligent STL fallbacks.
- **UI Consistency:** Unified centered 1000px layout and intelligent thumbnails throughout the dashboard.
- PDF reporting modules.
- Multi-volume archive detection logic.

---

## 📜 License

This project is licensed under the MIT License - see the LICENSE file for details.
