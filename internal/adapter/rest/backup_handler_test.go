package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	backuppkg "github.com/stainedhead/ingatan/internal/infrastructure/backup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBackuper is a test double for backup.Backuper.
type mockBackuper struct {
	name     string
	backupFn func(ctx context.Context, dataDir string) error
}

func (m *mockBackuper) Backup(ctx context.Context, dataDir string) error {
	return m.backupFn(ctx, dataDir)
}

func (m *mockBackuper) Name() string { return m.name }

// backupHandlerTestSecret and helpers — use different values from memHandlerTestSecret
// to avoid redeclaration (they live in the same package during test).
var backupHandlerTestSecret = []byte("backup-handler-test-secret-32!!")

func backupAdminToken() string {
	claims := apimw.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "admin-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Name: "Admin User",
		Type: domain.PrincipalTypeHuman,
		Role: domain.InstanceRoleAdmin,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString(backupHandlerTestSecret)
	return signed
}

func backupUserToken() string {
	claims := apimw.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Name: "Regular User",
		Type: domain.PrincipalTypeHuman,
		Role: domain.InstanceRoleUser,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString(backupHandlerTestSecret)
	return signed
}

func backupTestLookup(_ context.Context, claims apimw.JWTClaims) (*domain.Principal, error) {
	return &domain.Principal{
		ID:   claims.Subject,
		Name: claims.Name,
		Type: claims.Type,
		Role: claims.Role,
	}, nil
}

func newBackupTestRouter(providers []backuppkg.Backuper, dataDir string) *chi.Mux {
	r := chi.NewRouter()
	h := NewBackupHandler(providers, dataDir)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(apimw.JWTMiddleware(backupHandlerTestSecret, backupTestLookup))
		h.Register(r)
	})
	return r
}

func newBackupReq(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestBackupHandler_AdminSuccess(t *testing.T) {
	called := false
	providers := []backuppkg.Backuper{
		&mockBackuper{
			name: "s3",
			backupFn: func(_ context.Context, _ string) error {
				called = true
				return nil
			},
		},
	}

	rr := httptest.NewRecorder()
	newBackupTestRouter(providers, "/tmp/data").ServeHTTP(
		rr, newBackupReq(http.MethodPost, "/api/v1/admin/backup", backupAdminToken()),
	)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called, "backup provider should have been invoked")

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	results, ok := resp["results"].([]any)
	require.True(t, ok)
	require.Len(t, results, 1)
	result := results[0].(map[string]any)
	assert.Equal(t, "s3", result["provider"])
	assert.Equal(t, "ok", result["status"])
}

func TestBackupHandler_NonAdminForbidden(t *testing.T) {
	providers := []backuppkg.Backuper{
		&mockBackuper{
			name: "git",
			backupFn: func(_ context.Context, _ string) error {
				return nil
			},
		},
	}

	rr := httptest.NewRecorder()
	newBackupTestRouter(providers, "/tmp/data").ServeHTTP(
		rr, newBackupReq(http.MethodPost, "/api/v1/admin/backup", backupUserToken()),
	)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestBackupHandler_ProviderFailureReportedInBody(t *testing.T) {
	providers := []backuppkg.Backuper{
		&mockBackuper{
			name: "s3",
			backupFn: func(_ context.Context, _ string) error {
				return errors.New("connection refused")
			},
		},
		&mockBackuper{
			name: "git",
			backupFn: func(_ context.Context, _ string) error {
				return nil
			},
		},
	}

	rr := httptest.NewRecorder()
	newBackupTestRouter(providers, "/tmp/data").ServeHTTP(
		rr, newBackupReq(http.MethodPost, "/api/v1/admin/backup", backupAdminToken()),
	)

	// HTTP 200 always — individual errors in body.
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	results, ok := resp["results"].([]any)
	require.True(t, ok)
	require.Len(t, results, 2)

	byProvider := make(map[string]map[string]any)
	for _, item := range results {
		m := item.(map[string]any)
		byProvider[m["provider"].(string)] = m
	}

	s3 := byProvider["s3"]
	assert.Equal(t, "error", s3["status"])
	assert.Equal(t, "connection refused", s3["error"])

	git := byProvider["git"]
	assert.Equal(t, "ok", git["status"])
	_, hasErr := git["error"]
	assert.False(t, hasErr, "successful provider should not include error field")
}

func TestBackupHandler_NoProviders(t *testing.T) {
	rr := httptest.NewRecorder()
	newBackupTestRouter(nil, "/tmp/data").ServeHTTP(
		rr, newBackupReq(http.MethodPost, "/api/v1/admin/backup", backupAdminToken()),
	)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	results, ok := resp["results"].([]any)
	require.True(t, ok)
	assert.Empty(t, results)
}
