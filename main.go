package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"strings"
	"fmt"
	"path"
	"runtime"

	"gopkg.in/yaml.v3"
)

type Attributes struct {
	Dst string `yaml:"dst"`
	Typ string `yaml:"typ"`
	Os  string `yaml:"os"`
}

type Dots struct {
	Files map[string]Attributes `yaml:"files"`
}

var logger *log.Logger

var rcFile string
var verbose bool
var rmOnly bool

func initFlags() {
	flag.StringVar(&rcFile, "rc", "dots.yml", "the dots config file")
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	flag.BoolVar(&rmOnly, "rm", false, "only remove targets, do not create")
}

func init() {
	initFlags()
	logger = log.New(os.Stderr, "", 0)
}

func inferDestination(file string) string {
	return getHomeDir() + "/." + file
}

func pathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
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

func validateDots(dots *Dots) (bool, []error) {
	var errs []error
	for file, attr := range dots.Files {
		if len(attr.Dst) == 0 {
			attr.Dst = inferDestination(file)
		}
		if len(attr.Typ) == 0 {
			attr.Typ = "link"
			dots.Files[file] = attr
		}
		if ! pathExists(file) {
			errs = append(errs, fmt.Errorf("%s: path does not exist", file))
		}
		if isDirectory(file) && attr.Typ == "copy" {
			errs = append(errs, fmt.Errorf("%s: cannot use copy type with directory", file))
		}
		dots.Files[file] = attr
	}
	return len(errs) == 0, errs
}

func getHomeDir() string {
	usr, _ := user.Current()
	return usr.HomeDir
}
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~") {
		homeDir := getHomeDir()
		path = strings.Replace(path, "~", homeDir, 1)
	}
	return path
}

func readDots() Dots {
	rcFileData, err := ioutil.ReadFile(rcFile)
	if err != nil {
		log.Fatalf("error reading config data: %v", err)
	}
	var dots Dots
	if err := yaml.Unmarshal([]byte(rcFileData), &dots); err != nil {
		log.Fatalf("cannot decode data: %v", err)
	}
	valid, errs := validateDots(&dots)
	if ! valid {
		for _, err := range errs {
			logger.Printf("failed validating dots file: %v", err)
		}
		os.Exit(0)
	}
	return dots
}

func doLink(file string, dst string) {
	if !strings.HasPrefix(file, "/") {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("failed getting cwd: %v", err)
		}
		file = cwd + "/" + file
	}
	dstDir := path.Dir(dst)
	if ! pathExists(dstDir) {
		err := os.MkdirAll(dstDir, 0750)
		if err != nil {
			logger.Fatalf("failed creating path %s, %v", dst, err)
		}
	}
	err := os.Symlink(file, dst)
	if err != nil {
		logger.Fatalf("failed linking %s -> %s: %v", file, dst, err)
	}
	if verbose {
		logger.Printf("linking  %s -> %s\n", file, dst)
	}
}

func doCopy(file string, dst string) (bool, error) {
	fin, err := os.Open(file)
	if err != nil {
		log.Fatalf("failed opening file %s, %v", file, err)
	}
	defer fin.Close()

	fout, err := os.Create(dst)
	if err != nil {
		log.Fatalf("failed creating file %s, %v", file, err)
	}
	defer fout.Close()

	_, err = io.Copy(fout, fin)
	if err != nil {
		log.Fatalf("failed copying file %s to %s, %v", file, dst, err)
	}
	if verbose {
		logger.Printf("copying %s -> %s\n", file, dst)
	}

	return true, nil
}

func doDot(file string, attr Attributes) error {
	switch typ := attr.Typ; typ {
	case "link":
		doLink(file, attr.Dst)
	case "copy":
		doCopy(file, attr.Dst)
	}
	return nil
}

func remove(file string, attr Attributes) error {
	if exists, err := pathExists(file), os.Remove(attr.Dst); !exists && err != nil {
		logger.Printf("failed removing file %s, %v", attr.Dst, err)
		return err
	}
	if verbose {
		switch typ:= attr.Typ; typ {
		case "link":
			logger.Printf("removing link %s (from %s)", attr.Dst, file)
		case "copy":
			logger.Printf("removing link %s (from %s)", attr.Dst, file)
		}
	}
	return nil
}

func main() {
	flag.Parse()
	dots := readDots()
	osMap := map[string]string {
		"linux": "linux",
		"macos": "darwin",
		"darwin": "darwin",
		"all": runtime.GOOS,
		"": runtime.GOOS,
	}
	for file, attr := range dots.Files {
		if rmOnly {
			remove(file, attr)
		} else if osMap[attr.Os] == runtime.GOOS {
			doDot(file, attr)
		}
	}
}
