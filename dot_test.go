package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"os"
	"runtime"
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
	assert.Nil(t, createPath(to))
	defer func() {
		_ = os.RemoveAll(to)
	}()

	// creates a symlink
	m := FileMapping{
		From: "examples/bashrc",
		To:   "out/bashrc",
	}

	assert.Nil(t, m.doLink())
	assert.True(t, isSymlink(m.To))
}

func TestDoCopy(t *testing.T) {
	to := "out/"
	assert.Nil(t, createPath(to))
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

	err := m.doCopy()
	assert.Nil(t, err)
	assert.False(t, isSymlink(m.To))

	// same contents

	var fromContents []byte
	var toContents []byte
	fromContents, err = os.ReadFile(m.From)
	assert.Nil(t, err)
	toContents, err = os.ReadFile(m.To)
	assert.Nil(t, err)
	assert.Equal(t, fromContents, toContents)

	// copies a file with template
	m = FileMapping{
		From: "fixtures/gpg-agent.conf.input",
		To:   "out/gpg-agent.conf",
		With: map[string]string{
			"PinentryPath": "/foo/bar",
		},
	}

	err = m.doCopy()
	assert.Nil(t, err)
	assert.False(t, isSymlink(m.To))

	// correct contents

	fromContents, err = os.ReadFile("fixtures/gpg-agent.conf.output")
	assert.Nil(t, err)
	toContents, err = os.ReadFile(m.To)
	assert.Nil(t, err)
	assert.Equal(t, fromContents, toContents)
}

func TestUnmap(t *testing.T) {
	// removes a symlink
	to := "out/"
	assert.Nil(t, createPath(to))
	defer func() {
		_ = os.RemoveAll(to)
	}()

	// creates a symlink
	m := FileMapping{
		From: "examples/bashrc",
		To:   "out/bashrc",
	}
	assert.Nil(t, m.doLink())

	m.unmap()
	assert.False(t, pathExists(m.To))
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

	m.domap()
	assert.True(t, isSymlink(m.To))

	// creates path
	assert.True(t, pathExists("examples"))

	// creates a copy
	m = FileMapping{
		From: "examples/zshrc",
		To:   "out/foobar",
		As:   "copy",
	}

	m.domap()
	assert.False(t, isSymlink(m.To))

	// same contents

	var fromContents []byte
	var toContents []byte
	fromContents, err := os.ReadFile(m.From)
	assert.Nil(t, err)
	toContents, err = os.ReadFile(m.To)
	assert.Nil(t, err)
	assert.Equal(t, fromContents, toContents)
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
	assert.Nil(t, yaml.Unmarshal([]byte(dotData), &dots))
	assert.Equal(t, dots.Opts.Cd, "foo")
	assert.Equal(t, len(dots.FileMappings), 3)
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
	errs := d.validate()
	assert.Nil(t, errs)

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
	errs = d.validate()
	assert.Equal(t, len(errs), 1)
	assert.Contains(t, errs, fmt.Errorf("%s: path does not exist", d.FileMappings[0].From))

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
	errs = d.validate()
	assert.Equal(t, len(errs), 1)
	assert.Contains(t, errs, fmt.Errorf("%s: cannot use copy type with directory", d.FileMappings[0].From))
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
	assert.NotNil(t, dNew)

	// expands prefix
	cwd, err := os.Getwd()
	assert.Nil(t, err)
	assert.Equal(t, dNew.FileMappings[0].From, cwd+"/"+d.FileMappings[0].From)

	// expands tilde
	home := os.Getenv("HOME")
	assert.NotNil(t, home)
	assert.Equal(t, dNew.FileMappings[0].To, home+"/.zshrc")

	// infers destination
	assert.Equal(t, dNew.FileMappings[1].To, home+"/."+d.FileMappings[1].From)

	// defaults As to link
	assert.Equal(t, dNew.FileMappings[0].As, "link")
	assert.Equal(t, dNew.FileMappings[1].As, "link")

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
	assert.NotNil(t, dNew)
	assert.Equal(t, dNew.FileMappings[0].From, cwd+"/foo/examples/zshrc")
}

func TestReadDotFile(t *testing.T) {
	f := "examples/01-dots-basic.yml"
	dots := readDotFile(f)
	assert.NotNil(t, dots)

	assert.Equal(t, dots.Opts.Cd, "examples/")

	cwd, err := os.Getwd()
	assert.Nil(t, err)
	assert.NotNil(t, cwd)

	assert.Contains(t, dots.FileMappings, FileMapping{
		From: cwd + "/examples/gitconfig",
		To:   "out/gitconfig",
		As:   "link",
		Os:   "",
	})
	assert.Contains(t, dots.FileMappings, FileMapping{
		From: cwd + "/examples/zshrc",
		To:   "out/zshrc",
		As:   "link",
		Os:   "",
	})
}

func TestInferDestination(t *testing.T) {
	cases := map[string]string{
		".bashrc": os.Getenv("HOME") + "/" + ".bashrc",
		"bashrc":  os.Getenv("HOME") + "/" + ".bashrc",
	}
	for in, want := range cases {
		got := inferDestination(in)
		assert.Equal(t, got, want)
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
		assert.Equal(t, got, want)
	}
}

func TestCreatePath(t *testing.T) {
	f := "some/file"
	p := "some"

	defer func() {
		_ = os.RemoveAll(p)
	}()

	assert.Nil(t, createPath(f))
	assert.True(t, pathExists(p))
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
		assert.Equal(t, got, want)
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
		assert.Equal(t, got, want)
	}
}

func TestGetHomeDir(t *testing.T) {
	home := os.Getenv("HOME")

	cases := []string{"foo", "bar", home}

	for _, want := range cases {
		os.Setenv("HOME", want)
		got := getHomeDir()
		assert.Equal(t, got, want)
	}
}

func TestEvalTemplateString(t *testing.T) {
	cases := map[string]string{
		"{{ .v1 }}": "a value",
		"{{ .v2 }}": "<no value>",
	}
	env := map[string]string{
		"v1": "a value",
	}
	for templ, want := range cases {
		got := evalTemplateString(templ, env)
		assert.Equal(t, want, got)
	}
}

func TestEvalTemplate(t *testing.T) {
	curOs := runtime.GOOS
	var otherOs string
	if curOs == "darwin" {
		otherOs = "linux"
	} else {
		otherOs = "darwin"
	}
	with := map[string]string{
		"t1": "{{if eq .Os \"" + curOs + "\"}}it works{{end}}",
		"t2": "{{if eq .Os \"" + otherOs + "\"}}must not be this{{end}}",
		"t3": "{{if eq .Os \"" + otherOs + "\"}}must not be this{{else}}else{{end}}",
	}

	res := evalTemplate(with)
	assert.Equal(t, "it works", res["t1"])
	assert.Equal(t, "", res["t2"], "")
	assert.Equal(t, "else", res["t3"])
}
