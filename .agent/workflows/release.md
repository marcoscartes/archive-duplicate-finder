---
description: how to release a new version of the application
---

1. Ensure the `VERSION` file has the correct version number (e.g., 1.8.0).
2. Prepare a short summary of the changes.
// turbo
3. Run the release script using PowerShell:
   `powershell -ExecutionPolicy Bypass -File .\make_release.ps1 "Your release notes here"`

This script will automatically:
- Build the React UI.
- Build the Go binaries for Windows, Linux, and macOS (Intel & Silicon).
- Tag the repository.
- Push the changes and tags to GitHub.
- Create a new GitHub Release with all the binaries attached.
