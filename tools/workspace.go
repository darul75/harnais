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

	AllowedCommands map[string]bool
}

func NewWorkspace(
	root string,
) *Workspace {

	return &Workspace{
		Root: root,

		AllowedCommands: map[string]bool{
			"go":   true,
			"git":  true,
			"node": true,
			"npm":  true,
			"pnpm": true,
			"yarn": true,
		},
	}
}

// ------------------------------------------------------------
// Path safety
// ------------------------------------------------------------

func (w *Workspace) Resolve(
	path string,
) (string, error) {

	if path == "" {
		return "", fmt.Errorf(
			"path is required",
		)
	}

	clean :=
		filepath.Clean(path)

	if filepath.IsAbs(clean) {
		return "", fmt.Errorf(
			"absolute paths are not allowed",
		)
	}

	root, err :=
		filepath.Abs(w.Root)

	if err != nil {
		return "", err
	}

	full, err :=
		filepath.Abs(
			filepath.Join(
				root,
				clean,
			),
		)

	if err != nil {
		return "", err
	}

	relative, err :=
		filepath.Rel(
			root,
			full,
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

	return full, nil
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

func (ReadFile) Description() string {
	return "Read a UTF-8 text file from the workspace."
}

func (ReadFile) Parameters() map[string]any {
	return map[string]any{
		"type": "object",

		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Workspace-relative file path",
			},
		},

		"required": []string{
			"path",
		},

		"additionalProperties": false,
	}
}

func (t ReadFile) Execute(
	ctx context.Context,
	input map[string]any,
) (map[string]any, error) {

	path, ok :=
		input["path"].(string)

	if !ok {
		return nil, fmt.Errorf(
			"path must be a string",
		)
	}

	fmt.Printf(
		"[read_file] requested path=%q workspace=%q\n",
		path,
		t.Workspace.Root,
	)

	resolved, err :=
		t.Workspace.Resolve(path)

	if err != nil {
		return nil, err
	}

	content, err :=
		os.ReadFile(resolved)

	if err != nil {

		if os.IsNotExist(err) {
			return map[string]any{
				"path":    path,
				"exists":  false,
				"content": "",
			}, nil
		}

		return nil, err
	}

	return map[string]any{
		"path":    path,
		"exists":  true,
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

func (WriteFile) Description() string {
	return "Write UTF-8 text content to a workspace-relative file."
}

func (WriteFile) Parameters() map[string]any {
	return map[string]any{
		"type": "object",

		"properties": map[string]any{
			"path": map[string]any{
				"type": "string",
			},

			"content": map[string]any{
				"type": "string",
			},
		},

		"required": []string{
			"path",
			"content",
		},

		"additionalProperties": false,
	}
}

func (t WriteFile) Execute(
	ctx context.Context,
	input map[string]any,
) (map[string]any, error) {

	path, ok :=
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

	resolved, err :=
		t.Workspace.Resolve(path)

	if err != nil {
		return nil, err
	}

	if err :=
		os.MkdirAll(
			filepath.Dir(resolved),
			0o755,
		); err != nil {
		return nil, err
	}

	if err :=
		os.WriteFile(
			resolved,
			[]byte(content),
			0o644,
		); err != nil {
		return nil, err
	}

	return map[string]any{
		"path": path,

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

func (RunCommand) Description() string {
	return "Run an allowed development command in the workspace."
}

func (RunCommand) Parameters() map[string]any {
	return map[string]any{
		"type": "object",

		"properties": map[string]any{
			"program": map[string]any{
				"type": "string",
			},

			"args": map[string]any{
				"type": "array",

				"items": map[string]any{
					"type": "string",
				},
			},
		},

		"required": []string{
			"program",
			"args",
		},

		"additionalProperties": false,
	}
}

func (t RunCommand) Execute(
	ctx context.Context,
	input map[string]any,
) (map[string]any, error) {

	program, ok :=
		input["program"].(string)

	if !ok {
		return nil, fmt.Errorf(
			"program must be a string",
		)
	}

	if !t.Workspace.AllowedCommands[program] {
		return nil, fmt.Errorf(
			"command %q is not allowed",
			program,
		)
	}

	rawArgs, ok :=
		input["args"].([]any)

	if !ok {
		return nil, fmt.Errorf(
			"args must be an array",
		)
	}

	args :=
		make(
			[]string,
			0,
			len(rawArgs),
		)

	for _, rawArg := range rawArgs {

		arg, ok :=
			rawArg.(string)

		if !ok {
			return nil, fmt.Errorf(
				"all args must be strings",
			)
		}

		args =
			append(
				args,
				arg,
			)
	}

	cmd :=
		exec.CommandContext(
			ctx,
			program,
			args...,
		)

	cmd.Dir =
		t.Workspace.Root

	output, err :=
		cmd.CombinedOutput()

	result :=
		map[string]any{
			"program": program,

			"args": args,

			"output": string(output),
		}

	if err != nil {

		if cmd.ProcessState != nil {
			result["exitCode"] =
				cmd.ProcessState.ExitCode()
		}

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

func (GitDiff) Description() string {
	return "Show the current git diff for the workspace."
}

func (GitDiff) Parameters() map[string]any {
	return map[string]any{
		"type": "object",

		"properties": map[string]any{},

		"required": []string{},

		"additionalProperties": false,
	}
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

type ListFiles struct {
	Workspace *Workspace
}

func (ListFiles) ID() string {
	return "list_files"
}

func (ListFiles) Description() string {
	return `List files and directories inside the workspace.

The path is relative to the workspace root.
Use "." to list the workspace root.
Do not prefix paths with "workspace/".

The .git directory is intentionally excluded.`
}

func (ListFiles) Parameters() map[string]any {
	return map[string]any{
		"type": "object",

		"properties": map[string]any{
			"path": map[string]any{
				"type": "string",

				"description": "Workspace-relative directory. Use '.' for the workspace root.",
			},
		},

		"required": []string{
			"path",
		},

		"additionalProperties": false,
	}
}

func (t ListFiles) Execute(
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

	// Treat "." as the workspace root.
	if pathValue == "" {
		pathValue = "."
	}

	resolved, err :=
		t.Workspace.Resolve(pathValue)

	if err != nil {
		return nil, err
	}

	entries, err :=
		os.ReadDir(resolved)

	if err != nil {

		// Not found is a normal discovery result.
		if os.IsNotExist(err) {
			return map[string]any{
				"path":    pathValue,
				"exists":  false,
				"entries": []any{},
			}, nil
		}

		return nil, fmt.Errorf(
			"list_files %q: %w",
			pathValue,
			err,
		)
	}

	result := make(
		[]map[string]any,
		0,
		len(entries),
	)

	for _, entry := range entries {

		// Never expose .git.
		if entry.Name() == ".git" {
			continue
		}

		// Also ignore common internal directories.
		// Keep this list small for now.
		if entry.Name() == ".DS_Store" {
			continue
		}

		result =
			append(
				result,
				map[string]any{
					"name": entry.Name(),

					"isDir": entry.IsDir(),
				},
			)
	}

	return map[string]any{
		"path": pathValue,

		"exists": true,

		"entries": result,
	}, nil
}
