# Requirements for Creators Management and STL Previews

## 1. Overview
This project aims to extend the existing `archive-duplicate-finder` with two major features:
- **Creators Management**: Automatically identify and categorize creators/artists from images inside archives and filenames.
- **STL Preview Generation**: Automatically generate PNG previews for archives containing STL files but lacking images.

## 2. Creators Management System

### 2.1 Backend Requirements
- **Creator Discovery Service**: A background task that iterates through scanned archives and analyzes:
    - **Images**: Search for logos, URLs, or artist names within JPG/PNG/WebP files. (May require OCR or watermarking detection).
    - **Filenames**: Analyze archive names to distinguish between creator names and character names using similarity and pattern matching.
- **Database Schema**:
    - `creators` table: `id`, `name`, `url`, `logo_path`, `type` (Artist/Studio), `bio`.
    - `archive_creators` mapping table: Link archives to identified creators.
- **API Endpoints**:
    - `GET /api/creators`: List all creators.
    - `GET /api/creators/:id`: Get details and associated archives.
    - `POST /api/creators/scan`: Manually trigger or schedule the background scan.

### 2.2 Frontend Requirements (UI)
- **New Page**: `/creators`
    - Grid/List view of all creators.
    - Statistics (number of archives per creator).
    - Search and filter by creator type or name.
- **Creator Details View**:
    - Display all archives belonging to a creator.
    - Integration with existing "Reveal in Folder" and "Gallery View" functions.

## 3. STL Preview Generation

### 3.1 Backend Requirements
- **STL Analysis logic**:
    - Identify archives in the Gallery that have no associated preview image.
    - Find the largest STL file within the archive (scanning ZIP, RAR, 7Z).
- **Renderer**:
    - Implement a Go-based STL to PNG renderer (or use a lightweight CLI tool).
    - Capture a PNG screenshot of the 3D model.
- **Persistent Storage**:
    - Store the generated PNG in a persistent local directory (`.cache/previews`).
    - The directory should be relative to the application executable to remain portable.
    - Avoid modifying original archives (especially RAR/7Z) to maintain file integrity and better performance.
- **Integration**:
    - Update `internal/archive/extractor.go` to prioritize this new preview.

### 3.2 Frontend Requirements
- **Gallery Update**:
    - Seamlessly display the new STL-generated previews.
    - Visual indicator that a preview was auto-generated from an STL.

## 4. Technical Constraints & Decisions
- **Database**: Use existing SQLite cache (`archive-finder-cache.db`).
- **Language**: Go (Backend), TypeScript/Next.js (Frontend).
- **Libraries**:
    - OCR candidate: `gosseract` (requires Tesseract) or a smaller Go-native OCR.
    - 3D Rendering: `fogleman/pt` or similar, or potentially a headless browser approach.
- **Performance**: Background tasks should be throttled to avoid high CPU usage during regular usage.
