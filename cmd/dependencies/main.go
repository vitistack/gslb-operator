package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

type module struct {
	Path     string
	Version  string
	Versions []string
	Time     *time.Time
	Main     bool
	Indirect bool
}

func main() {
	days := flag.Uint("days", 7, "minimum age in days a dependency must have before it is eligible for update")
	dryRun := flag.Bool("dry-run", false, "report available updates without changing go.mod")
	flag.Parse()
	minAge := time.Duration(*days) * 24 * time.Hour

	deps, err := directDependencies()
	if err != nil {
		log.Fatalf("listing dependencies: %s", err.Error())
	}

	changed := false
	for _, dep := range deps {
		target, err := safeUpdateDependency(dep, minAge)
		if err != nil {
			log.Printf("skipping %s: reason: %s", dep.Path, err.Error())
			continue
		}

		if target == "" {
			continue
		}

		fmt.Printf("%s: %s -> %s\n", dep.Path, dep.Version, target)
		if *dryRun {
			continue
		}

		if err := runGo("get", dep.Path+"@"+target); err != nil {
			log.Printf("updating %s: %v", dep.Path, err)
			continue
		}
		changed = true
	}

	if changed && !*dryRun {
		if err := runGo("mod", "tidy"); err != nil {
			log.Fatalf("go mod tidy: %s", err.Error())
		}
	}

}

func directDependencies() ([]module, error) {
	out, err := runGoOutput("list", "-m", "-json", "all")
	if err != nil {
		return nil, err
	}

	mods := make([]module, 0)
	decoder := json.NewDecoder(bytes.NewReader(out))
	for {
		var m module
		if err := decoder.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if m.Main || m.Indirect {
			continue
		}

		mods = append(mods, m)
	}
	return mods, nil
}

func safeUpdateDependency(dep module, minAge time.Duration) (string, error) {
	out, err := runGoOutput("list", "-m", "-versions", "-json", dep.Path)
	if err != nil {
		return "", err
	}

	var info module
	if err := json.Unmarshal(out, &info); err != nil {
		return "", err
	}

	idx := -1
	for i, v := range info.Versions {
		if v == dep.Version {
			idx = i
			break
		}
	}
	if idx == -1 {
		return "", fmt.Errorf("current version %s not found in tagged release list", dep.Version)
	}

	cutoff := time.Now().Add(-minAge)

	// since versions are sorted oldest to newest; scan from the newest candidate
	// dso we pick the freshest version that still clears the age
	for i := len(info.Versions) - 1; i > idx; i-- {
		v := info.Versions[i]
		published, err := publishedAt(dep.Path, v)
		if err != nil {
			return "", err
		}
		if published.Before(cutoff) {
			return v, nil
		}
	}
	return "", nil
}

func publishedAt(path, version string) (time.Time, error) {
	out, err := runGoOutput("list", "-m", "-json", path+"@"+version)
	if err != nil {
		return time.Time{}, err
	}

	var m module
	if err := json.Unmarshal(out, &m); err != nil {
		return time.Time{}, err
	}
	if m.Time == nil {
		return time.Time{}, fmt.Errorf("no publish time reported for %s@%s", path, version)
	}
	return *m.Time, nil
}

func runGo(args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGoOutput(args ...string) ([]byte, error) {
	cmd := exec.Command("go", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}

	return out, nil
}
