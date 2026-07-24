//go:build unit

package service

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceScope_GenericSettingsUpdateExcludesMarketplaceKey(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	require.NoError(t, svc.UpdateSettings(context.Background(), &SystemSettings{}))
	require.NotContains(t, repo.updates, SettingKeyMarketplaceEnabled)
}

func TestMarketplaceScope_GlobalRoleModelRemainsUserAndAdmin(t *testing.T) {
	require.Equal(t, [2]string{RoleUser, RoleAdmin}, supportedGlobalUserRoles)

	for _, role := range supportedGlobalUserRoles {
		got, err := normalizeUserRole(role, RoleUser)
		require.NoError(t, err)
		require.Equal(t, role, got)
	}

	for _, role := range []string{"merchant", "supplier", "seller", "operator", "root", "superuser"} {
		_, err := normalizeUserRole(role, RoleUser)
		require.Error(t, err)
	}
}

func TestMarketplaceScope_DomainRoleConstantsRemainUserAndAdmin(t *testing.T) {
	root := marketplaceRepositoryRoot(t)
	path := filepath.Join(root, "backend", "internal", "domain", "constants.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	require.NoError(t, err)

	var roles []string
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}
		for _, specification := range general.Specs {
			values, ok := specification.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range values.Names {
				if strings.HasPrefix(name.Name, "Role") {
					roles = append(roles, name.Name)
				}
			}
		}
	}

	slices.Sort(roles)
	require.Equal(t, []string{"RoleAdmin", "RoleUser"}, roles)
}

func TestMarketplaceScope_NoProductionRoutesNavigationOrAdminWrites(t *testing.T) {
	root := marketplaceRepositoryRoot(t)
	assertNoMarketplaceProductionSurface(t, root,
		"backend/internal/server/routes",
		"backend/internal/handler/admin",
		"frontend/src/router",
		"frontend/src/components/layout/AppSidebar.vue",
		"frontend/src/api/admin",
		"frontend/src/views/admin",
	)
}

func TestMarketplaceScope_NoMarketplaceMigrations(t *testing.T) {
	root := marketplaceRepositoryRoot(t)
	migrationRoot := filepath.Join(root, "backend", "migrations")
	marketplaceDDL := regexp.MustCompile(`(?i)\b(?:create|alter|drop)\s+table(?:\s+if\s+(?:not\s+)?exists)?\s+(?:[a-z0-9_]+\.)?"?marketplace(?:_|"?\b)`)

	require.NoError(t, filepath.WalkDir(migrationRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".sql" {
			return nil
		}

		require.NotContains(t, strings.ToLower(entry.Name()), "marketplace", path)
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.False(t, marketplaceDDL.Match(content), path)
		require.NotContains(t, strings.ToLower(string(content)), "marketplace_", path)
		return nil
	}))
}

func marketplaceRepositoryRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func assertNoMarketplaceProductionSurface(t *testing.T, root string, relativePaths ...string) {
	t.Helper()

	for _, relativePath := range relativePaths {
		path := filepath.Join(root, relativePath)
		info, err := os.Stat(path)
		require.NoError(t, err)

		if !info.IsDir() {
			assertFileHasNoMarketplaceSurface(t, path)
			continue
		}

		require.NoError(t, filepath.WalkDir(path, func(filePath string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if entry.Name() == "__tests__" {
					return filepath.SkipDir
				}
				return nil
			}
			assertFileHasNoMarketplaceSurface(t, filePath)
			return nil
		}))
	}
}

func assertFileHasNoMarketplaceSurface(t *testing.T, path string) {
	t.Helper()

	name := filepath.Base(path)
	if strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, ".spec.ts") {
		return
	}
	switch filepath.Ext(path) {
	case ".go", ".ts", ".vue", ".sql":
	default:
		return
	}

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	lower := strings.ToLower(string(content))
	require.NotContains(t, lower, "marketplace", path)
	require.NotContains(t, lower, "marketplace_enabled", path)
	require.NotContains(t, string(content), "模型商城", path)
}
