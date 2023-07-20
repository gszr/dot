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

type FileMapping struct {
	From string
	To string
	As string
	Os  string
}

type Opts struct {
	Cd string
}

type Dots struct {
	Opts Opts `yaml:"opts"`
	FileMappings []FileMapping `yaml:"map"`
}

func (d *Dots) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var tmpDots struct {
		Opts Opts `yaml:"files_opts"`
		Mappings map[string]FileMapping `yaml:"map"`
	}
	err := unmarshal(&tmpDots)
	if err != nil {
		return err
	}
	d.Opts.Cd = tmpDots.Opts.Cd
	for file, mapping := range tmpDots.Mappings {
		mapping.From = file
		mapping.To = expandTilde(mapping.To)
		if len(mapping.To) == 0 {
			mapping.To = inferDestination(mapping.From)
		}
		if len(d.Opts.Cd) > 0 {
			mapping.From = path.Join(d.Opts.Cd, file)
		}
		if len(mapping.As) == 0 {
			mapping.As = "link"
		}
		d.FileMappings = append(d.FileMappings, mapping)
	}

	return nil
}

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

func inferDestination(file string) string {
	if strings.HasPrefix(file, ".") {
		return getHomeDir() + "/" + file
	} else {
		return getHomeDir() + "/." + file
	}
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

func validateDots(dots *Dots) (bool, []error) {
	var errs []error
	for _, mapping := range dots.FileMappings {
		if ! pathExists(mapping.From) {
			errs = append(errs, fmt.Errorf("%s: path does not exist", mapping.From))
		}  else if isDirectory(mapping.From) && mapping.As == "copy" {
			errs = append(errs, fmt.Errorf("%s: cannot use copy type with directory", mapping.From))
		}
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

func readDotFile() Dots {
	rcFileData, err := ioutil.ReadFile(flagDotFile)
	if err != nil {
		logger.Fatalf("error reading config data: %v", err)
	}
	var dots Dots
	if err := yaml.Unmarshal([]byte(rcFileData), &dots); err != nil {
		logger.Fatalf("cannot decode data: %v", err)
	}
	valid, errs := validateDots(&dots)
	if ! valid {
		for _, err := range errs {
			logger.Printf("failed validating dots file: %v", err)
		}
		os.Exit(1)
	}
	return dots
}

func doLink(file string, dst string) {
	if !strings.HasPrefix(file, "/") {
		cwd, err := os.Getwd()
		if err != nil {
			logger.Fatalf("failed getting cwd: %v", err)
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
	if flagVerbose {
		logger.Printf("linking  %s -> %s\n", file, dst)
	}
}

func doCopy(file string, dst string) (bool, error) {
	fin, err := os.Open(file)
	if err != nil {
		log.Fatalf("failed opening file %s, %v", file, err)
	}
	defer fin.Close()

	dstDir := path.Dir(dst)
	if ! pathExists(dstDir) {
		err := os.MkdirAll(dstDir, 0750)
		if err != nil {
			logger.Fatalf("failed creating path %s, %v", dst, err)
		}
	}

	fout, err := os.Create(dst)
	if err != nil {
		log.Fatalf("failed creating file %s, %v", file, err)
	}
	defer fout.Close()
	_, err = io.Copy(fout, fin)
	if err != nil {
		log.Fatalf("failed copying file %s to %s, %v", file, dst, err)
	}
	if flagVerbose {
		logger.Printf("copying %s -> %s\n", file, dst)
	}

	return true, nil
}

func remove(mapping FileMapping) error {
	if dstExists := pathExists(mapping.To); ! dstExists && flagVerbose {
		logger.Printf("rm %s: skipping, file not there\n", mapping.To)
	} else {
		err := os.Remove(mapping.To)
		if err != nil {
			logger.Printf("failed removing file %s, %v\n", mapping.To, err)
		}
		if flagVerbose {
			logger.Printf("rm %s: success\n", mapping.To)
		}
	}
	return nil
}

func doDots(mapping FileMapping) error {
	switch typ := mapping.As; typ {
	case "link":
		doLink(mapping.From, mapping.To)
	case "copy":
		doCopy(mapping.From, mapping.To)
	}
	return nil
}

func iterate(dots Dots) {
	osMap := map[string]string {
		"linux": "linux",
		"macos": "darwin",
		"darwin": "darwin",
		"all": runtime.GOOS,
		"": runtime.GOOS,
	}
	for _, mapping := range dots.FileMappings {
		if osMap[mapping.Os] != runtime.GOOS {
			if flagVerbose {
				logger.Printf("skipping %s: not on %s\n", mapping.From, mapping.Os)
			}
			continue
		}
		if flagRm {
			remove(mapping)
			if flagRmOnly {
				continue
			}
		}
		doDots(mapping)
	}
}

func main() {
	flag.Parse()
	iterate(readDotFile())
}
