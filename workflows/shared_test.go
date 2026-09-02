package workflows

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"harnais/config"
	"harnais/tools"
)

func TestTestCommandGoModInWorkspace(t *testing.T) {

	root :=
		filepath.Join(t.TempDir(), "ws")

	if err :=
		os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	if err :=
		os.WriteFile(
			filepath.Join(root, "go.mod"),
			[]byte("module demo\n"),
			0o644,
		); err != nil {
		t.Fatal(err)
	}

	s :=
		NewShared(
			tools.NewWorkspace(root),
			config.NewStore(""),
		)

	program, args, dir :=
		s.testCommand()

	if program != "go" {
		t.Errorf("expected go, got %q", program)
	}

	if !reflect.DeepEqual(
		args,
		[]string{"test", "-v", "./..."},
	) {
		t.Errorf("expected go test ./..., got %v", args)
	}

	if dir != root {
		t.Errorf("expected dir %s, got %s", root, dir)
	}
}

func TestTestCommandGoModInParent(t *testing.T) {

	root :=
		filepath.Join(t.TempDir(), "repo", "ws")

	if err :=
		os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	parent :=
		filepath.Dir(root)

	if err :=
		os.WriteFile(
			filepath.Join(parent, "go.mod"),
			[]byte("module demo\n"),
			0o644,
		); err != nil {
		t.Fatal(err)
	}

	s :=
		NewShared(
			tools.NewWorkspace(root),
			config.NewStore(""),
		)

	program, args, dir :=
		s.testCommand()

	if program != "go" {
		t.Errorf("expected go, got %q", program)
	}

	if !reflect.DeepEqual(
		args,
		[]string{"test", "-v", "./..."},
	) {
		t.Errorf("expected go test ./..., got %v", args)
	}

	if dir != root {
		t.Errorf("expected dir %s, got %s", root, dir)
	}
}

func TestTestCommandPackageJSON(t *testing.T) {

	root :=
		filepath.Join(t.TempDir(), "ws")

	if err :=
		os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	if err :=
		os.WriteFile(
			filepath.Join(root, "package.json"),
			[]byte("{}\n"),
			0o644,
		); err != nil {
		t.Fatal(err)
	}

	s :=
		NewShared(
			tools.NewWorkspace(root),
			config.NewStore(""),
		)

	program, args, dir :=
		s.testCommand()

	if program != "npm" {
		t.Errorf("expected npm, got %q", program)
	}

	if !reflect.DeepEqual(
		args,
		[]string{"test"},
	) {
		t.Errorf("expected npm test, got %v", args)
	}

	if dir != root {
		t.Errorf("expected dir %s, got %s", root, dir)
	}
}

func TestTestCommandNone(t *testing.T) {

	root :=
		filepath.Join(t.TempDir(), "ws")

	if err :=
		os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	s :=
		NewShared(
			tools.NewWorkspace(root),
			config.NewStore(""),
		)

	program, args, _ :=
		s.testCommand()

	if program != "" {
		t.Errorf("expected empty program, got %q", program)
	}

	if args != nil {
		t.Errorf("expected nil args, got %v", args)
	}
}

func TestSummarizeTestOutput(t *testing.T) {

	cases := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "pass and fail counts",
			output: "--- PASS: TestA (0.00s)\n" +
				"--- PASS: TestB (0.00s)\n" +
				"--- FAIL: TestC (0.00s)\n",
			want: "[summary] 3 tests: 2 passed, 1 failed",
		},
		{
			name: "all pass",
			output: "--- PASS: TestA (0.00s)\n" +
				"--- PASS: TestB (0.00s)\n",
			want: "[summary] 2 tests: 2 passed, 0 failed",
		},
		{
			name:   "no tests",
			output: "? harnais/workspace [no test files]\n",
			want:   "",
		},
		{
			name:   "empty",
			output: "",
			want:   "",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := summarizeTestOutput(test.output); got != test.want {
				t.Errorf("summarizeTestOutput(%q) = %q, want %q", test.output, got, test.want)
			}
		})
	}
}