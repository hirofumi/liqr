package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/osteele/liquid/filters"
	"github.com/osteele/liquid/parser"
	"github.com/osteele/liquid/render"
	"github.com/osteele/liquid/tags"
	"gopkg.in/yaml.v2"
)

func main() {
	n := len(os.Args)
	if n < 2 || 3 < n {
		_, _ = fmt.Fprintf(os.Stderr, "Usage\n  %s source-file [destination-file]\n", path.Base(os.Args[0]))
		os.Exit(1)
	}

	cfg := renderConfig()

	node, err := parse(os.Args[1], cfg)
	if err != nil {
		panic(err)
	}

	var w io.WriteCloser

	switch n {
	case 2:
		w = os.Stdout
	case 3:
		w, err = os.OpenFile(os.Args[2], os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			panic(err)
		}
		defer w.Close()
	}

	if err := render.Render(node, w, nil, cfg); err != nil {
		panic(err)
	}
}

func renderConfig() render.Config {
	cfg := render.NewConfig()

	filters.AddStandardFilters(&cfg)
	tags.AddStandardTags(cfg)
	cfg.AddFilter("bash", bashFilter)
	cfg.AddFilter("prompt", promptFilter)
	cfg.AddFilter("select", selectFilter)
	cfg.AddFilter("yaml", yamlFilter)

	return cfg
}

func parse(name string, cfg render.Config) (render.Node, error) {
	source, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}

	node, err := cfg.Compile(string(source), parser.SourceLoc{Pathname: name})
	if err != nil {
		return nil, err
	}

	return node, nil
}

func bashFilter(input string, script func(string) string) (string, error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("set -euo pipefail; %s", script(input)))

	r, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("bash: failed to get stderr: %w", err)
	}

	var stderr []byte

	go func() {
		defer r.Close()
		stderr, _ = io.ReadAll(r)
	}()

	w, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("bash: failed to get stdin: %w", err)
	}

	go func() {
		defer w.Close()
		io.WriteString(w, input)
	}()

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(stderr)), err)
	}

	return string(output), nil
}

func promptFilter(pattern string, label string, initial func(string) string) (interface{}, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("prompt: failed to compile pattern: %w", err)
	}

	validate := func(s string) error {
		if !regex.MatchString(s) {
			return fmt.Errorf("invalid value (required pattern: %s)", regex.String())
		}

		return nil
	}

	prompt := promptui.Prompt{
		Label:     label,
		Default:   initial(""),
		AllowEdit: true,
		Validate:  validate,
	}

	value, err := prompt.Run()
	if err != nil {
		return nil, fmt.Errorf("prompt: failed to run: %w", err)
	}

	return value, nil
}

func selectFilter(values []interface{}, label string) (interface{}, error) {
	sel := promptui.Select{
		Label: label,
		Items: values,
	}

	i, _, err := sel.Run()
	if err != nil {
		return nil, fmt.Errorf("select: failed to run: %w", err)
	}

	return values[i], nil
}

func yamlFilter(source string) (interface{}, error) {
	var got interface{}

	if err := yaml.Unmarshal([]byte(source), &got); err != nil {
		return nil, fmt.Errorf("yaml: failed to unmarshal: %w", err)
	}

	return got, nil
}
