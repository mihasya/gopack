package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

const (
	GopackDir          = ".gopack"
	GopackChecksum     = ".gopack/checksum"
	GopackTestProjects = ".gopack/test-projects"
	VendorDir          = ".gopack/vendor"
)

const (
	Blue     = uint8(94)
	Green    = uint8(92)
	Red      = uint8(31)
	Gray     = uint8(90)
	EndColor = "\033[0m"
)

var (
	pwd        string
	showColors = true
)

func main() {
	if os.Getenv("GOPACK_SKIP_COLORS") == "1" {
		showColors = false
	}

	fmtcolor(104, "/// g o p a c k ///")
	fmt.Println()
	// localize GOPATH
	setupEnv()
	loadDependencies(".")
}

func loadDependencies(root string) {
	p, err := AnalyzeSourceTree(root)
	if err != nil {
		fail(err)
	}

	config, dependencies := loadConfiguration(root, NewGraph())
	if dependencies != nil {
		failWith(dependencies.Validate(p))
		// prepare dependencies
		loadTransitiveDependencies(dependencies)
		config.WriteChecksum()
	}
	// run the specified command
	runCommand()
}

func loadConfiguration(dir string, importGraph *Graph) (*Config, *Dependencies) {
	config := NewConfig(dir)
	config.InitRepo(importGraph)

	var dependencies *Dependencies
	if config.FetchDependencies() {
		dependencies = LoadDependencyModel(config.DepsTree, importGraph)
	}

	return config, dependencies
}

func runCommand() {
	cmd := exec.Command("go", os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fail(err)
	}
}

func loadTransitiveDependencies(dependencies *Dependencies) {
	dependencies.VisitDeps(
		func(dep *Dep) {
			fmtcolor(Gray, "updating %s\n", dep.Import)
			dep.goGetUpdate()
			if dep.CheckoutType() != "" {
				fmtcolor(Gray, "pointing %s at %s %s\n", dep.Import, dep.CheckoutType(), dep.CheckoutSpec)
				dep.switchToBranchOrTag()
			}
			transitive := dep.LoadTransitiveDeps(dependencies.ImportGraph)
			if transitive != nil {
				loadTransitiveDependencies(transitive)
			}
		})
}

// Set the working directory.
// It's the current directory by default.
// It can be overriden setting the environment variable GOPACK_APP_CONFIG.
func setPwd() {
	var dir string
	var err error

	dir = os.Getenv("GOPACK_APP_CONFIG")
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			fail(err)
		}
	}

	pwd = dir
}

// set GOPATH to the local vendor dir
func setupEnv() {
	setPwd()
	vendor := fmt.Sprintf("%s/%s", pwd, VendorDir)
	err := os.Setenv("GOPATH", vendor)
	if err != nil {
		fail(err)
	}
}

func fmtcolor(c uint8, s string, args ...interface{}) {
	if showColors {
		fmt.Printf("\033[%dm", c)
	}

	if len(args) > 0 {
		fmt.Printf(s, args...)
	} else {
		fmt.Printf(s)
	}

	if showColors {
		fmt.Printf(EndColor)
	}
}

func logcolor(c uint8, s string, args ...interface{}) {
	log.Printf("\033[%dm", c)
	if len(args) > 0 {
		log.Printf(s, args...)
	} else {
		log.Printf(s)
	}
	log.Printf(EndColor)
}

func failf(s string, args ...interface{}) {
	fmtcolor(Red, s, args...)
	os.Exit(1)
}

func fail(a ...interface{}) {
	fmt.Printf("\033[%dm", Red)
	fmt.Print(a)
	fmt.Printf(EndColor)
	os.Exit(1)
}

func failWith(errors []*ProjectError) {
	if len(errors) > 0 {
		fmt.Printf("\033[%dm", Red)
		for _, e := range errors {
			fmt.Printf(e.String())
		}
		fmt.Printf(EndColor)
		fmt.Println()
		os.Exit(len(errors))
	}
}
