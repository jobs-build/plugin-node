package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// packageLock is the subset of npm's package-lock.json (lockfileVersion 2/3) we
// need: the `packages` map keyed by install path ("" = root project,
// "node_modules/<name>", or nested "node_modules/a/node_modules/b").
type packageLock struct {
	Packages map[string]struct {
		Version   string `json:"version"`
		Resolved  string `json:"resolved"`
		Integrity string `json:"integrity"`
	} `json:"packages"`
}

// parsePackageLock parses an npm package-lock.json (v2/v3) into the set of npm
// tarballs to fetch — one per `packages` entry with a registry `resolved` URL and
// an `integrity` digest. The "" root and link/workspace/bundled entries (no
// resolved) are skipped. The package name is the path after the LAST
// "node_modules/" in the key, so nested and scoped names resolve correctly.
// Output is deduped by name@version and sorted, matching parseYarnLock.
func parsePackageLock(data []byte) ([]npmPackage, error) {
	var lock packageLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse package-lock.json: %w", err)
	}
	if lock.Packages == nil {
		return nil, fmt.Errorf("package-lock.json has no \"packages\" map (need lockfileVersion 2 or 3)")
	}

	var out []npmPackage
	for path, p := range lock.Packages {
		if p.Resolved == "" || p.Integrity == "" {
			continue // "" root, workspace links, bundled deps
		}
		if !strings.HasPrefix(p.Resolved, "http") {
			continue // git+/file: deps are not registry tarballs
		}
		name := path
		if i := strings.LastIndex(path, "node_modules/"); i >= 0 {
			name = path[i+len("node_modules/"):]
		}
		if name == "" {
			continue
		}
		out = append(out, npmPackage{Name: name, Version: p.Version, URL: p.Resolved, Integrity: p.Integrity})
	}

	seen := map[string]bool{}
	var deduped []npmPackage
	for _, p := range out {
		k := p.Name + "@" + p.Version
		if seen[k] {
			continue
		}
		seen[k] = true
		deduped = append(deduped, p)
	}
	sort.Slice(deduped, func(i, j int) bool {
		if deduped[i].Name != deduped[j].Name {
			return deduped[i].Name < deduped[j].Name
		}
		return deduped[i].Version < deduped[j].Version
	})
	return deduped, nil
}
