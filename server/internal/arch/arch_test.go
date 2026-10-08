package arch_test

// Dependency-flow guard for the server module tree.
//
// Rule: domain models and business logic must not reference the database,
// HTTP or UI layer. Handlers and repositories may; model.go and service.go
// may not, except for the documented allowlist below.
//
// The allowlist keeps CI green on the violations that exist today. Any NEW
// violation fails the test. If an allowlisted import is removed by a later
// cleanup, the entry becomes stale but the test still passes; delete the
// stale entry in the same commit.
//
// Uses only the standard library. Package dirs are located with `go list`
// (invoked via os/exec); imports are read per file with go/parser so the
// check is file-granular, which package-level `go list -deps` cannot do.

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// allowModelPgtype documents the current pgtype leak in models. New model
// files importing pgtype, or any model importing gin/platform/pgconn, fail.
var allowModelPgtype = map[string]bool{
	"pins":        true, // server/internal/modules/pins/model.go:6
	"collections": true, // server/internal/modules/collections/model.go:6
	"comments":    true, // server/internal/modules/comments/model.go:6
	"reports":     true, // server/internal/modules/reports/model.go:7
}

// allowServicePgconn documents pgconn references in business logic.
// Empty: repositories translate driver errors into domain errors.
var allowServicePgconn = map[string]bool{}

// allowServiceMiddleware documents user/service.go importing the HTTP layer.
var allowServiceMiddleware = map[string]bool{
	"user": true, // server/internal/modules/user/service.go:6 (SessionState)
}

func moduleDirs(t *testing.T) []string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller: cannot locate test file")
	}
	serverRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	var out bytes.Buffer
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", "./internal/modules/...")
	cmd.Dir = serverRoot
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list ./internal/modules/...: %v", err)
	}
	var dirs []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			dirs = append(dirs, line)
		}
	}
	if len(dirs) == 0 {
		t.Fatal("go list returned no module dirs")
	}
	return dirs
}

func fileImports(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var imports []string
	for _, imp := range f.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, p)
	}
	return imports
}

func moduleName(dir string) string {
	return filepath.Base(dir)
}

func TestModelHasNoInfraImports(t *testing.T) {
	for _, dir := range moduleDirs(t) {
		path := filepath.Join(dir, "model.go")
		// Not every module has a model (e.g. auth reuses users.User).
		if _, err := os.Stat(path); err != nil {
			continue
		}
		imports := fileImports(t, path)
		mod := moduleName(dir)
		for _, imp := range imports {
			switch {
			case imp == "github.com/jackc/pgx/v5/pgtype":
				if !allowModelPgtype[mod] {
					t.Errorf("%s: new pgtype import in model (allowlisted: pins, collections, comments, reports)", path)
				}
			case strings.Contains(imp, "gin-gonic/gin"):
				t.Errorf("%s: model must not import gin (%s)", path, imp)
			case strings.Contains(imp, "platform/database"):
				t.Errorf("%s: model must not import platform/database (%s)", path, imp)
			case strings.Contains(imp, "jackc/pgx/v5/pgconn"):
				t.Errorf("%s: model must not import pgconn (%s)", path, imp)
			case strings.Contains(imp, "jackc/pgx/v5/pgxpool"):
				t.Errorf("%s: model must not import pgxpool (%s)", path, imp)
			}
		}
	}
}

func TestServiceHasNoInfraImports(t *testing.T) {
	for _, dir := range moduleDirs(t) {
		path := filepath.Join(dir, "service.go")
		// Not every module has a service (e.g. pins, social use handler+repo).
		// Probe via go list file set instead of failing on missing files.
		imports, missing := tryFileImports(path)
		if missing {
			continue
		}
		mod := moduleName(dir)
		for _, imp := range imports {
			switch {
			case imp == "github.com/jackc/pgx/v5/pgconn":
				if !allowServicePgconn[mod] {
					t.Errorf("%s: service must not import pgconn; repositories translate driver errors into domain errors", path)
				}
			case strings.Contains(imp, "internal/http/middleware"):
				if !allowServiceMiddleware[mod] {
					t.Errorf("%s: new http/middleware import in service (allowlisted: user)", path)
				}
			case strings.Contains(imp, "gin-gonic/gin"):
				t.Errorf("%s: service must not import gin (%s)", path, imp)
			case strings.Contains(imp, "platform/database"):
				t.Errorf("%s: service must not import platform/database (%s)", path, imp)
			}
		}
	}
}

func tryFileImports(path string) ([]string, bool) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, true
	}
	var imports []string
	for _, imp := range f.Imports {
		imports = append(imports, strings.Trim(imp.Path.Value, `"`))
	}
	return imports, false
}
