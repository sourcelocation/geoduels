// Package architecture checks backend module boundaries. Each directory under
// internal/ is a module. A module calls only the generated queries from its
// own query files and imports only the modules listed in allowedImports.
// Adding an entry to either table is a design decision, not a fix.
package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

const queriesImport = "geoduels/pkg/persistence/sqlc/db"

// queryOwners maps each file in db/queries to the module allowed to call its
// queries. "*" marks shared infrastructure any module may use.
var queryOwners = map[string]string{
	"account_deletion.sql": "accounts", "account_readers.sql": "accounts", "account_writes.sql": "accounts",
	"audit.sql": "audit", "auth_sessions.sql": "authsession", "badges.sql": "badges", "chat.sql": "chat",
	"content.sql": "content", "controlplane.sql": "controlplane", "curation.sql": "curation", "leaderboard.sql": "leaderboard",
	"map_comments.sql": "maps", "map_identity.sql": "maps", "map_ingest.sql": "maps", "map_match_plans.sql": "maps",
	"map_revisions.sql": "maps", "map_settings.sql": "maps", "map_stats.sql": "maps", "map_trust.sql": "maps", "maps.sql": "maps",
	"match_readers.sql": "matches", "match_sessions.sql": "matches", "match_writes.sql": "matches",
	"moderation_enforcement.sql": "moderation", "moderation_review.sql": "moderation", "moderation_v2.sql": "moderation", "warnings.sql": "moderation",
	"notifications.sql": "notifications", "parties.sql": "parties", "preferences.sql": "preferences", "profiles.sql": "profiles",
	"notify.sql": "*", "queue.sql": "queue", "rate_limits.sql": "*",
	"settings.sql": "*", "social.sql": "social", "staff.sql": "staff", "storage_cleanup.sql": "storage",
}

// infrastructure modules may be imported by any module; storekit's shared SQL
// helpers are also exempt from query ownership.
var infrastructure = []string{"envcfg", "httpx", "jobs", "rating", "storekit"}

// queryExceptions are calls into another module's queries that predate this
// check. Remove entries as they are fixed; do not add new ones.
var queryExceptions = map[string][]string{
	"accounts": {"BanUserOAuthIdentities", "EnsureRankedStats", "EnsureUserRank", "EnsureUserStats", "GetStaffRoles", "UpsertUser"},
	"jobs":     {"ListDiscordIdentities"},
	"maps":     {"GetProfile", "ListMapCountryStats"},
	"profiles": {"GetLeaderboardTotals"},
	"seasons": {"EnsureRankedSeasonSettings", "GetRankedSeasonSettings", "GetRankedSeasonSettingsForUpdate", "ListRankedSeasonFinishers",
		"SeedRankedSeasonRanks", "SeedRankedSeasonStats", "WriteRankedSeasonSettings"},
	"social": {"GetSocialAccount", "GetSocialSettings", "UpdateSocialSettings"},
}

// allowedImports lists, per module, the other domain modules it may import.
var allowedImports = map[string][]string{
	"badges":     {"accounts", "audit"},
	"curation":   {"badges"},
	"matches":    {"badges", "seasons"},
	"moderation": {"accounts", "audit", "seasons"},
	"parties":    {"badges", "maps"},
	"profiles":   {"badges", "seasons"},
	"seasons":    {"badges"},
	"staff":      {"audit", "badges"},
	"storage":    {"matches"},
}

func TestModulesCallOnlyTheirOwnQueries(t *testing.T) {
	owners := loadQueryOwners(t)
	var violations []string
	for module, calls := range moduleQueryCalls(t) {
		if module == "storekit" {
			continue
		}
		for _, query := range calls {
			owner, known := owners[query]
			if !known || owner == module || owner == "*" || slices.Contains(queryExceptions[module], query) {
				continue
			}
			violations = append(violations, fmt.Sprintf("%s calls %s, owned by %s", module, query, owner))
		}
	}
	report(t, violations, "call the owning module's exported function instead, or move the query to the caller's query file")
}

func TestModulesImportOnlyAllowedModules(t *testing.T) {
	var violations []string
	for module, imports := range moduleImports(t) {
		for _, dep := range imports {
			if !slices.Contains(infrastructure, dep) && !slices.Contains(allowedImports[module], dep) {
				violations = append(violations, fmt.Sprintf("%s imports %s", module, dep))
			}
		}
	}
	report(t, violations, "prefer the owning module's exported API; update allowedImports only after review")
}

func report(t *testing.T, violations []string, hint string) {
	t.Helper()
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("%d boundary violation(s); %s:\n  %s", len(violations), hint, strings.Join(violations, "\n  "))
	}
}

func backendRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

var queryName = regexp.MustCompile(`(?m)^-- name: (\w+)`)

func loadQueryOwners(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(backendRoot(t), "db", "queries", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	owners := map[string]string{}
	for _, path := range files {
		owner, ok := queryOwners[filepath.Base(path)]
		if !ok {
			t.Fatalf("add %s to queryOwners", filepath.Base(path))
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range queryName.FindAllSubmatch(body, -1) {
			owners[string(match[1])] = owner
		}
	}
	return owners
}

// modules maps each module name to its package directories.
func modules(t *testing.T) map[string][]string {
	t.Helper()
	internal := filepath.Join(backendRoot(t), "internal")
	out := map[string][]string{}
	for _, dir := range goPackageDirs(t, internal) {
		rel, _ := filepath.Rel(internal, dir)
		module := strings.Split(filepath.ToSlash(rel), "/")[0]
		if module != "architecture" {
			out[module] = append(out[module], dir)
		}
	}
	return out
}

func goPackageDirs(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			seen[filepath.Dir(path)] = true
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	dirs := make([]string, 0, len(seen))
	for dir := range seen {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

func parseDir(t *testing.T, dir string) []*ast.File {
	t.Helper()
	pkgs, err := parser.ParseDir(token.NewFileSet(), dir, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			files = append(files, file)
		}
	}
	return files
}

func moduleImports(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for module, dirs := range modules(t) {
		deps := map[string]bool{}
		for _, dir := range dirs {
			for _, file := range parseDir(t, dir) {
				for _, spec := range file.Imports {
					path := strings.Trim(spec.Path.Value, `"`)
					if dep, ok := strings.CutPrefix(path, "geoduels/internal/"); ok {
						dep = strings.Split(dep, "/")[0]
						if dep != module {
							deps[dep] = true
						}
					}
				}
			}
		}
		for dep := range deps {
			out[module] = append(out[module], dep)
		}
		sort.Strings(out[module])
	}
	return out
}

// moduleQueryCalls finds calls on generated *db.Queries values. It recognizes
// the receivers this codebase uses: db.New(...), helpers and fields typed
// *db.Queries, WithTx, and variables assigned from those.
func moduleQueryCalls(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for module, dirs := range modules(t) {
		calls := map[string]bool{}
		for _, dir := range dirs {
			files := parseDir(t, dir)
			producers, fields := queryProducers(files)
			for _, file := range files {
				// Files may reach queries through a helper without importing
				// the generated package; alias then matches no declared type.
				alias := importName(file, queriesImport)
				for _, decl := range file.Decls {
					fn, ok := decl.(*ast.FuncDecl)
					if !ok || fn.Body == nil {
						continue
					}
					collectQueryCalls(fn, alias, producers, fields, calls)
				}
			}
		}
		for call := range calls {
			out[module] = append(out[module], call)
		}
		sort.Strings(out[module])
	}
	return out
}

func importName(file *ast.File, path string) string {
	for _, spec := range file.Imports {
		if strings.Trim(spec.Path.Value, `"`) == path {
			if spec.Name != nil {
				return spec.Name.Name
			}
			return "db"
		}
	}
	return ""
}

// queryProducers returns functions/methods that return *db.Queries and struct
// fields declared with that type.
func queryProducers(files []*ast.File) (map[string]bool, map[string]bool) {
	producers, fields := map[string]bool{"QueriesFor": true}, map[string]bool{}
	for _, file := range files {
		alias := importName(file, queriesImport)
		if alias == "" {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				if node.Type.Results != nil && len(node.Type.Results.List) == 1 && isQueriesType(node.Type.Results.List[0].Type, alias) {
					producers[node.Name.Name] = true
				}
			case *ast.StructType:
				for _, field := range node.Fields.List {
					if isQueriesType(field.Type, alias) {
						for _, name := range field.Names {
							fields[name.Name] = true
						}
					}
				}
			}
			return true
		})
	}
	return producers, fields
}

func isQueriesType(expr ast.Expr, alias string) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == alias && sel.Sel.Name == "Queries"
}

func collectQueryCalls(fn *ast.FuncDecl, alias string, producers, fields map[string]bool, calls map[string]bool) {
	vars := map[string]bool{}
	for _, list := range []*ast.FieldList{fn.Recv, fn.Type.Params} {
		if list == nil {
			continue
		}
		for _, field := range list.List {
			if isQueriesType(field.Type, alias) {
				for _, name := range field.Names {
					vars[name.Name] = true
				}
			}
		}
	}
	var isQueries func(ast.Expr) bool
	isQueries = func(expr ast.Expr) bool {
		switch x := expr.(type) {
		case *ast.Ident:
			return vars[x.Name]
		case *ast.SelectorExpr:
			return fields[x.Sel.Name]
		case *ast.CallExpr:
			switch f := x.Fun.(type) {
			case *ast.Ident:
				return producers[f.Name]
			case *ast.SelectorExpr:
				if pkg, ok := f.X.(*ast.Ident); ok && f.Sel.Name == "New" {
					return alias != "" && pkg.Name == alias
				}
				if f.Sel.Name == "WithTx" {
					return isQueries(f.X)
				}
				return producers[f.Sel.Name]
			}
		}
		return false
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && i < len(node.Rhs) && isQueries(node.Rhs[i]) {
					vars[id.Name] = true
				}
			}
		case *ast.ValueSpec:
			if node.Type != nil && isQueriesType(node.Type, alias) {
				for _, name := range node.Names {
					vars[name.Name] = true
				}
			}
		case *ast.CallExpr:
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if ok && isQueries(sel.X) {
				calls[sel.Sel.Name] = true
			}
		}
		return true
	})
}
