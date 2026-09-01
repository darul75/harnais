package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Workspace struct {
	Root string
}

func NewWorkspace(root string) *Workspace {
	return &Workspace{
		Root: root,
	}
}

// ------------------------------------------------------------
// Secure path resolution
// ------------------------------------------------------------

func (w *Workspace) resolve(
	path string,
) (string, error) {

	if path == "" {
		return "", fmt.Errorf(
			"path is required",
		)
	}

	clean := filepath.Clean(path)

	if filepath.IsAbs(clean) {
		return "", fmt.Errorf(
			"absolute paths are not allowed",
		)
	}

	full := filepath.Join(
		w.Root,
		clean,
	)

	rootAbs, err :=
		filepath.Abs(w.Root)

	if err != nil {
		return "", err
	}

	fullAbs, err :=
		filepath.Abs(full)

	if err != nil {
		return "", err
	}

	relative, err :=
		filepath.Rel(
			rootAbs,
			fullAbs,
		)

	if err != nil {
		return "", err
	}

	if relative == ".." ||
		strings.HasPrefix(
			relative,
			".."+string(os.PathSeparator),
		) {

		return "", fmt.Errorf(
			"path escapes workspace",
		)
	}

	return fullAbs, nil
}

// ------------------------------------------------------------
// read_file
// ------------------------------------------------------------

type ReadFile struct {
	Workspace *Workspace
}

func (ReadFile) ID() string {
	return "read_file"
}

func (t ReadFile) Execute(
	ctx context.Context,
	input map[string]any,
) (map[string]any, error) {

	pathValue, ok :=
		input["path"].(string)

	if !ok {
		return nil, fmt.Errorf(
			"path must be a string",
		)
	}

	path, err :=
		t.Workspace.resolve(
			pathValue,
		)

	if err != nil {
		return nil, err
	}

	content, err :=
		os.ReadFile(path)

	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":    pathValue,
		"content": string(content),
	}, nil
}

// ------------------------------------------------------------
// write_file
// ------------------------------------------------------------

type WriteFile struct {
	Workspace *Workspace
}

func (WriteFile) ID() string {
	return "write_file"
}

func (t WriteFile) Execute(
	ctx context.Context,
	input map[string]any,
) (map[string]any, error) {

	pathValue, ok :=
		input["path"].(string)

	if !ok {
		return nil, fmt.Errorf(
			"path must be a string",
		)
	}

	content, ok :=
		input["content"].(string)

	if !ok {
		return nil, fmt.Errorf(
			"content must be a string",
		)
	}

	path, err :=
		t.Workspace.resolve(
			pathValue,
		)

	if err != nil {
		return nil, err
	}

	if err :=
		os.MkdirAll(
			filepath.Dir(path),
			0o755,
		); err != nil {
		return nil, err
	}

	if err :=
		os.WriteFile(
			path,
			[]byte(content),
			0o644,
		); err != nil {
		return nil, err
	}

	return map[string]any{
		"path":    pathValue,
		"written": true,
	}, nil
}

// ------------------------------------------------------------
// run_command
// ------------------------------------------------------------

type RunCommand struct {
	Workspace *Workspace
}

func (RunCommand) ID() string {
	return "run_command"
}

func (t RunCommand) Execute(
	ctx context.Context,
	input map[string]any,
) (map[string]any, error) {

	command, ok :=
		input["command"].(string)

	if !ok {
		return nil, fmt.Errorf(
			"command must be a string",
		)
	}

	cmd :=
		exec.CommandContext(
			ctx,
			"sh",
			"-c",
			command,
		)

	cmd.Dir =
		t.Workspace.Root

	output, err :=
		cmd.CombinedOutput()

	result :=
		map[string]any{
			"command": command,

			"output": string(output),
		}

	if err != nil {

		result["exitCode"] =
			cmd.ProcessState.ExitCode()

		return result, err
	}

	result["exitCode"] = 0

	return result, nil
}

// ------------------------------------------------------------
// git_diff
// ------------------------------------------------------------

type GitDiff struct {
	Workspace *Workspace
}

func (GitDiff) ID() string {
	return "git_diff"
}

func (t GitDiff) Execute(
	ctx context.Context,
	input map[string]any,
) (map[string]any, error) {

	cmd :=
		exec.CommandContext(
			ctx,
			"git",
			"diff",
			"--",
		)

	cmd.Dir =
		t.Workspace.Root

	output, err :=
		cmd.CombinedOutput()

	if err != nil {
		return map[string]any{
			"output": string(output),
		}, err
	}

	return map[string]any{
		"output": string(output),
	}, nil
}
