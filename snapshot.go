package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
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
	return parts[len(parts)-1]
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

func GenerateConfig() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Could not get working dir: %s\n", err)
	}
	gopath := os.Getenv("GOPATH")
	buf := bytes.NewBuffer(make([]byte, 0))
	uniqueImports := make(map[string]*Dep)
	depsListCmd := exec.Command("go", "list", "-f", `'{{join .Deps "\n"}}'`, "./...")
	depsListCmd.Dir = cwd
	depsBytes, err := depsListCmd.CombinedOutput()
	if err != nil {
		failf("Could not get list of dependencies: %s\n%s\n", err, string(depsBytes))
	}
	scanner := bufio.NewScanner(bytes.NewBuffer(depsBytes))
	uniqueDeps := make(map[string]bool)
	for scanner.Scan() {
		dep := scanner.Text()
		if strings.Contains(dep, ".") {
			uniqueDeps[dep] = true
		}
	}
	for dep, _ := range uniqueDeps {
		depPath, err := findDepInGopath(gopath, dep)
		if err != nil {
			return "", err
		}
		d := DepFromPath(depPath)
		uniqueImports[d.Source] = d
	}

	for _, d := range uniqueImports {
		if strings.HasSuffix(cwd, d.Import) {
			// TODO: would be nice if the repo clause was always at the top of the config
			// I guess map order is also not reliable, so we should do something else here
			buf.WriteString(fmt.Sprintf("repo = \"%s\"\n\n", d.Import))
		} else {
			buf.WriteString(fmt.Sprintf(template, StripScmFromImport(d.Import), d.Import, d.CheckoutType(), d.CheckoutSpec, d.Provider, d.Source))
		}
	}
	return buf.String(), nil
}
