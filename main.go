// Command nodeplugin is a JOBS build plugin (build.md §6) for Node/Yarn projects.
// It reads a CBOR request {call:{yarn_lock:<bytes>}, source} on stdin, turns a
// Yarn (v1) lockfile into one import spec per package (fetcher "npm", pinned by
// the Subresource-Integrity digest), and writes the CBOR response (an array of
// {name, version, input}) on stdout. Network-free and statically linked (CGO
// disabled), so it runs in the hermetic plugin sandbox.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jobs-build/plugin-node/internal/importdef"
	"github.com/fxamacker/cbor/v2"
)

type request struct {
	Call   map[string]any `cbor:"call"`
	Source string         `cbor:"source"`
}

type inputSpec struct {
	Kind       string `cbor:"kind"`
	Definition []byte `cbor:"definition"`
}

type pkgOut struct {
	Name    string    `cbor:"name"`
	Version string    `cbor:"version"`
	Input   inputSpec `cbor:"input"`
}

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "nodeplugin:", err)
		os.Exit(1)
	}
}

func run(stdin io.Reader, stdout io.Writer) error {
	in, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	var req request
	if err := cbor.Unmarshal(in, &req); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}

	var lock []byte
	switch v := req.Call["yarn_lock"].(type) {
	case []byte:
		lock = v
	case string:
		lock = []byte(v)
	default:
		return fmt.Errorf("yarn_lock kwarg missing or not bytes/string (got %T)", req.Call["yarn_lock"])
	}

	pkgs, err := parseYarnLock(lock)
	if err != nil {
		return err
	}
	out := make([]pkgOut, 0, len(pkgs))
	for _, p := range pkgs {
		spec, err := pkgInput(p)
		if err != nil {
			return fmt.Errorf("package %s %s: %w", p.Name, p.Version, err)
		}
		out = append(out, pkgOut{Name: p.Name, Version: p.Version, Input: spec})
	}

	resp, err := cbor.Marshal(out)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	if _, err := stdout.Write(resp); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	return nil
}

// pkgInput builds the npm import spec: a canonical importdef for fetcher "npm"
// with params {name, version, url, integrity}.
func pkgInput(p npmPackage) (inputSpec, error) {
	params, err := importdef.CanonicalParams(map[string]any{
		"name":      p.Name,
		"version":   p.Version,
		"url":       p.URL,
		"integrity": p.Integrity,
	})
	if err != nil {
		return inputSpec{}, err
	}
	def, err := importdef.Definition{Fetcher: "npm", Params: params}.Canonical()
	if err != nil {
		return inputSpec{}, err
	}
	return inputSpec{Kind: "import", Definition: def}, nil
}
