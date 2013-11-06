package main

// LOL so we're gonna try and avoid THIS situation http://golang.org/src/cmd/go/vcs.go#L331

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
)

var detectors []func(path string) (Scm, string)

func findFolderInPath(searchPath, folder string) string {
	// stop when we hit GOPATH, basically
	for !strings.HasSuffix(path.Clean(searchPath), "src") {
		if _, err := os.Stat(path.Join(searchPath, folder)); err == nil {
			return searchPath
		}
		searchPath, _ = path.Split(path.Clean(searchPath))
	}
	return ""
}

func init() {
	gitDetector := func(path string) (Scm, string) {
		gitPath := findFolderInPath(path, ".git")
		if gitPath != "" {
			return Git{}, gitPath
		} else {
			return nil, ""
		}
	}
	hgDetector := func(path string) (Scm, string) {
		hgPath := findFolderInPath(path, ".hg")
		if hgPath != "" {
			return Hg{}, hgPath
		} else {
			return nil, ""
		}
	}

	detectors = append(detectors, gitDetector, hgDetector)
}

func DetectScm(searchPath string) (Scm, string) {
	for _, detector := range detectors {
		if scm, path := detector(searchPath); scm != nil {
			return scm, path
		}
	}
	return nil, ""
}

type Scm interface {
	Init(d *Dep) error
	Checkout(d *Dep) error
	PopulateDep(scmPath string, d *Dep) error
}

type Git struct{}

func (g Git) Init(d *Dep) error {
	path := fmt.Sprintf("%s/%s/src/%s", pwd, VendorDir, d.Import)
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("Error creating import dir %s", err)
	} else {
		if _, err := os.Stat(fmt.Sprintf("%s/%s", path, ".git")); os.IsNotExist(err) {
			fmtcolor(Gray, "cloning %s to %s\n", d.Source, path)
			cmd := exec.Command("git", "clone", d.Source, path)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("Error cloning repo %s", err)
			}
		} else if err == nil {
			log.Printf("Git dir exists for %s, skipping clone. To reset the source, run `rm -R %s`, then run gopack again", d.Import, path)
		} else {
			return fmt.Errorf("Error while examining git dir for %s: %s", d.Import, err)
		}
	}
	return nil
}

func (g Git) Checkout(d *Dep) error {
	cmd := exec.Command("git", "checkout", d.CheckoutSpec)
	return cmd.Run()
}

func (g Git) PopulateDep(scmPath string, d *Dep) error {
	d.Provider = "git"
	d.SourceDir = scmPath
	d.Import = path.Clean(strings.Split(scmPath, "/src/")[1])
	headBytes, err := ioutil.ReadFile(path.Join(scmPath, ".git", "HEAD"))
	if err != nil {
		return fmt.Errorf("Unable to read HEAD: %s", err)
	}
	head := strings.TrimSpace(string(headBytes))
	headParts := strings.Split(head, "/")
	if len(headParts) == 1 {
		// a commit
		d.CheckoutFlag = CommitFlag
		d.CheckoutSpec = string(head)
	} else if headParts[len(headParts)-2] == "heads" {
		d.CheckoutFlag = BranchFlag
		d.CheckoutSpec = headParts[len(headParts)-1]
	} else if headParts[len(headParts)-2] == "tags" {
		d.CheckoutFlag = TagFlag
		d.CheckoutSpec = headParts[len(headParts)-1]
	}

	remotesCmd := exec.Command("git", "config", "--list")
	remotesCmd.Dir = scmPath
	outputBytes, err := remotesCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Could not run git config --list: %s", err)
	}

	scanner := bufio.NewScanner(bytes.NewBuffer(outputBytes))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "remote.origin.url") {
			remote := strings.TrimPrefix(line, "remote.origin.url=")
			d.Source = remote
		}
	}
	if d.Source == "" {
		return fmt.Errorf("Unable to determine source for %s", scmPath)
	}
	return nil
}

func (g Git) String() string {
	return "Git"
}

type Hg struct{}

// TODO someone should vet this that knows hg
func (h Hg) Init(d *Dep) error {
	scmPath := path.Join(pwd, VendorDir, d.Import)
	if err := os.MkdirAll(scmPath, 0755); err != nil {
		return err
	} else {
		if _, err := os.Stat(fmt.Sprintf("%s/%s", scmPath, ".hg")); os.IsNotExist(err) {
			cmd := exec.Command("hg", "clone", d.Source, scmPath)
			if err := cmd.Run(); err != nil {
				return err
			}
		} else if err == nil {

			log.Printf("Hg dir exists for %s, skipping clone. To reset the source, run `rm -R %s`, then run gopack again", d.Import, scmPath)
		} else {
			return fmt.Errorf("Error while examining hg dir for %s: %s", d.Import, err)
		}
	}
	return nil
}

func (h Hg) Checkout(d *Dep) error {
	var cmd *exec.Cmd

	if d.CheckoutFlag == CommitFlag {
		cmd = exec.Command("hg", "update", "-c", d.CheckoutSpec)
	} else {
		cmd = exec.Command("hg", "checkout", d.CheckoutSpec)
	}

	return cmd.Run()
}

func (h Hg) PopulateDep(scmPath string, d *Dep) error {
	return fmt.Errorf("Snapshotting hg deps not yet supported\n")
}

func (h Hg) String() string {
	return "Hg"
}

type Svn struct {
}

// FIXME someone that has an SVN repo accessible, please
func (s Svn) Init(d *Dep) error {
	return fmt.Errorf("Explicitly initializing SVN deps not yet supported\n")
}

func (s Svn) Checkout(d *Dep) error {
	var cmd *exec.Cmd

	switch d.CheckoutFlag {
	case CommitFlag:
		cmd = exec.Command("svn", "up", "-r", d.CheckoutSpec)
	case BranchFlag:
		cmd = exec.Command("svn", "switch", "^/branches/"+d.CheckoutSpec)
	case TagFlag:
		cmd = exec.Command("svn", "switch", "^/tags/"+d.CheckoutSpec)
	}

	return cmd.Run()
}

func (s Svn) PopulateDep(scmPath string, d *Dep) error {
	return fmt.Errorf("Snapshotting SVN deps not yet supported")
}

func (s Svn) String() string {
	return "Svn"
}

// The Go provider embeds another provider and only implements Init so that
// deps that don't specify a provider keep working like they did before
type Go struct {
	Scm
}

func (g Go) Init(d *Dep) error {
	cmd := exec.Command("go", "get", "-d", "-u", d.Import)
	return cmd.Run()
}

func (g Go) String() string {
	return "Go"
}
