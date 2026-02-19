package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type entry struct {
	name    string
	full    string
	modTime time.Time
	isDir   bool
}

type config struct {
	repo           string
	apply          bool
	ttlLogsDays    int
	keepLogs       int
	keepReviewpack int
	keepGHA        int
	maxDelete      int
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ERROR: panic recovered: %v\n", r)
			fmt.Printf("ERROR: gc_out stopped\n")
		}
	}()

	cfg, ok := parseConfig()
	if !ok {
		return
	}
	run(cfg)
}

func parseConfig() (config, bool) {
	cfg := config{}
	flag.StringVar(&cfg.repo, "repo", "", "repo root (default: current dir)")
	flag.BoolVar(&cfg.apply, "apply", false, "apply deletions (default: dry-run)")
	flag.IntVar(&cfg.ttlLogsDays, "ttl-logs-days", 14, "delete out/logs entries older than N days")
	flag.IntVar(&cfg.keepLogs, "keep-logs", 5, "keep N newest out/logs entries")
	flag.IntVar(&cfg.keepReviewpack, "keep-reviewpack", 5, "keep latest.tar.gz + N newest review-pack-*.tar.gz")
	flag.IntVar(&cfg.keepGHA, "keep-gha", 10, "keep N newest out/gha-artifacts/<run_id>/ dirs")
	flag.IntVar(&cfg.maxDelete, "max-delete", 200, "max deletion count per section")
	flag.Parse()

	if cfg.repo == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Printf("ERROR: config reason=getwd_failed err=%v\n", err)
			return config{}, false
		}
		cfg.repo = cwd
	}
	if cfg.maxDelete <= 0 {
		fmt.Printf("ERROR: config reason=invalid_max_delete value=%d\n", cfg.maxDelete)
		return config{}, false
	}
	if cfg.ttlLogsDays < 0 || cfg.keepLogs < 0 || cfg.keepReviewpack < 0 || cfg.keepGHA < 0 {
		fmt.Printf("ERROR: config reason=negative_parameter\n")
		return config{}, false
	}
	return cfg, true
}

func run(cfg config) {
	fmt.Printf("OK: gc_out start repo=%s apply=%t\n", cfg.repo, cfg.apply)
	stop := false

	if stop {
		fmt.Printf("SKIP: step=logs reason=STOP\n")
	} else if !stepLogs(cfg) {
		stop = true
	}

	if stop {
		fmt.Printf("SKIP: step=reviewpack reason=STOP\n")
	} else if !stepReviewpack(cfg) {
		stop = true
	}

	if stop {
		fmt.Printf("SKIP: step=gha-artifacts reason=STOP\n")
	} else if !stepGHAArtifacts(cfg) {
		stop = true
	}

	if stop {
		fmt.Printf("ERROR: gc_out stopped\n")
	} else {
		fmt.Printf("OK: gc_out completed\n")
	}
}

func stepLogs(cfg config) bool {
	step := "logs"
	logsDir := filepath.Join(cfg.repo, "out", "logs")
	ents, err := listDir(logsDir)
	if err != nil {
		fmt.Printf("SKIP: step=%s reason=missing_dir path=%s\n", step, logsDir)
		return true
	}

	cutoff := time.Now().Add(-time.Duration(cfg.ttlLogsDays) * 24 * time.Hour)
	sort.Slice(ents, func(i, j int) bool { return ents[i].modTime.After(ents[j].modTime) })

	var targets []entry
	for i, e := range ents {
		if i >= cfg.keepLogs || e.modTime.Before(cutoff) {
			targets = append(targets, e)
		}
	}
	if len(targets) == 0 {
		fmt.Printf("OK: step=%s candidates=0 keep=%d ttl_days=%d\n", step, cfg.keepLogs, cfg.ttlLogsDays)
		return true
	}
	if !cfg.apply {
		fmt.Printf("SKIP: step=%s reason=apply=0 keep=%d ttl_days=%d candidates=%d\n", step, cfg.keepLogs, cfg.ttlLogsDays, len(targets))
		return true
	}

	deleted := 0
	remaining := 0
	for _, t := range targets {
		if deleted >= cfg.maxDelete {
			remaining++
			continue
		}
		if err := os.RemoveAll(t.full); err != nil {
			fmt.Printf("ERROR: step=%s reason=remove_failed path=%s err=%v\n", step, t.full, err)
			return false
		}
		deleted++
	}
	if remaining > 0 {
		fmt.Printf("SKIP: step=%s reason=max_delete_reached deleted=%d remaining=%d\n", step, deleted, remaining)
	} else {
		fmt.Printf("OK: step=%s deleted=%d\n", step, deleted)
	}
	return true
}

func stepReviewpack(cfg config) bool {
	step := "reviewpack"
	dir := filepath.Join(cfg.repo, "out", "reviewpack")
	ents, err := listDir(dir)
	if err != nil {
		fmt.Printf("SKIP: step=%s reason=missing_dir path=%s\n", step, dir)
		return true
	}

	protected := map[string]bool{
		"latest.tar.gz":          true,
		"latest-optional.tar.gz": true,
	}
	var packs []entry
	var packDirs []entry
	for _, e := range ents {
		base := filepath.Base(e.full)
		if e.isDir {
			if strings.HasPrefix(base, "review-pack-") {
				packDirs = append(packDirs, e)
			}
			continue
		}
		if protected[base] {
			continue
		}
		if strings.HasPrefix(base, "review-pack-") && strings.HasSuffix(base, ".tar.gz") {
			packs = append(packs, e)
		}
	}
	if len(packs) == 0 && len(packDirs) == 0 {
		fmt.Printf("OK: step=%s packs=0 dirs=0\n", step)
		return true
	}

	sort.Slice(packs, func(i, j int) bool { return packs[i].modTime.After(packs[j].modTime) })
	sort.Slice(packDirs, func(i, j int) bool { return packDirs[i].modTime.After(packDirs[j].modTime) })

	keepPackID := map[string]bool{}
	for i, p := range packs {
		if i < cfg.keepReviewpack {
			keepPackID[packIDFromTarBase(filepath.Base(p.full))] = true
		}
	}

	var toDeleteFiles []entry
	if len(packs) > cfg.keepReviewpack {
		toDeleteFiles = packs[cfg.keepReviewpack:]
	}

	var toDeleteDirs []entry
	for _, d := range packDirs {
		if !keepPackID[filepath.Base(d.full)] {
			toDeleteDirs = append(toDeleteDirs, d)
		}
	}

	totalDelete := len(toDeleteFiles) + len(toDeleteDirs)
	if totalDelete == 0 {
		fmt.Printf("OK: step=%s keep=%d delete=0\n", step, cfg.keepReviewpack)
		return true
	}
	if !cfg.apply {
		fmt.Printf("SKIP: step=%s reason=apply=0 keep=%d delete=%d\n", step, cfg.keepReviewpack, totalDelete)
		return true
	}

	deleted := 0
	remaining := 0
	for _, t := range toDeleteFiles {
		if deleted >= cfg.maxDelete {
			remaining++
			continue
		}
		if err := os.Remove(t.full); err != nil {
			fmt.Printf("ERROR: step=%s reason=remove_failed path=%s err=%v\n", step, t.full, err)
			return false
		}
		deleted++
	}
	for _, t := range toDeleteDirs {
		if deleted >= cfg.maxDelete {
			remaining++
			continue
		}
		if err := os.RemoveAll(t.full); err != nil {
			fmt.Printf("ERROR: step=%s reason=remove_failed path=%s err=%v\n", step, t.full, err)
			return false
		}
		deleted++
	}
	if remaining > 0 {
		fmt.Printf("SKIP: step=%s reason=max_delete_reached deleted=%d remaining=%d\n", step, deleted, remaining)
	} else {
		fmt.Printf("OK: step=%s deleted=%d\n", step, deleted)
	}
	return true
}

func packIDFromTarBase(base string) string {
	return strings.TrimSuffix(base, ".tar.gz")
}

func stepGHAArtifacts(cfg config) bool {
	step := "gha-artifacts"
	dir := filepath.Join(cfg.repo, "out", "gha-artifacts")
	ents, err := listDir(dir)
	if err != nil {
		fmt.Printf("SKIP: step=%s reason=missing_dir path=%s\n", step, dir)
		return true
	}

	var runs []entry
	for _, e := range ents {
		if e.isDir {
			runs = append(runs, e)
		}
	}
	if len(runs) == 0 {
		fmt.Printf("OK: step=%s runs=0\n", step)
		return true
	}

	sort.Slice(runs, func(i, j int) bool { return runs[i].modTime.After(runs[j].modTime) })
	var toDelete []entry
	if len(runs) > cfg.keepGHA {
		toDelete = runs[cfg.keepGHA:]
	}
	if len(toDelete) == 0 {
		fmt.Printf("OK: step=%s keep=%d delete=0\n", step, cfg.keepGHA)
		return true
	}
	if !cfg.apply {
		fmt.Printf("SKIP: step=%s reason=apply=0 keep=%d delete=%d\n", step, cfg.keepGHA, len(toDelete))
		return true
	}

	deleted := 0
	remaining := 0
	for _, t := range toDelete {
		if deleted >= cfg.maxDelete {
			remaining++
			continue
		}
		if err := os.RemoveAll(t.full); err != nil {
			fmt.Printf("ERROR: step=%s reason=remove_failed path=%s err=%v\n", step, t.full, err)
			return false
		}
		deleted++
	}
	if remaining > 0 {
		fmt.Printf("SKIP: step=%s reason=max_delete_reached deleted=%d remaining=%d\n", step, deleted, remaining)
	} else {
		fmt.Printf("OK: step=%s deleted=%d\n", step, deleted)
	}
	return true
}

func listDir(path string) ([]entry, error) {
	des, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	out := make([]entry, 0, len(des))
	for _, de := range des {
		info, err := de.Info()
		if err != nil {
			continue
		}
		out = append(out, entry{
			name:    de.Name(),
			full:    filepath.Join(path, de.Name()),
			modTime: info.ModTime(),
			isDir:   de.IsDir(),
		})
	}
	return out, nil
}
