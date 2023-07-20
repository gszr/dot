package main

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"testing"
)

func isSymlink(path string) bool {
	fInfo, _ := os.Lstat(path)
	return fInfo.Mode()&os.ModeSymlink != 0
}

/*
 * core data structures and operations
 */

func TestDoLink(t *testing.T) {
	to := "out/"
	_ = createPath(to)
	defer func() {
		_ = os.RemoveAll(to)
	}()

	// creates a symlink
	m := FileMapping{
		From: "examples/bashrc",
		To:   "out/bashrc",
	}

	m.doLink()
	if !isSymlink(m.To) {
		t.Errorf("%s not symlink", m.To)
	}
}

func TestDoCopy(t *testing.T) {
	to := "out/"
	_ = createPath(to)
	defer func() {
		_ = os.RemoveAll(to)
	}()

	/* reports nonexistent file
	m := FileMapping {
		From: "examples/foobar",
		To: "out/bashrc",
	}

	m.doCopy()
	if isSymlink(m.To) {
		t.Errorf("%s not copy", m.To)
	}
	*/

	// copies a file
	m := FileMapping{
		From: "examples/zshrc",
		To:   "out/bashrc",
	}

	m.doCopy()
	if isSymlink(m.To) {
		t.Errorf("%s not copy", m.To)
	}
}

func TestUnmap(t *testing.T) {
	// removes a symlink
	to := "out/"
	_ = createPath(to)
	defer func() {
		_ = os.RemoveAll(to)
	}()

	// creates a symlink
	m := FileMapping{
		From: "examples/bashrc",
		To:   "out/bashrc",
	}
	m.doLink()

	err := m.unmap()
	if err != nil {
		t.Errorf("failed unmapping %s", m.To)
	}

	if pathExists(m.To) {
		t.Errorf("did not unmap %s", m.To)
	}
}

func TestDoMap(t *testing.T) {
	defer func() {
		_ = os.RemoveAll("out")
	}()

	// creates a symlink
	m := FileMapping{
		From: "examples/bashrc",
		To:   "out/bashrc",
		As:   "link",
	}

	err := m.domap()
	if err != nil {
		t.Errorf("failed mapping %s", m.To)
	}

	if !isSymlink(m.To) {
		t.Errorf("failed creating %s as symlink", m.To)
	}

	// creates path
	if !pathExists("examples") {
		t.Errorf("should have created path to %s", m.To)
	}

	// creates a copy
	m = FileMapping{
		From: "examples/zshrc",
		To:   "out/foobar",
		As:   "copy",
	}

	err = m.domap()
	if err != nil {
		t.Errorf("failed mapping %s", m.To)
	}

	if isSymlink(m.To) {
		t.Errorf("failed creating %s as copy", m.To)
	}

	// same contents

	var fromContents []byte
	var toContents []byte
	fromContents, err = ioutil.ReadFile(m.From)
	if err != nil {
		t.Errorf("failed reading contents of %s", m.From)
	}
	toContents, err = ioutil.ReadFile(m.To)
	if err != nil {
		t.Errorf("failed reading contents of %s", m.To)
	}
	if !bytes.Equal(fromContents, toContents) {
		t.Errorf("%s and %s contents differ", m.From, m.To)
	}
}

func TestUnharshalYAML(t *testing.T) {
	dotData := `
map:
  f1:
  f2:
  f3:
opt:
  cd: foo
`

	var dots Dots
	if err := yaml.Unmarshal([]byte(dotData), &dots); err != nil {
		t.Errorf("cannot decode data: %v", err)
	}

	if dots.Opts.Cd != "foo" {
		t.Errorf("failed unmarshaling field")
	}

	if len(dots.FileMappings) != 3 {
		t.Errorf("failed unmarshaling field")
	}
}

func TestValidate(t *testing.T) {
	// valid dots
	d := Dots{
		Opts: Opts{
			Cd: "foo",
		},
		FileMappings: []FileMapping{
			FileMapping{
				From: "examples/zshrc",
			},
		},
	}
	ok, _ := d.validate()
	if !ok {
		t.Errorf("failed validating")
	}

	// invalid dots: path does not exist
	d = Dots{
		Opts: Opts{
			Cd: "foo",
		},
		FileMappings: []FileMapping{
			FileMapping{
				From: "examples/foo",
			},
		},
	}
	var errs []error
	ok, errs = d.validate()
	if len(errs) != 1 {
		t.Errorf("more errors than expected")
	}
	if errs[0].Error() != fmt.Sprintf("%s: path does not exist", d.FileMappings[0].From) {
		t.Errorf("unexpected error %s", errs[0].Error())
	}

	// invalid dots: copy on directory
	d = Dots{
		Opts: Opts{
			Cd: "foo",
		},
		FileMappings: []FileMapping{
			FileMapping{
				From: "examples/",
				As:   "copy",
			},
		},
	}
	ok, errs = d.validate()
	if len(errs) != 1 {
		t.Errorf("more errors than expected")
	}
	if errs[0].Error() != fmt.Sprintf("%s: cannot use copy type with directory",
		d.FileMappings[0].From) {
		t.Errorf("unexpected error %s", errs[0].Error())
	}
}

func TestTransform(t *testing.T) {
	// prefix and tilde expansion
	d := Dots{
		FileMappings: []FileMapping{
			FileMapping{
				From: "examples/zshrc",
				To:   "~/.zshrc",
			},
			FileMapping{
				From: "examples/zshrc",
			},
		},
	}
	dNew := d.transform()

	// expands prefix
	cwd, _ := os.Getwd()
	if dNew.FileMappings[0].From != cwd+"/"+d.FileMappings[0].From {
		t.Errorf("path not expanded correctly")
	}

	// expands tilde
	home := os.Getenv("HOME")
	if dNew.FileMappings[0].To != home+"/.zshrc" {
		t.Errorf("path not expanded correctly")
	}

	// infers destination
	if dNew.FileMappings[1].To != home+"/."+d.FileMappings[1].From {
		t.Errorf("home not inferred correctly")
	}

	// defaults As to link
	if dNew.FileMappings[0].As != "link" || dNew.FileMappings[1].As != "link" {
		t.Errorf("default for As not set correctly")
	}

	// valid dots
	d = Dots{
		FileMappings: []FileMapping{
			FileMapping{
				From: "examples/zshrc",
				To:   "~/.zshrc",
			},
		},
		Opts: Opts{
			Cd: "foo",
		},
	}
	dNew = d.transform()
	if dNew.FileMappings[0].From != cwd+"/foo/examples/zshrc" {
		t.Errorf("prefix opt not expanded correctly")
	}
}

func TestReadDotFile(t *testing.T) {
	f := "examples/01-dots-basic.yml"
	dots := readDotFile(f)

	if dots.Opts.Cd != "examples/" {
		t.Errorf("invalid Cd opt")
	}

	cwd, _ := os.Getwd()
	if dots.FileMappings[0].From != cwd+"/examples/gitconfig" {
		t.Error("invalid mapping")
	}
	if dots.FileMappings[1].From != cwd+"/examples/zshrc" {
		t.Error("invalid mapping")
	}
}

func TestInferDestination(t *testing.T) {
	cases := map[string]string{
		".bashrc": os.Getenv("HOME") + "/" + ".bashrc",
		"bashrc":  os.Getenv("HOME") + "/" + ".bashrc",
	}
	for in, want := range cases {
		got := inferDestination(in)
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}
}

func TestExpandTilde(t *testing.T) {
	cases := map[string]string{
		"~/.foo": os.Getenv("HOME") + "/" + ".foo",
		"~/foo":  os.Getenv("HOME") + "/" + "foo",
		"foo":    "foo",
	}
	for in, want := range cases {
		got := expandTilde(in)
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}
}

func TestCreatePath(t *testing.T) {
	f := "some/file"
	p := "some"

	defer func() {
		_ = os.RemoveAll(p)
	}()

	err := createPath(f)
	if err != nil {
		t.Errorf("failed creating path")
	}

	if !pathExists(p) {
		t.Errorf("path not correctly created")
	}
}

func TestPathExists(t *testing.T) {
	cases := map[string]bool{
		"/":               true,
		"/usr":            true,
		os.Getenv("HOME"): true,
		"/foobar":         false,
	}
	for in, want := range cases {
		got := pathExists(in)
		if got != want {
			t.Errorf("got %t, want %t", got, want)
		}
	}
}

func TestIsDirectory(t *testing.T) {
	cases := map[string]bool{
		"/":               true,
		"/bin/ls":         false,
		os.Getenv("HOME"): true,
	}
	for in, want := range cases {
		got := isDirectory(in)
		if got != want {
			t.Errorf("got %t, want %t", got, want)
		}
	}
}

func TestGetHomeDir(t *testing.T) {
	home := os.Getenv("HOME")

	cases := []string{"foo", "bar", home}

	for _, want := range cases {
		os.Setenv("HOME", want)
		got := getHomeDir()
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}
}
