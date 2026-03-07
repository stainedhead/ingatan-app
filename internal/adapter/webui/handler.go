package webui

//go:generate templ generate ./templates/

import (
	"crypto/subtle"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/stainedhead/ingatan/internal/adapter/webui/templates"
	"github.com/stainedhead/ingatan/internal/domain"
	"github.com/stainedhead/ingatan/internal/infrastructure/backup"
	principaluc "github.com/stainedhead/ingatan/internal/usecase/principal"
	storeuc "github.com/stainedhead/ingatan/internal/usecase/store"
)

const sessionTTL = 24 * time.Hour

// adminPrincipal is the synthetic admin principal injected into all WebUI service calls.
// It bypasses store membership checks (admin bypass is enforced in the service layer).
var adminPrincipal = &domain.Principal{
	ID:   "webui-admin",
	Name: "WebUI Admin",
	Type: domain.PrincipalTypeHuman,
	Role: domain.InstanceRoleAdmin,
}

// Handler is the Admin WebUI adapter.
// It implements rest.RouteRegistrar and mounts /webui/* routes with its own
// middleware chain (localhost enforcement + session auth). JWT is NOT applied.
type Handler struct {
	token        string
	sessions     *SessionStore
	principalSvc principaluc.Service
	storeSvc     storeuc.Service
	backups      []backup.Backuper
}

// NewHandler creates a Handler.
// token is the startup token printed to stdout on boot; it never touches disk.
func NewHandler(
	token string,
	sessions *SessionStore,
	principalSvc principaluc.Service,
	storeSvc storeuc.Service,
	backups []backup.Backuper,
) *Handler {
	return &Handler{
		token:        token,
		sessions:     sessions,
		principalSvc: principalSvc,
		storeSvc:     storeSvc,
		backups:      backups,
	}
}

// Register mounts all /webui/* routes on r.
// LocalhostOnly is applied to every /webui route.
// SessionAuth is applied to all routes except /login and /logout.
func (h *Handler) Register(r chi.Router) {
	r.Route("/webui", func(r chi.Router) {
		r.Use(LocalhostOnly)

		// Serve embedded static assets (htmx.min.js, pico.min.css).
		staticSub, err := fs.Sub(staticFiles, "static")
		if err != nil {
			panic(err)
		}
		r.Handle("/static/*", http.StripPrefix("/webui/static", http.FileServer(http.FS(staticSub))))

		// Redirect /webui → /webui/dashboard.
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/webui/dashboard", http.StatusSeeOther)
		})

		// Unauthenticated routes: login / logout.
		r.Get("/login", h.loginGet)
		r.Post("/login", h.loginPost)
		r.Post("/logout", h.logout)

		// Authenticated routes (session required).
		r.Group(func(r chi.Router) {
			r.Use(SessionAuth(h.sessions))
			r.Get("/dashboard", h.dashboard)
			r.Get("/principals", h.principalsList)
			r.Get("/principals/new", h.principalsNew)
			r.Post("/principals", h.principalsCreate)
			r.Get("/principals/{id}", h.principalsDetail)
			r.Post("/principals/{id}/reissue-key", h.principalsReissueKey)
			r.Post("/principals/{id}/revoke-key", h.principalsRevokeKey)
			r.Get("/stores", h.storesList)
			r.Get("/stores/{name}", h.storesDetail)
			r.Post("/stores/{name}/delete", h.storesDelete)
			r.Get("/system", h.system)
			r.Post("/system/backup", h.systemBackup)
		})
	})
}

// loginGet renders the login page.
func (h *Handler) loginGet(w http.ResponseWriter, r *http.Request) {
	renderLogin(w, r, "")
}

// loginPost validates the startup token and creates a session on success.
func (h *Handler) loginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderLogin(w, r, "Invalid form submission.")
		return
	}
	// Constant-time comparison prevents timing-based token oracle attacks.
	if subtle.ConstantTimeCompare([]byte(r.FormValue("token")), []byte(h.token)) != 1 {
		renderLogin(w, r, "Incorrect token. Check the server startup log.")
		return
	}
	id := h.sessions.Create()
	setSessionCookie(w, id, sessionTTL)
	http.Redirect(w, r, "/webui/dashboard", http.StatusSeeOther)
}

// logout clears the session and redirects to the login page.
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		h.sessions.Delete(cookie.Value)
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/webui/login", http.StatusSeeOther)
}

// dashboard renders the admin dashboard with system overview.
func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	principals, _ := h.principalSvc.List(r.Context(), adminPrincipal)
	stores, _ := h.storeSvc.List(r.Context(), adminPrincipal)
	renderTempl(w, r, "Dashboard", templates.DashboardContent(len(principals), len(stores)))
}

// principalsList renders the paginated principal list.
func (h *Handler) principalsList(w http.ResponseWriter, r *http.Request) {
	principals, err := h.principalSvc.List(r.Context(), adminPrincipal)
	if err != nil {
		renderError(w, r, "Failed to load principals: "+err.Error())
		return
	}
	rows := make([]templates.PrincipalRow, 0, len(principals))
	for _, p := range principals {
		rows = append(rows, templates.PrincipalRow{
			ID:   p.ID,
			Name: p.Name,
			Type: string(p.Type),
			Role: string(p.Role),
		})
	}
	renderTempl(w, r, "Principals", templates.PrincipalsListContent(rows))
}

// principalsNew renders the create principal form.
func (h *Handler) principalsNew(w http.ResponseWriter, r *http.Request) {
	renderTempl(w, r, "New Principal", templates.PrincipalsNewContent())
}

// principalsCreate handles the POST to create a new principal.
func (h *Handler) principalsCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderError(w, r, "Invalid form submission.")
		return
	}
	req := principaluc.CreateRequest{
		Name:  r.FormValue("name"),
		Type:  domain.PrincipalType(r.FormValue("type")),
		Role:  domain.InstanceRole(r.FormValue("role")),
		Email: r.FormValue("email"),
	}
	resp, err := h.principalSvc.Create(r.Context(), adminPrincipal, req)
	if err != nil {
		renderError(w, r, "Failed to create principal: "+err.Error())
		return
	}
	// The API key is shown exactly once and is never retrievable again.
	renderTempl(w, r, "Principal Created", templates.PrincipalCreatedContent(
		resp.Principal.Name,
		resp.Principal.ID,
		string(resp.Principal.Type),
		string(resp.Principal.Role),
		resp.APIKey,
	))
}

// principalsDetail renders a single principal's detail page.
func (h *Handler) principalsDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Find the principal in the list (no single-Get on the service interface).
	all, err := h.principalSvc.List(r.Context(), adminPrincipal)
	if err != nil {
		renderError(w, r, "Failed to load principals.")
		return
	}
	var found *domain.Principal
	for _, p := range all {
		if p.ID == id {
			found = p
			break
		}
	}
	if found == nil {
		renderError(w, r, "Principal not found.")
		return
	}

	whoami, err := h.principalSvc.WhoAmI(r.Context(), found)
	if err != nil {
		renderError(w, r, "Failed to load principal detail.")
		return
	}

	memberRows := make([]templates.MembershipRow, 0, len(whoami.StoreMemberships))
	for _, m := range whoami.StoreMemberships {
		memberRows = append(memberRows, templates.MembershipRow{
			StoreName: m.StoreName,
			Role:      string(m.Role),
		})
	}

	detail := templates.PrincipalDetailData{
		ID:          found.ID,
		Name:        found.Name,
		Type:        string(found.Type),
		Role:        string(found.Role),
		Email:       found.Email,
		HasAPIKey:   found.APIKeyHash != "",
		CreatedAt:   found.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
		Memberships: memberRows,
	}
	renderTempl(w, r, found.Name, templates.PrincipalDetailContent(detail))
}

// principalsReissueKey re-issues the API key for a principal.
func (h *Handler) principalsReissueKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	newKey, err := h.principalSvc.ReissueAPIKey(r.Context(), adminPrincipal, id)
	if err != nil {
		renderError(w, r, "Failed to reissue API key: "+err.Error())
		return
	}
	renderTempl(w, r, "API Key Reissued", templates.KeyReissuedContent(id, newKey))
}

// principalsRevokeKey revokes the API key for a principal.
func (h *Handler) principalsRevokeKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.principalSvc.RevokeAPIKey(r.Context(), adminPrincipal, id); err != nil {
		renderError(w, r, "Failed to revoke API key: "+err.Error())
		return
	}
	http.Redirect(w, r, "/webui/principals/"+id, http.StatusSeeOther)
}

// storesList renders the store list.
func (h *Handler) storesList(w http.ResponseWriter, r *http.Request) {
	stores, err := h.storeSvc.List(r.Context(), adminPrincipal)
	if err != nil {
		renderError(w, r, "Failed to load stores: "+err.Error())
		return
	}
	rows := make([]templates.StoreRow, 0, len(stores))
	for _, s := range stores {
		rows = append(rows, templates.StoreRow{
			Name:        s.Name,
			OwnerID:     s.OwnerID,
			IsPersonal:  s.IsPersonal(),
			MemberCount: len(s.Members),
		})
	}
	renderTempl(w, r, "Stores", templates.StoresListContent(rows))
}

// storesDetail renders a single store's detail page.
func (h *Handler) storesDetail(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	s, err := h.storeSvc.Get(r.Context(), name, adminPrincipal)
	if err != nil {
		renderError(w, r, "Store not found.")
		return
	}
	members := make([]templates.MembershipRow, 0, len(s.Members))
	for _, m := range s.Members {
		members = append(members, templates.MembershipRow{
			StoreName: m.PrincipalID,
			Role:      string(m.Role),
		})
	}
	detail := templates.StoreDetailData{
		Name:           s.Name,
		OwnerID:        s.OwnerID,
		Description:    s.Description,
		EmbeddingModel: s.EmbeddingModel,
		CreatedAt:      s.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
		IsPersonal:     s.IsPersonal(),
		Members:        members,
	}
	renderTempl(w, r, s.Name, templates.StoreDetailContent(detail))
}

// storesDelete handles the POST to delete a store.
func (h *Handler) storesDelete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := r.ParseForm(); err != nil {
		renderError(w, r, "Invalid form submission.")
		return
	}
	confirm := r.FormValue("confirm")
	if err := h.storeSvc.Delete(r.Context(), name, confirm, adminPrincipal); err != nil {
		renderError(w, r, "Delete failed: "+err.Error())
		return
	}
	http.Redirect(w, r, "/webui/stores", http.StatusSeeOther)
}

// system renders the system information and backup page.
func (h *Handler) system(w http.ResponseWriter, r *http.Request) {
	names := make([]string, 0, len(h.backups))
	for _, b := range h.backups {
		names = append(names, b.Name())
	}
	renderTempl(w, r, "System", templates.SystemContent(names))
}

// systemBackup triggers the selected (or all) backup provider(s).
// Returns a plain HTML fragment suitable for HTMX OOB swap or direct rendering.
func (h *Handler) systemBackup(w http.ResponseWriter, r *http.Request) {
	providerFilter := r.URL.Query().Get("provider")
	results := ""
	for _, b := range h.backups {
		if providerFilter != "" && b.Name() != providerFilter {
			continue
		}
		if err := b.Backup(r.Context(), ""); err != nil {
			results += fmt.Sprintf(
				`<p>%s: <strong style="color:red">failed</strong> — %s</p>`,
				escapeHTML(b.Name()), escapeHTML(err.Error()))
		} else {
			results += fmt.Sprintf(
				`<p>%s: <strong style="color:green">success</strong></p>`,
				escapeHTML(b.Name()))
		}
	}
	if results == "" {
		results = "<p>No backup providers ran.</p>"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, results)
}

// setSessionCookie sets the admin session cookie on the response.
func setSessionCookie(w http.ResponseWriter, id string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/webui",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// clearSessionCookie removes the admin session cookie.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/webui",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// escapeHTML escapes characters that must not appear unescaped in HTML.
// Used for user-controlled strings in the systemBackup HTML fragment.
func escapeHTML(s string) string {
	var out []byte
	for i := range len(s) {
		switch s[i] {
		case '&':
			out = append(out, '&', 'a', 'm', 'p', ';')
		case '<':
			out = append(out, '&', 'l', 't', ';')
		case '>':
			out = append(out, '&', 'g', 't', ';')
		case '"':
			out = append(out, '&', 'q', 'u', 'o', 't', ';')
		case '\'':
			out = append(out, '&', '#', '3', '9', ';')
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}

// GenerateStartupToken generates a cryptographically random 64-character hex token.
// Called once at startup; the token is printed to stdout and never persisted.
func GenerateStartupToken() string {
	return generateHex(32)
}

// renderLogin renders the login page using the LoginPage templ component.
func renderLogin(w http.ResponseWriter, r *http.Request, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.LoginPage(errMsg).Render(r.Context(), w)
}

// renderTempl renders a full admin page using the Layout + a content component.
func renderTempl(w http.ResponseWriter, r *http.Request, title string, content templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = templates.Layout(title, content).Render(r.Context(), w)
}

// renderError renders an error message inside the page layout.
func renderError(w http.ResponseWriter, r *http.Request, msg string) {
	renderTempl(w, r, "Error", templates.ErrorContent(msg))
}

// Ensure Handler satisfies the chi.Router-registrar contract at compile time.
// rest.RouteRegistrar is defined in adapter/rest; we avoid importing it to
// keep the webui package dependency-free of the rest package.
// The Register method signature matches the interface.
var _ interface{ Register(chi.Router) } = (*Handler)(nil)
