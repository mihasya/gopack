package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)

var template = `[deps.%s]
import = "%s"
%s = "%s"
provider = "%s"
source = "%s"

`

var ScmRe = regexp.MustCompile(`\w\.\w+/`)

func StripScmFromImport(scmPath string) string {
	parts := ScmRe.Split(scmPath, -1)
	return strings.Replace(parts[len(parts)-1], ".", "_", -1)
}

func findDepInGopath(gopath, dep string) (string, error) {
	gopaths := strings.Split(gopath, ":")
	for _, gopathpart := range gopaths {
		deppath := path.Join(gopathpart, "src", dep)
		if _, err := os.Stat(deppath); err == nil {
			return deppath, nil
		}
		depparts := strings.Split(dep, "/")
		dep = path.Join(depparts[:len(depparts)-1]...)
	}
	return "", fmt.Errorf("Could not find dependency %s in GOPATH %s", dep, gopath)
}

func GenerateConfig(p *ProjectStats) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Could not get working dir: %s\n", err)
	}
	gopath := os.Getenv("GOPATH")
	buf := bytes.NewBuffer(make([]byte, 0))
	uniqueImports := make(map[string]*Dep)
	uniqueDeps := make(map[string]bool)
	depsToAnalyze := make([]string, 0)
	for dep, stats := range p.ImportStatsByPath {
		if stats.Remote {
			depsToAnalyze = append(depsToAnalyze, dep)
		}
	}

	// recursively run dependency analysis to pull in transitive deps
	for len(depsToAnalyze) > 0 {
		dep := depsToAnalyze[0]
		depsToAnalyze = depsToAnalyze[1:]
		if _, alreadyAnalyzed := uniqueDeps[dep]; !alreadyAnalyzed {
			depPath, err := findDepInGopath(gopath, dep)
			if err != nil {
				return "", err
			}
			depStats, err := AnalyzeSourceTree(depPath)
			for subDep, subStats := range depStats.ImportStatsByPath {
				if subStats.Remote {
					depsToAnalyze = append(depsToAnalyze, subDep)
				}
			}
			uniqueDeps[dep] = true
		}
	}
	var repo = ""
	for dep, _ := range uniqueDeps {
		depPath, err := findDepInGopath(gopath, dep)
		if err != nil {
			return "", err
		}
		d := DepFromPath(depPath)
		if strings.HasSuffix(cwd, d.Import) {
			repo = d.Import
		} else {
			uniqueImports[d.Source] = d

		}
	}
	if repo != "" {
		buf.WriteString(fmt.Sprintf("repo = \"%s\"\n\n", repo))
	}
	for _, d := range uniqueImports {
		buf.WriteString(fmt.Sprintf(template, StripScmFromImport(d.Import), d.Import, d.CheckoutType(), d.CheckoutSpec, d.Provider, d.Source))
	}
	return buf.String(), nil
}
