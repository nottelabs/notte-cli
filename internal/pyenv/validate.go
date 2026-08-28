package pyenv

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

//go:embed validate.py
var validateScript string

// Verdict is the result of validating one artifact.
type Verdict struct {
	OK     bool     `json:"ok"`
	Stage  string   `json:"stage"`
	Errors []string `json:"errors"`
	// Variables are run()'s parameters, which become invocation variables.
	Variables []Variable `json:"variables"`
}

// Variable is one parameter of run().
type Variable struct {
	Name    string  `json:"name"`
	Type    *string `json:"type"`
	Default *string `json:"default"`
}

// Validate runs the SDK's ScriptValidator against an artifact, with the
// runtime's import list substituted for the SDK's own.
//
// The split is deliberate: the endpoint owns which imports are allowed, and
// the validator owns structure. Trusting the SDK's list would reject
// httpcloak, which ~333 deployed functions import.
func Validate(ctx context.Context, venvDir string, health *Health, source string) (*Verdict, error) {
	if !health.Complete() {
		return nil, fmt.Errorf("cannot validate against a %s runtime report: it carries no import list", health.Status)
	}

	allowed := make([]string, 0, len(health.StdlibModules)+len(health.Packages))
	allowed = append(allowed, health.StdlibModules...)
	for _, p := range health.Packages {
		allowed = append(allowed, p.ImportName)
	}

	request, err := json.Marshal(map[string]any{"source": source, "allowed_imports": allowed})
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, PythonPath(venvDir), "-c", validateScript)
	cmd.Stdin = bytes.NewReader(request)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run validator: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var v Verdict
	if err := json.Unmarshal(stdout.Bytes(), &v); err != nil {
		return nil, fmt.Errorf("decode validator output: %w (stdout: %q)", err, stdout.String())
	}
	return &v, nil
}
