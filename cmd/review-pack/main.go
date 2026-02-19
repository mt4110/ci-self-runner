package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type manifest struct {
	PackID      string         `json:"pack_id"`
	Profile     string         `json:"profile"`
	GeneratedAt string         `json:"generated_at"`
	SourceRoot  string         `json:"source_root"`
	Files       []manifestFile `json:"files"`
	Notes       []string       `json:"notes"`
}

type manifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type packEntry struct {
	name    string
	full    string
	modTime time.Time
	isDir   bool
}

func main() {
	if err := run(); err != nil {
		fmt.Printf("ERROR: review-pack %s\n", err.Error())
		fmt.Println("STATUS: ERROR")
	}
}

func run() error {
	var outDirFlag string
	var profile string
	flag.StringVar(&outDirFlag, "out-dir", "out/reviewpack", "output directory for review packs")
	flag.StringVar(&profile, "profile", "core", "pack profile: core or optional")
	flag.Parse()
	profile = strings.ToLower(strings.TrimSpace(profile))
	if profile != "core" && profile != "optional" {
		return fmt.Errorf("invalid profile: %s", profile)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	now := time.Now().UTC()
	stamp := now.Format("20060102T150405Z")
	prefix := "review-pack-"
	latestName := "latest.tar.gz"
	if profile == "optional" {
		prefix = "review-pack-optional-"
		latestName = "latest-optional.tar.gz"
	}
	packID := prefix + stamp
	outDir := filepath.Join(repoRoot, outDirFlag)
	stageDir := filepath.Join(outDir, packID)
	filesDir := filepath.Join(stageDir, "files")

	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		return fmt.Errorf("mkdir files dir: %w", err)
	}

	includeRoots := []string{
		".codex",
		".cspell.json",
		".cspell-project-words.txt",
		"README.md",
		"go.mod",
		"docs/ci",
		"ci/image",
		"ci/policy",
		"cmd",
		"ops/ci",
	}
	if profile == "optional" {
		includeRoots = append(includeRoots,
			".github/workflows/verify.yml",
			"out/verify-full.status",
			"out/logs",
			"out/remote/verify-full.status",
			"out/remote/logs",
		)
	}
	relativeFiles, err := collectFiles(repoRoot, includeRoots)
	if err != nil {
		return err
	}
	if len(relativeFiles) == 0 {
		return fmt.Errorf("no files found for review pack")
	}

	manifestData := manifest{
		PackID:      packID,
		Profile:     profile,
		GeneratedAt: now.Format(time.RFC3339),
		SourceRoot:  repoRoot,
		Notes: []string{
			"Local tar.gz file does not expire.",
			"Open PACK_SUMMARY.md first when sharing with ChatGPT.",
			"Source files are under files/ preserving repository-relative paths.",
		},
	}

	for _, rel := range relativeFiles {
		src := filepath.Join(repoRoot, rel)
		dst := filepath.Join(filesDir, rel)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", rel, err)
		}
		sum, size, err := fileHashAndSize(src)
		if err != nil {
			return fmt.Errorf("hash %s: %w", rel, err)
		}
		manifestData.Files = append(manifestData.Files, manifestFile{
			Path:   rel,
			Size:   size,
			SHA256: sum,
		})
	}

	sort.Slice(manifestData.Files, func(i, j int) bool {
		return manifestData.Files[i].Path < manifestData.Files[j].Path
	})

	summaryPath := filepath.Join(stageDir, "PACK_SUMMARY.md")
	if err := os.WriteFile(summaryPath, []byte(buildSummary(manifestData)), 0o644); err != nil {
		return fmt.Errorf("write PACK_SUMMARY.md: %w", err)
	}

	manifestBytes, err := json.MarshalIndent(manifestData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	manifestPath := filepath.Join(stageDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		return fmt.Errorf("write manifest.json: %w", err)
	}

	tarPath := filepath.Join(outDir, packID+".tar.gz")
	if err := createTarGz(tarPath, stageDir); err != nil {
		return fmt.Errorf("create tar.gz: %w", err)
	}
	if err := updateLatestTar(tarPath, filepath.Join(outDir, latestName)); err != nil {
		return fmt.Errorf("update %s: %w", latestName, err)
	}
	deleted, gcErr := autoTrimReviewpack(outDir, 5)
	if gcErr != nil {
		fmt.Printf("ERROR: review-pack auto_gc err=%s\n", gcErr.Error())
	} else {
		fmt.Printf("OK: review-pack auto_gc deleted=%d keep=5\n", deleted)
	}

	fmt.Println("OK: review-pack generated")
	fmt.Println("STATUS: OK")
	fmt.Printf("PROFILE=%s\n", profile)
	fmt.Printf("PACK_ID=%s\n", packID)
	fmt.Printf("TAR_GZ=%s\n", tarPath)
	fmt.Printf("LATEST=%s\n", filepath.Join(outDir, latestName))
	fmt.Printf("SUMMARY=%s\n", summaryPath)
	return nil
}

func collectFiles(repoRoot string, roots []string) ([]string, error) {
	seen := map[string]struct{}{}
	var files []string

	for _, root := range roots {
		full := filepath.Join(repoRoot, root)
		info, err := os.Stat(full)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", root, err)
		}

		if info.IsDir() {
			err := filepath.WalkDir(full, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() {
					base := filepath.Base(path)
					if base == ".git" || base == "out" || base == "cache" || base == "tmp" {
						return filepath.SkipDir
					}
					return nil
				}
				rel, err := filepath.Rel(repoRoot, path)
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
				if _, ok := seen[rel]; ok {
					return nil
				}
				seen[rel] = struct{}{}
				files = append(files, rel)
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("walk %s: %w", root, err)
			}
			continue
		}

		rel, err := filepath.Rel(repoRoot, full)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		if _, ok := seen[rel]; !ok {
			seen[rel] = struct{}{}
			files = append(files, rel)
		}
	}

	sort.Strings(files)
	return files, nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode().Perm())
}

func fileHashAndSize(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

func buildSummary(m manifest) string {
	var b strings.Builder
	b.WriteString("# Review Pack Summary\n\n")

	b.WriteString("## Overview\n\n")
	b.WriteString("- Pack ID: `" + m.PackID + "`\n")
	b.WriteString("- Profile: `" + m.Profile + "`\n")
	b.WriteString("- Generated: `" + m.GeneratedAt + "`\n")
	b.WriteString("- Source Root: `" + m.SourceRoot + "`\n")
	b.WriteString("- This tar.gz is local and does not expire.\n\n")

	b.WriteString("## How To Use With ChatGPT\n\n")
	b.WriteString("1. Upload this `tar.gz` directly to ChatGPT.\n")
	b.WriteString("2. Ask ChatGPT to read `PACK_SUMMARY.md` first.\n")
	b.WriteString("3. Ask ChatGPT to inspect `manifest.json` and then `files/`.\n\n")

	b.WriteString("## Included Files\n\n")
	b.WriteString(fmt.Sprintf("- Total files: `%d`\n\n", len(m.Files)))
	for _, file := range m.Files {
		b.WriteString("- `files/" + file.Path + "`\n")
	}

	b.WriteString("\n## Notes\n\n")
	for _, note := range m.Notes {
		b.WriteString("- " + note + "\n")
	}
	return b.String()
}

func createTarGz(targetTarGz, sourceDir string) error {
	if err := os.MkdirAll(filepath.Dir(targetTarGz), 0o755); err != nil {
		return err
	}
	out, err := os.Create(targetTarGz)
	if err != nil {
		return err
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

func updateLatestTar(srcTarGz, latestTarGz string) error {
	if err := os.Remove(latestTarGz); err != nil && !os.IsNotExist(err) {
		return err
	}
	return copyFile(srcTarGz, latestTarGz)
}

func autoTrimReviewpack(outDir string, keep int) (int, error) {
	des, err := os.ReadDir(outDir)
	if err != nil {
		return 0, err
	}
	protected := map[string]bool{
		"latest.tar.gz":          true,
		"latest-optional.tar.gz": true,
	}
	files := []packEntry{}
	dirs := []packEntry{}
	for _, de := range des {
		info, infoErr := de.Info()
		if infoErr != nil {
			continue
		}
		name := de.Name()
		full := filepath.Join(outDir, name)
		if de.IsDir() {
			if strings.HasPrefix(name, "review-pack-") {
				dirs = append(dirs, packEntry{name: name, full: full, modTime: info.ModTime(), isDir: true})
			}
			continue
		}
		if protected[name] {
			continue
		}
		if strings.HasPrefix(name, "review-pack-") && strings.HasSuffix(name, ".tar.gz") {
			files = append(files, packEntry{name: name, full: full, modTime: info.ModTime()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.After(files[j].modTime) })
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].modTime.After(dirs[j].modTime) })

	keepID := map[string]bool{}
	for i, file := range files {
		if i < keep {
			keepID[strings.TrimSuffix(file.name, ".tar.gz")] = true
		}
	}
	toDelete := []packEntry{}
	if len(files) > keep {
		toDelete = append(toDelete, files[keep:]...)
	}
	for _, dir := range dirs {
		if !keepID[dir.name] {
			toDelete = append(toDelete, dir)
		}
	}
	deleted := 0
	for _, old := range toDelete {
		if err := os.RemoveAll(old.full); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}
