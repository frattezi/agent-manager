package main

import (
	"flag"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"slices"
	"strings"
)

const module = "github.com/YoanWai/agent-manager"

// platforms are the GOOS values .goreleaser.yaml builds. A dependency behind
// a build tag on one of them is still a dependency.
var platforms = []string{"darwin", "linux", "windows"}

// runtimePackages are what only a runtime package may import. A feature
// package returns values; the root runs store, tmux, git and SSH work.
var runtimePackages = []string{"/internal/ui", "/internal/git", "/internal/store", "/internal/tmux", "/internal/execution", "/internal/app", "/internal/status", "/internal/catalog", "/internal/config", "/internal/remote", "/internal/sessioncmd"}

type boundary struct {
	name    string
	path    string
	runtime bool
	allowed []string
	// refused are imports the package may not make itself, even where a
	// package it imports makes them. A path that starts with / is within
	// the module.
	refused []string
}

func main() {
	write := flag.Bool("write", false, "rewrite boundaries.go from the current graphs, refusing a runtime import in a feature package")
	flag.Parse()
	if *write {
		if err := writeBoundaries("tools/architecture/check-ui-boundaries/boundaries.go"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	for _, feature := range boundaries {
		if err := checkBoundary(feature); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("PASS: %s production dependencies stay within its explicit boundary\n", feature.name)
	}
}

func checkBoundary(feature boundary) error {
	dependencies, err := graph(feature)
	if err != nil {
		return err
	}
	if err := checkBoundaryDependencies(feature, dependencies); err != nil {
		return err
	}
	imports, err := listed(feature, `{{join .Imports " "}}`, false)
	if err != nil {
		return err
	}
	return checkRefusedImports(feature, imports)
}

// graph is the package's transitive imports within the module on every
// platform, without the module prefix.
func graph(feature boundary) ([]string, error) {
	return listed(feature, "{{.ImportPath}}", true)
}

// listed is what go list prints for the package, or with deps for its whole
// graph, on every platform.
func listed(feature boundary, format string, deps bool) ([]string, error) {
	var all []string
	for _, goos := range platforms {
		args := []string{"list", "-f", format, module + feature.path}
		if deps {
			args = []string{"list", "-deps", "-f", format, module + feature.path}
		}
		command := exec.Command("go", args...)
		command.Env = append(os.Environ(), "GOOS="+goos)
		output, err := command.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("load %s dependency graph on %s: %w\n%s", feature.name, goos, err, output)
		}
		all = append(all, strings.Fields(string(output))...)
	}
	slices.Sort(all)
	return slices.Compact(all), nil
}

func checkBoundaryDependencies(feature boundary, dependencies []string) error {
	allowed := make(map[string]bool, len(feature.allowed))
	for _, path := range feature.allowed {
		allowed[module+path] = true
	}
	for _, dependency := range dependencies {
		if dependency == module {
			return fmt.Errorf("%s depends on application root %s", feature.name, dependency)
		}
		if !strings.HasPrefix(dependency, module+"/") {
			continue
		}
		if !feature.runtime && forbidden(dependency) {
			return fmt.Errorf("%s depends on runtime package %s; feature packages return values to the root", feature.name, dependency)
		}
		if !allowed[dependency] {
			return fmt.Errorf("%s depends on forbidden package %s", feature.name, dependency)
		}
	}
	return nil
}

// checkRefusedImports fails on an import the package makes itself that its
// boundary refuses.
func checkRefusedImports(feature boundary, imports []string) error {
	for _, path := range feature.refused {
		if strings.HasPrefix(path, "/") {
			path = module + path
		}
		if slices.Contains(imports, path) {
			return fmt.Errorf("%s imports %s, which its boundary refuses", feature.name, path)
		}
	}
	return nil
}

func forbidden(dependency string) bool {
	for _, path := range runtimePackages {
		if dependency == module+path {
			return true
		}
	}
	return false
}

// writeBoundaries regenerates every allowed list from the graphs, so a new
// import shows up as a reviewed diff rather than a hand edit.
func writeBoundaries(file string) error {
	var b strings.Builder
	b.WriteString(boundariesHeader)
	b.WriteString("var (\n")
	for i, feature := range boundaries {
		dependencies, err := graph(feature)
		if err != nil {
			return err
		}
		var allowed []string
		for _, dependency := range dependencies {
			if !strings.HasPrefix(dependency, module+"/") {
				continue
			}
			if !feature.runtime && forbidden(dependency) {
				return fmt.Errorf("%s imports runtime package %s; return a value to the root, or mark the package runtime", feature.name, dependency)
			}
			allowed = append(allowed, fmt.Sprintf("%q", strings.TrimPrefix(dependency, module)))
		}
		refused := "nil"
		if len(feature.refused) > 0 {
			quoted := make([]string, len(feature.refused))
			for j, path := range feature.refused {
				quoted[j] = fmt.Sprintf("%q", path)
			}
			refused = "[]string{" + strings.Join(quoted, ", ") + "}"
		}
		fmt.Fprintf(&b, "\t%s = boundary{%q, %q, %t, []string{%s}, %s}\n", boundaryNames[i], feature.name, feature.path, feature.runtime, strings.Join(allowed, ", "), refused)
	}
	quoted := make([]string, len(boundaryNames))
	for i, name := range boundaryNames {
		quoted[i] = fmt.Sprintf("%q", name)
	}
	b.WriteString(")\n\nvar boundaries = []boundary{" + strings.Join(boundaryNames, ", ") + "}\n\nvar boundaryNames = []string{" + strings.Join(quoted, ", ") + "}\n")
	source, err := format.Source([]byte(b.String()))
	if err != nil {
		return err
	}
	return os.WriteFile(file, source, 0o644)
}

const boundariesHeader = `package main

// Each line is a package's whole transitive graph within the module, on every
// platform the release builds. A runtime package may reach the store, tmux,
// git and SSH; a feature package may not. Regenerate the lists with
// go run ./tools/architecture/check-ui-boundaries -write
`
