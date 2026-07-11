package main

import "testing"

// Aliased entries ("alias@npm:real@version") must take their identity from
// the resolved URL — yarn's offline-mirror lookup derives the filename from
// the URL, so a header-derived alias name would mis-name the mirror file.
func TestParseYarnLockAliasedEntry(t *testing.T) {
	lock := `# yarn lockfile v1

"@gitlab/vue-router-vue3@npm:vue-router@4.5.1":
  version "4.5.1"
  resolved "https://registry.yarnpkg.com/vue-router/-/vue-router-4.5.1.tgz#47bffe2d"
  integrity sha512-og==
  dependencies:
    "@vue/devtools-api" "^6.6.4"

"@esbuild/linux-x64@0.25.12":
  version "0.25.12"
  resolved "https://registry.yarnpkg.com/@esbuild/linux-x64/-/linux-x64-0.25.12.tgz#ab"
  integrity sha512-AP==
`
	pkgs, err := parseYarnLock([]byte(lock))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("got %d packages", len(pkgs))
	}
	byName := map[string]npmPackage{}
	for _, p := range pkgs {
		byName[p.Name] = p
	}
	if _, ok := byName["vue-router"]; !ok {
		t.Fatalf("aliased entry must resolve to its real name; got %v", pkgs)
	}
	if byName["vue-router"].Version != "4.5.1" {
		t.Fatalf("vue-router version %q", byName["vue-router"].Version)
	}
	if _, ok := byName["@esbuild/linux-x64"]; !ok {
		t.Fatalf("scoped name must keep its scope; got %v", pkgs)
	}
}
