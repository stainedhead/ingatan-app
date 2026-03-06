package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	apimw "github.com/stainedhead/ingatan/internal/adapter/rest/middleware"
	"github.com/stainedhead/ingatan/internal/domain"
	backuppkg "github.com/stainedhead/ingatan/internal/infrastructure/backup"
)

// BackupHandler handles POST /admin/backup, triggering all configured backup providers.
// It is admin-only. Implements RouteRegistrar.
type BackupHandler struct {
	providers []backuppkg.Backuper
	dataDir   string
}

// NewBackupHandler creates a new BackupHandler.
func NewBackupHandler(providers []backuppkg.Backuper, dataDir string) *BackupHandler {
	return &BackupHandler{providers: providers, dataDir: dataDir}
}

// Register mounts the backup route on the given Chi router.
// Expects to be registered under /api/v1 (already authenticated).
func (h *BackupHandler) Register(r chi.Router) {
	r.Post("/admin/backup", h.triggerBackup)
}

// backupResult holds the outcome of a single backup provider run.
type backupResult struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

func (h *BackupHandler) triggerBackup(w http.ResponseWriter, r *http.Request) {
	p := apimw.PrincipalFromContext(r.Context())
	if p == nil || p.Role != domain.InstanceRoleAdmin {
		WriteError(w, http.StatusForbidden, domain.ErrCodeForbidden, "admin access required")
		return
	}

	results := make([]backupResult, 0, len(h.providers))
	for _, provider := range h.providers {
		res := backupResult{Provider: provider.Name()}
		if err := provider.Backup(r.Context(), h.dataDir); err != nil {
			res.Status = "error"
			res.Error = err.Error()
		} else {
			res.Status = "ok"
		}
		results = append(results, res)
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}
