package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/osteele/liquid/expressions"
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
	cfg.AddFilter("yaml", yamlFilter)
	cfg.AddTag("prompt", promptTag)
	cfg.AddTag("select", selectTag)

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

func yamlFilter(source string) (interface{}, error) {
	var got interface{}

	if err := yaml.Unmarshal([]byte(source), &got); err != nil {
		return nil, fmt.Errorf("yaml: failed to unmarshal: %w", err)
	}

	return got, nil
}

func promptTag(source string) (func(io.Writer, render.Context) error, error) {
	stmt, err := expressions.ParseStatement(expressions.AssignStatementSelector, source)
	if err != nil {
		return nil, fmt.Errorf("prompt: failed to parse: %w", err)
	}

	return func(w io.Writer, ctx render.Context) error {
		defaultValue, err := ctx.Evaluate(stmt.Assignment.ValueFn)
		if err != nil {
			return fmt.Errorf("prompt: failed to evaluate default value: %w", err)
		}

		value := fmt.Sprint(defaultValue)

		prompt := promptui.Prompt{
			Label:     stmt.Assignment.Variable,
			Default:   value,
			AllowEdit: true,
		}

		value, err = prompt.Run()
		if err != nil {
			return fmt.Errorf("prompt: failed to run: %w", err)
		}

		ctx.Set(stmt.Assignment.Variable, value)

		return nil
	}, nil
}

func selectTag(source string) (func(io.Writer, render.Context) error, error) {
	ss := append(strings.SplitN(source, "=", 2), "", "")

	stmt, err := expressions.ParseStatement(expressions.AssignStatementSelector, ss[0]+` = ""`)
	if err != nil {
		return nil, fmt.Errorf("select: failed to parse lhs: %w", err)
	}
	assignment := stmt.Assignment

	stmt, err = expressions.ParseStatement(expressions.WhenStatementSelector, ss[1])
	if err != nil {
		return nil, fmt.Errorf("select: failed to parse rhs: %w", err)
	}
	when := stmt.When

	return func(w io.Writer, ctx render.Context) error {
		var err error

		values := make([]interface{}, len(when.Exprs))
		for i := range when.Exprs {
			values[i], err = ctx.Evaluate(when.Exprs[i])
			if err != nil {
				return fmt.Errorf("select: failed to evaluate (%d): %w", i, err)
			}
		}

		sel := promptui.Select{
			Label: assignment.Variable,
			Items: values,
		}

		i, _, err := sel.Run()
		if err != nil {
			return fmt.Errorf("select: failed to run: %w", err)
		}

		ctx.Set(assignment.Variable, values[i])

		return nil
	}, nil
}
