package main

import (
	"fmt"
	"sort"
	"strings"
)

// npmPackage is one npm tarball to fetch: name, version, the resolved registry
// URL, and the Subresource-Integrity digest.
type npmPackage struct {
	Name      string
	Version   string
	URL       string
	Integrity string
}

// parseYarnLock parses a Yarn classic (v1) lockfile into the set of npm tarballs
// to fetch. Entries without a `resolved` URL (e.g. the workspace root) are
// skipped. The `#<sha1>` fragment yarn appends to the URL is stripped.
func parseYarnLock(data []byte) ([]npmPackage, error) {
	var out []npmPackage
	var cur *npmPackage
	flush := func() {
		if cur != nil && cur.URL != "" && cur.Integrity != "" {
			out = append(out, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			// Entry header: `key[, key...]:` where each key is name@range.
			flush()
			name, err := nameFromHeader(strings.TrimSuffix(strings.TrimSpace(line), ":"))
			if err != nil {
				return nil, err
			}
			cur = &npmPackage{Name: name}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(line, "    ") {
			continue // nested (dependencies / optionalDependencies)
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "version "):
			cur.Version = unquote(strings.TrimPrefix(trimmed, "version "))
		case strings.HasPrefix(trimmed, "resolved "):
			u := unquote(strings.TrimPrefix(trimmed, "resolved "))
			if i := strings.IndexByte(u, '#'); i >= 0 {
				u = u[:i]
			}
			cur.URL = u
		case strings.HasPrefix(trimmed, "integrity "):
			cur.Integrity = strings.TrimSpace(strings.TrimPrefix(trimmed, "integrity "))
		}
	}
	flush()

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

// nameFromHeader extracts the package name from a yarn entry header key like
// `esbuild@^0.21.5, esbuild@~0.21.0` or `"@esbuild/linux-x64@0.21.5"`.
func nameFromHeader(h string) (string, error) {
	first := strings.TrimSpace(strings.SplitN(h, ",", 2)[0])
	first = strings.Trim(first, `"`)
	i := strings.LastIndexByte(first, '@')
	if i <= 0 { // 0 would be a bare "@scope" with no version; <0 has no '@'
		return "", fmt.Errorf("unparseable yarn.lock entry key %q", h)
	}
	return first[:i], nil
}

func unquote(s string) string {
	return strings.Trim(strings.TrimSpace(s), `"`)
}
