package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

/*
 * initialization and flags
 */

var logger *log.Logger

var flagDotFile string
var flagVerbose bool
var flagRmOnly bool
var flagRm bool

func initFlags() {
	flag.StringVar(&flagDotFile, "dot", "dot.yml", "the dots config file")
	flag.BoolVar(&flagVerbose, "verbose", false, "verbose output")
	flag.BoolVar(&flagRm, "rm", true, "remove targets before creating")
	flag.BoolVar(&flagRmOnly, "rm-only", false, "only remove targets, do not create")
}

func init() {
	initFlags()
	logger = log.New(os.Stderr, "", 0)
}

/*
 * core data structures and operations
 */

type FileMapping struct {
	From string
	To   string
	As   string
	Os   string
}

// TODO return error - caller handles error
func (m FileMapping) doLink() {
	err := os.Symlink(m.From, m.To)
	if err != nil {
		logger.Fatalf("failed linking %s -> %s: %v", m.From, m.To, err)
	}
	if flagVerbose {
		logger.Printf("linking  %s -> %s\n", m.From, m.To)
	}
}

// TODO return error - caller handles error
func (m FileMapping) doCopy() (bool, error) {
	fin, err := os.Open(m.From)
	if err != nil {
		log.Fatalf("failed opening file %s, %v", m.From, err)
	}
	defer fin.Close()

	fout, err := os.Create(m.To)
	if err != nil {
		log.Fatalf("failed creating file %s, %v", m.From, err)
	}
	defer fout.Close()

	_, err = io.Copy(fout, fin)
	if err != nil {
		log.Fatalf("failed copying file %s to %s, %v", m.From, m.To, err)
	}
	if flagVerbose {
		logger.Printf("copying %s -> %s\n", m.From, m.To)
	}

	return true, nil
}

// TODO return error - caller handles
func (m FileMapping) unmap() error {
	if dstExists := pathExists(m.To); !dstExists && flagVerbose {
		logger.Printf("rm %s: skipping, file not there\n", m.To)
	} else {
		err := os.Remove(m.To)
		if err != nil {
			logger.Printf("failed removing file %s, %v\n", m.To, err)
		}
		if flagVerbose {
			logger.Printf("rm %s: success\n", m.To)
		}
	}
	return nil
}

// TODO return error - caller handles
func (m FileMapping) domap() error {
	// ensure destination path exists
	if err := createPath(m.To); err != nil {
		logger.Fatalf("failed creating path %s, %v", m.To, err)
	}

	switch typ := m.As; typ {
	case "link":
		m.doLink()
	case "copy":
		m.doCopy()
	}
	return nil
}

func (m FileMapping) isMatchingOs() bool {
	osMap := map[string]string{
		"linux":  "linux",
		"macos":  "darwin",
		"darwin": "darwin",
		"all":    runtime.GOOS,
		"":       runtime.GOOS,
	}
	if osMap[m.Os] != runtime.GOOS {
		return false
	}
	return true
}

type Opts struct {
	Cd string
}

type Dots struct {
	Opts         Opts          `yaml:"opt"`
	FileMappings []FileMapping `yaml:"map"`
}

func (d *Dots) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var tmpDots struct {
		Opts     Opts                   `yaml:"opt"`
		Mappings map[string]FileMapping `yaml:"map"`
	}
	err := unmarshal(&tmpDots)
	if err != nil {
		return err
	}
	d.Opts = tmpDots.Opts
	for file, mapping := range tmpDots.Mappings {
		mapping.From = file
		d.FileMappings = append(d.FileMappings, mapping)
	}
	return nil
}

func (dots Dots) validate() (bool, []error) {
	var errs []error
	for _, mapping := range dots.FileMappings {
		if !pathExists(mapping.From) {
			errs = append(errs, fmt.Errorf("%s: path does not exist", mapping.From))
		} else if isDirectory(mapping.From) && mapping.As == "copy" {
			errs = append(errs, fmt.Errorf("%s: cannot use copy type with directory", mapping.From))
		}
	}
	return len(errs) == 0, errs
}

func inferDestination(file string) string {
	if strings.HasPrefix(file, ".") {
		return getHomeDir() + "/" + file
	} else {
		return getHomeDir() + "/." + file
	}
}

func (dots Dots) transform() Dots {
	opts := dots.Opts
	mappings := dots.FileMappings

	var newDots Dots
	newDots.Opts = opts

	for _, mapping := range mappings {
		// To is expanded / inferred first: it's value is based off of
		// `from` before prefix or cwd are added to it
		if len(mapping.To) > 0 {
			// expand destination ~
			mapping.To = expandTilde(mapping.To)
		} else {
			// infer destination based on From
			mapping.To = inferDestination(mapping.From)
		}

		if len(opts.Cd) > 0 {
			// Cd set: add prefix to From
			mapping.From = path.Join(opts.Cd, mapping.From)
		}
		if !strings.HasPrefix(mapping.From, "/") {
			cwd, _ := os.Getwd()
			mapping.From = cwd + "/" + mapping.From
		}

		// default As to symlink
		if len(mapping.As) == 0 {
			mapping.As = "link"
		}

		newDots.FileMappings = append(newDots.FileMappings, mapping)
	}

	return newDots
}

func (dots Dots) iterate() {
	for _, mapping := range dots.FileMappings {
		if !mapping.isMatchingOs() {
			if flagVerbose {
				logger.Printf("not on %s, skipping %s\n", mapping.Os, mapping.From)
			}
			continue
		}
		if flagRm { // remove before mapping by default
			mapping.unmap()
			if flagRmOnly {
				continue
			}
		}
		mapping.domap()
	}
}

/*
 * Helpers
 */

func createPath(p string) error {
	dstDir := path.Dir(p)
	if !pathExists(dstDir) {
		err := os.MkdirAll(dstDir, 0750)
		if err != nil {
			return err
		}
	}
	return nil
}

func pathExists(path string) bool {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func isDirectory(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		logger.Fatalf("failed reading path %s, %v", path, err)
	}
	return fileInfo.IsDir()
}

func getHomeDir() string {
	return os.Getenv("HOME")
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~") {
		homeDir := getHomeDir()
		path = strings.Replace(path, "~", homeDir, 1)
	}
	return path
}

func readDotFile(file string) Dots {
	rcFileData, err := ioutil.ReadFile(file)
	if err != nil {
		logger.Fatalf("error reading config data: %v", err)
	}

	var dots Dots
	if err := yaml.Unmarshal([]byte(rcFileData), &dots); err != nil {
		logger.Fatalf("cannot decode data: %v", err)
	}

	newDots := dots.transform()
	valid, errs := newDots.validate()
	if !valid {
		for _, err := range errs {
			logger.Printf("failed validating dots file: %v", err)
		}
		os.Exit(1)
	}

	return newDots
}

func main() {
	flag.Parse()
	dots := readDotFile(flagDotFile)
	dots.iterate()
}
