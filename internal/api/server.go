package api

import (
    "context"
    "embed"
    "encoding/json"
    "errors"
    "log/slog"
    "net/http"
    "os"
    "strconv"
    "strings"
    "time"

    "github.com/vaheed/kubeop/internal/auth"
    "github.com/vaheed/kubeop/internal/db"
    "github.com/vaheed/kubeop/internal/kms"
    "github.com/vaheed/kubeop/internal/models"
    "github.com/vaheed/kubeop/internal/metrics"
    "github.com/vaheed/kubeop/internal/webhook"
    "github.com/vaheed/kubeop/internal/version"
)

//go:embed openapi.json
var openapiFS embed.FS

type Server struct {
    log    *slog.Logger
    db     *db.DB
    store  *models.Store
    kms    *kms.Envelope
    cfgAuth bool
    jwtKey []byte
    hooks *webhook.Client
}

func New(l *slog.Logger, d *db.DB, kmsEnc *kms.Envelope, requireAuth bool, jwtKey []byte) *Server {
    return &Server{log: l, db: d, store: models.NewStore(d.DB), kms: kmsEnc, cfgAuth: requireAuth, jwtKey: jwtKey, hooks: &webhook.Client{URL: os.Getenv("KUBEOP_HOOK_URL"), Secret: []byte(os.Getenv("KUBEOP_HOOK_SECRET"))}}
}

func (s *Server) Router() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
    mux.HandleFunc("/readyz", s.handleReady)
    mux.HandleFunc("/version", s.handleVersion)
    mux.HandleFunc("/openapi.json", s.handleOpenAPI)
    // metrics
    mux.Handle("/metrics", promHandler())

    mux.HandleFunc("/v1/tenants", s.tenantsCollection)
    mux.HandleFunc("/v1/tenants/", s.requireRoleOrTenantPath(s.tenantsGetDelete))
    mux.HandleFunc("/v1/projects", s.projectsCollection)
    mux.HandleFunc("/v1/projects/", s.requireRoleOrProjectPath(s.projectsGetDelete))
    mux.HandleFunc("/v1/apps", s.appsCollection)
    mux.HandleFunc("/v1/apps/", s.requireRoleOrProjectPath(s.appsGetDelete))
    mux.HandleFunc("/v1/usage/snapshot", s.requireRole("admin", s.usageSnapshot))
    mux.HandleFunc("/v1/usage/ingest", s.requireRole("admin", s.usageIngest))
    mux.HandleFunc("/v1/invoices/", s.requireRoleOrTenantPath(s.invoice))
    mux.HandleFunc("/v1/kubeconfigs/", s.requireRole("admin", s.kubeconfigIssue))
    mux.HandleFunc("/v1/kubeconfigs/project/", s.requireRoleOrProjectPath(s.kubeconfigProject))
    mux.HandleFunc("/v1/jwt/project", s.requireRoleOrTenant(s.jwtMintProject))
    return s.withJSON(s.withAccessLog(instrument(recoverer(mux))))
}

func (s *Server) withJSON(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        next.ServeHTTP(w, r)
    })
}

// recoverer ensures handler panics don't crash the server; returns 500 and logs minimal info
func recoverer(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}

// withAccessLog logs method, path, status code and duration for every request
func (s *Server) withAccessLog(next http.Handler) http.Handler {
    type swrap struct {
        http.ResponseWriter
        code int
    }
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sw := &swrap{ResponseWriter: w, code: 200}
        start := time.Now()
        // capture code
        defer func() {
            d := time.Since(start)
            s.log.Info("http", slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.Int("status", sw.code), slog.String("remote", r.RemoteAddr), slog.String("duration", d.String()))
        }()
        // implement WriteHeader capture
        if _, ok := w.(*swrap); !ok {
            // override WriteHeader on our wrapper
            w = sw
        }
        // shim for WriteHeader
        sw.ResponseWriter = w
        next.ServeHTTP(sw, r)
    })
}

func (s *Server) requireRole(role string, fn func(http.ResponseWriter, *http.Request, *auth.Claims)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var claims *auth.Claims
        if s.cfgAuth {
            a := r.Header.Get("Authorization")
            if !strings.HasPrefix(a, "Bearer ") {
                http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
                return
            }
            t := strings.TrimPrefix(a, "Bearer ")
            c, err := auth.VerifyHS256(t, s.jwtKey)
            if err != nil {
                http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
                return
            }
            if role != "" && c.Role != role {
                http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
                return
            }
            claims = c
        }
        fn(w, r, claims)
    }
}

// allow admin or tenant-scoped claim matching body. Expects JSON with tenantID.
func (s *Server) requireRoleOrTenant(fn func(http.ResponseWriter, *http.Request, *auth.Claims)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var claims *auth.Claims
        if s.cfgAuth {
            a := r.Header.Get("Authorization")
            if !strings.HasPrefix(a, "Bearer ") { http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized); return }
            t := strings.TrimPrefix(a, "Bearer ")
            c, err := auth.VerifyHS256(t, s.jwtKey)
            if err != nil { http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized); return }
            claims = c
        }
        fn(w, r, claims)
    }
}

// allow admin or project-scoped claim matching body. Expects JSON with projectID.
func (s *Server) requireRoleOrProject(fn func(http.ResponseWriter, *http.Request, *auth.Claims)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var claims *auth.Claims
        if s.cfgAuth {
            a := r.Header.Get("Authorization")
            if !strings.HasPrefix(a, "Bearer ") { http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized); return }
            t := strings.TrimPrefix(a, "Bearer ")
            c, err := auth.VerifyHS256(t, s.jwtKey)
            if err != nil { http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized); return }
            claims = c
        }
        fn(w, r, claims)
    }
}

// allow admin or tenant-scoped claim from path param /v1/invoices/{tenantID}
func (s *Server) requireRoleOrTenantPath(fn func(http.ResponseWriter, *http.Request, *auth.Claims)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var claims *auth.Claims
        if s.cfgAuth {
            a := r.Header.Get("Authorization")
            if !strings.HasPrefix(a, "Bearer ") { http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized); return }
            t := strings.TrimPrefix(a, "Bearer ")
            c, err := auth.VerifyHS256(t, s.jwtKey)
            if err != nil { http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized); return }
            claims = c
        }
        fn(w, r, claims)
    }
}

// allow admin or project-scoped claim from path param /v1/apps/{id} or /v1/projects/{id}
func (s *Server) requireRoleOrProjectPath(fn func(http.ResponseWriter, *http.Request, *auth.Claims)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var claims *auth.Claims
        if s.cfgAuth {
            a := r.Header.Get("Authorization")
            if !strings.HasPrefix(a, "Bearer ") { http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized); return }
            t := strings.TrimPrefix(a, "Bearer ")
            c, err := auth.VerifyHS256(t, s.jwtKey)
            if err != nil { http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized); return }
            claims = c
        }
        fn(w, r, claims)
    }
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
    type resp struct{
        Service string `json:"service"`
        Version string `json:"version"`
        Build string `json:"gitCommit"`
        BuildDate string `json:"buildDate"`
    }
    v := versionFull()
    json.NewEncoder(w).Encode(resp{Service: "manager", Version: v.Version, Build: v.Build, BuildDate: version.BuildDate})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.dbTimeoutMS())*time.Millisecond)
    defer cancel()
    if err := s.db.Ping(ctx); err != nil { http.Error(w, "db not ready", http.StatusServiceUnavailable); return }
    if s.kms == nil { http.Error(w, "kms not ready", http.StatusServiceUnavailable); return }
    w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
    b, _ := openapiFS.ReadFile("openapi.json")
    w.Header().Set("Content-Type", "application/json")
    w.Write(b)
}

func (s *Server) tenantsCreate(w http.ResponseWriter, r *http.Request, _ *auth.Claims) {
    if r.Method != http.MethodPost { http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed); return }
    var in struct{ Name string `json:"name"` }
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Name == "" {
        http.Error(w, `{"error":"invalid"}`, http.StatusBadRequest); return
    }
    ctx := r.Context()
    start := time.Now()
    t, err := s.store.CreateTenant(ctx, in.Name)
    metrics.ObserveDB("create_tenant", time.Since(start))
    if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
    _ = s.hooks.Send("tenant.created", t)
    metrics.IncCreated("tenant")
    json.NewEncoder(w).Encode(t)
}

func (s *Server) tenantsCollection(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodPost:
        s.requireRole("admin", s.tenantsCreate)(w, r)
        return
    case http.MethodGet:
        // list
        claims := (*auth.Claims)(nil)
        if s.cfgAuth {
            a := r.Header.Get("Authorization")
            if strings.HasPrefix(a, "Bearer ") { c, _ := auth.VerifyHS256(strings.TrimPrefix(a, "Bearer "), s.jwtKey); claims = c }
        }
        if s.cfgAuth && !auth.IsAdmin(claims) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        t0 := time.Now()
        list, err := s.store.ListTenants(r.Context())
        metrics.ObserveDB("list_tenants", time.Since(t0))
        if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        json.NewEncoder(w).Encode(list)
        return
    case http.MethodPut, http.MethodPatch:
        if s.cfgAuth { // admin only
            a := r.Header.Get("Authorization")
            if !strings.HasPrefix(a, "Bearer ") { http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized); return }
            c, err := auth.VerifyHS256(strings.TrimPrefix(a, "Bearer "), s.jwtKey)
            if err != nil || !auth.IsAdmin(c) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        }
        var in struct{ ID, Name string }
        if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ID == "" || in.Name == "" { http.Error(w, `{"error":"invalid"}`, http.StatusBadRequest); return }
        t0 := time.Now(); err := s.store.UpdateTenant(r.Context(), in.ID, in.Name); metrics.ObserveDB("update_tenant", time.Since(t0))
        if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        w.WriteHeader(http.StatusNoContent)
        return
    default:
        http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
    }
}

func (s *Server) projectsCreate(w http.ResponseWriter, r *http.Request, claims *auth.Claims) {
    if r.Method != http.MethodPost { http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed); return }
    var in struct{ TenantID, Name string }
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.TenantID == "" || in.Name == "" {
        http.Error(w, `{"error":"invalid"}`, http.StatusBadRequest); return
    }
    if s.cfgAuth && !(auth.IsAdmin(claims) || auth.IsTenant(claims, in.TenantID)) {
        http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return
    }
    start := time.Now()
    p, err := s.store.CreateProject(r.Context(), in.TenantID, in.Name)
    metrics.ObserveDB("create_project", time.Since(start))
    if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
    _ = s.hooks.Send("project.created", p)
    metrics.IncCreated("project")
    json.NewEncoder(w).Encode(p)
}

func (s *Server) projectsCollection(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodPost:
        s.requireRoleOrTenant(s.projectsCreate)(w, r)
        return
    case http.MethodGet:
        claims := (*auth.Claims)(nil)
        if s.cfgAuth {
            a := r.Header.Get("Authorization")
            if strings.HasPrefix(a, "Bearer ") { c, _ := auth.VerifyHS256(strings.TrimPrefix(a, "Bearer "), s.jwtKey); claims = c }
            if claims == nil { http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized); return }
        }
        tenantID := r.URL.Query().Get("tenantID")
        // tenant role can only list own tenant
        if s.cfgAuth && !auth.IsAdmin(claims) && !auth.IsTenant(claims, tenantID) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        t0 := time.Now(); list, err := s.store.ListProjects(r.Context(), tenantID); metrics.ObserveDB("list_projects", time.Since(t0))
        if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        json.NewEncoder(w).Encode(list)
        return
    case http.MethodPut, http.MethodPatch:
        var in struct{ ID, Name string }
        if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ID == "" || in.Name == "" { http.Error(w, `{"error":"invalid"}`, http.StatusBadRequest); return }
        if s.cfgAuth {
            a := r.Header.Get("Authorization")
            if !strings.HasPrefix(a, "Bearer ") { http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized); return }
            c, err := auth.VerifyHS256(strings.TrimPrefix(a, "Bearer "), s.jwtKey)
            if err != nil { http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized); return }
            // allow admin or project-scoped
            if !(auth.IsAdmin(c)) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        }
        t0 := time.Now(); err := s.store.UpdateProject(r.Context(), in.ID, in.Name); metrics.ObserveDB("update_project", time.Since(t0))
        if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        w.WriteHeader(http.StatusNoContent)
        return
    default:
        http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
    }
}

func (s *Server) appsCreate(w http.ResponseWriter, r *http.Request, claims *auth.Claims) {
    if r.Method != http.MethodPost { http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed); return }
    var in struct{ ProjectID, Name, Image, Host string }
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ProjectID == "" || in.Name == "" {
        http.Error(w, `{"error":"invalid"}`, http.StatusBadRequest); return
    }
    if s.cfgAuth && !(auth.IsAdmin(claims) || auth.IsProject(claims, in.ProjectID)) {
        http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return
    }
    start := time.Now()
    a, err := s.store.CreateApp(r.Context(), in.ProjectID, in.Name, in.Image, in.Host)
    metrics.ObserveDB("create_app", time.Since(start))
    if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
    _ = s.hooks.Send("app.created", a)
    metrics.IncCreated("app")
    json.NewEncoder(w).Encode(a)
}

func (s *Server) appsCollection(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodPost:
        s.requireRoleOrProject(s.appsCreate)(w, r)
        return
    case http.MethodGet:
        claims := (*auth.Claims)(nil)
        if s.cfgAuth {
            a := r.Header.Get("Authorization")
            if strings.HasPrefix(a, "Bearer ") { c, _ := auth.VerifyHS256(strings.TrimPrefix(a, "Bearer "), s.jwtKey); claims = c }
            if claims == nil { http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized); return }
        }
        projectID := r.URL.Query().Get("projectID")
        if s.cfgAuth && !(auth.IsAdmin(claims) || auth.IsProject(claims, projectID)) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        t0 := time.Now(); list, err := s.store.ListApps(r.Context(), projectID); metrics.ObserveDB("list_apps", time.Since(t0))
        if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        json.NewEncoder(w).Encode(list)
        return
    case http.MethodPut, http.MethodPatch:
        var in struct{ ID, Name, Image, Host string }
        if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ID == "" || in.Name == "" { http.Error(w, `{"error":"invalid"}`, http.StatusBadRequest); return }
        if s.cfgAuth {
            a := r.Header.Get("Authorization")
            if !strings.HasPrefix(a, "Bearer ") { http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized); return }
            c, err := auth.VerifyHS256(strings.TrimPrefix(a, "Bearer "), s.jwtKey)
            if err != nil { http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized); return }
            if !(auth.IsAdmin(c)) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        }
        t0 := time.Now(); err := s.store.UpdateApp(r.Context(), in.ID, in.Name, in.Image, in.Host); metrics.ObserveDB("update_app", time.Since(t0))
        if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        w.WriteHeader(http.StatusNoContent)
        return
    default:
        http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
    }
}

func (s *Server) usageSnapshot(w http.ResponseWriter, r *http.Request, _ *auth.Claims) {
    // context timeout for DB
    ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.dbTimeoutMS())*time.Millisecond)
    defer cancel()
    totals, err := s.store.Totals(ctx)
    if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
    json.NewEncoder(w).Encode(totals)
}

func (s *Server) invoice(w http.ResponseWriter, r *http.Request, claims *auth.Claims) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/invoices/"), "/")
    if len(parts) < 1 || parts[0] == "" { http.Error(w, `{"error":"tenant required"}`, http.StatusBadRequest); return }
    tenantID := parts[0]
    if s.cfgAuth && !(auth.IsAdmin(claims) || auth.IsTenant(claims, tenantID)) {
        http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return
    }
    now := time.Now().UTC()
    start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
    end := start.AddDate(0, 1, 0)
    t0 := time.Now()
    ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.dbTimeoutMS())*time.Millisecond)
    defer cancel()
    lines, err := s.store.Invoice(ctx, tenantID, start, end)
    metrics.ObserveDB("invoice_lines", time.Since(t0))
    if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
    // compute subtotal using simple rates (per milliCPU-hour and per MiB-hour)
    rateCPU := rateFromEnv("KUBEOP_RATE_CPU_MILLI", 0.000001)
    rateMem := rateFromEnv("KUBEOP_RATE_MEM_MIB", 0.0000002)
    if tr, _ := s.store.GetTenantRate(r.Context(), tenantID); tr != nil {
        if tr.CPUmRate > 0 { rateCPU = tr.CPUmRate }
        if tr.MemMiBRate > 0 { rateMem = tr.MemMiBRate }
    }
    var subtotal float64
    for _, l := range lines { subtotal += float64(l.CPUm)*rateCPU + float64(l.MemMiB)*rateMem }
    metrics.AddInvoiceLines(len(lines))
    out := struct {
        TenantID string               `json:"tenant_id"`
        Start    time.Time            `json:"start"`
        End      time.Time            `json:"end"`
        Lines    []models.UsageLine   `json:"lines"`
        Subtotal float64             `json:"subtotal"`
    }{tenantID, start, end, lines, subtotal}
    json.NewEncoder(w).Encode(out)
}

func (s *Server) tenantsGetDelete(w http.ResponseWriter, r *http.Request, claims *auth.Claims) {
    id := strings.TrimPrefix(r.URL.Path, "/v1/tenants/")
    if id == "" { http.Error(w, `{"error":"id"}`, http.StatusBadRequest); return }
    switch r.Method {
    case http.MethodGet:
        if s.cfgAuth && !(auth.IsAdmin(claims) || auth.IsTenant(claims, id)) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        t0 := time.Now()
        t, err := s.store.GetTenant(r.Context(), id)
        metrics.ObserveDB("get_tenant", time.Since(t0))
        if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        if t == nil { http.Error(w, `{"error":"not found"}`, http.StatusNotFound); return }
        json.NewEncoder(w).Encode(t)
    case http.MethodDelete:
        if s.cfgAuth && !auth.IsAdmin(claims) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        t0 := time.Now()
        if err := s.store.DeleteTenant(r.Context(), id); err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        metrics.ObserveDB("delete_tenant", time.Since(t0))
        w.WriteHeader(http.StatusNoContent)
    default:
        http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
    }
}

func (s *Server) projectsGetDelete(w http.ResponseWriter, r *http.Request, claims *auth.Claims) {
    id := strings.TrimPrefix(r.URL.Path, "/v1/projects/")
    if id == "" { http.Error(w, `{"error":"id"}`, http.StatusBadRequest); return }
    switch r.Method {
    case http.MethodGet:
        t0 := time.Now()
        p, err := s.store.GetProject(r.Context(), id)
        metrics.ObserveDB("get_project", time.Since(t0))
        if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        if p == nil { http.Error(w, `{"error":"not found"}`, http.StatusNotFound); return }
        if s.cfgAuth && !(auth.IsAdmin(claims) || auth.IsTenant(claims, p.TenantID) || auth.IsProject(claims, p.ID)) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        json.NewEncoder(w).Encode(p)
    case http.MethodDelete:
        if s.cfgAuth && !auth.IsAdmin(claims) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        t0 := time.Now()
        if err := s.store.DeleteProject(r.Context(), id); err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        metrics.ObserveDB("delete_project", time.Since(t0))
        w.WriteHeader(http.StatusNoContent)
    default:
        http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
    }
}

func (s *Server) appsGetDelete(w http.ResponseWriter, r *http.Request, claims *auth.Claims) {
    id := strings.TrimPrefix(r.URL.Path, "/v1/apps/")
    if id == "" { http.Error(w, `{"error":"id"}`, http.StatusBadRequest); return }
    switch r.Method {
    case http.MethodGet:
        t0 := time.Now()
        a, err := s.store.GetApp(r.Context(), id)
        metrics.ObserveDB("get_app", time.Since(t0))
        if err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        if a == nil { http.Error(w, `{"error":"not found"}`, http.StatusNotFound); return }
        if s.cfgAuth && !(auth.IsAdmin(claims) || auth.IsProject(claims, a.ProjectID)) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        json.NewEncoder(w).Encode(a)
    case http.MethodDelete:
        if s.cfgAuth && !auth.IsAdmin(claims) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        t0 := time.Now()
        if err := s.store.DeleteApp(r.Context(), id); err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        metrics.ObserveDB("delete_app", time.Since(t0))
        w.WriteHeader(http.StatusNoContent)
    default:
        http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
    }
}

func (s *Server) usageIngest(w http.ResponseWriter, r *http.Request, claims *auth.Claims) {
    if r.Method != http.MethodPost { http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed); return }
    var items []models.UsageLine
    if err := json.NewDecoder(r.Body).Decode(&items); err != nil { http.Error(w, `{"error":"invalid"}`, http.StatusBadRequest); return }
    for _, it := range items {
        if it.TS.IsZero() { http.Error(w, `{"error":"ts required"}`, http.StatusBadRequest); return }
        if s.cfgAuth && !(auth.IsAdmin(claims) || auth.IsTenant(claims, it.TenantID)) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
        t0 := time.Now()
        if err := s.store.AddUsageHour(r.Context(), it.TS, it.TenantID, it.CPUm, it.MemMiB); err != nil { http.Error(w, `{"error":"db"}`, http.StatusInternalServerError); return }
        metrics.ObserveDB("usage_ingest", time.Since(t0))
    }
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) kubeconfigIssue(w http.ResponseWriter, r *http.Request, _ *auth.Claims) {
    // Return simple kubeconfig content base64: Provide namespace from path after prefix
    ns := strings.TrimPrefix(r.URL.Path, "/v1/kubeconfigs/")
    if ns == "" { ns = "default" }
    cfg := `apiVersion: v1
clusters:
- cluster:
    server: https://kubernetes.default.svc
  name: kind-kubeop
contexts:
- context:
    cluster: kind-kubeop
    namespace: ` + ns + `
    user: token-user
  name: ` + ns + `
current-context: ` + ns + `
kind: Config
preferences: {}
users:
- name: token-user
  user:
    token: placeholder
`
    json.NewEncoder(w).Encode(map[string]string{"kubeconfig": cfg})
}

// Mint a project-scoped JWT and return kubeconfig for that project
func (s *Server) jwtMintProject(w http.ResponseWriter, r *http.Request, claims *auth.Claims) {
    if r.Method != http.MethodPost { http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed); return }
    var in struct{ ProjectID string; TTLMinutes int }
    if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ProjectID == "" { http.Error(w, `{"error":"invalid"}`, http.StatusBadRequest); return }
    if s.cfgAuth && !(auth.IsAdmin(claims)) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
    ttl := in.TTLMinutes
    if ttl <= 0 { ttl = 60 }
    now := time.Now().UTC()
    c := &auth.Claims{Iss: "kubeop", Sub: "project:" + in.ProjectID, Role: "project", Scope: "project:" + in.ProjectID, Iat: now.Unix(), Exp: now.Add(time.Duration(ttl) * time.Minute).Unix()}
    tok, err := auth.SignHS256(c, s.jwtKey)
    if err != nil { http.Error(w, `{"error":"sign"}`, http.StatusInternalServerError); return }
    json.NewEncoder(w).Encode(map[string]string{"token": tok})
}

// Issue kubeconfig for project by id, includes namespace and token placeholder
func (s *Server) kubeconfigProject(w http.ResponseWriter, r *http.Request, claims *auth.Claims) {
    pid := strings.TrimPrefix(r.URL.Path, "/v1/kubeconfigs/project/")
    if pid == "" { http.Error(w, `{"error":"project id"}`, http.StatusBadRequest); return }
    if s.cfgAuth && !(auth.IsAdmin(claims) || auth.IsProject(claims, pid)) { http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden); return }
    p, err := s.store.GetProject(r.Context(), pid)
    if err != nil || p == nil { http.Error(w, `{"error":"not found"}`, http.StatusNotFound); return }
    t, err := s.store.GetTenant(r.Context(), p.TenantID)
    if err != nil || t == nil { http.Error(w, `{"error":"not found"}`, http.StatusNotFound); return }
    ns := "kubeop-" + strings.ToLower(t.Name) + "-" + strings.ToLower(p.Name)
    cfg := `apiVersion: v1
clusters:
- cluster:
    server: https://kubernetes.default.svc
  name: kind-kubeop
contexts:
- context:
    cluster: kind-kubeop
    namespace: ` + ns + `
    user: token-user
  name: ` + ns + `
current-context: ` + ns + `
kind: Config
preferences: {}
users:
- name: token-user
  user:
    token: ` + r.Header.Get("Authorization") + `
`
    json.NewEncoder(w).Encode(map[string]string{"kubeconfig": cfg})
}

func (s *Server) Start(ctx context.Context, addr string) error {
    srv := &http.Server{
        Addr:              addr,
        Handler:           s.Router(),
        ReadHeaderTimeout: 5 * time.Second,
        ReadTimeout:       10 * time.Second,
        WriteTimeout:      15 * time.Second,
        IdleTimeout:       60 * time.Second,
        MaxHeaderBytes:    1 << 20,
    }
    go func() {
        <-ctx.Done()
        _ = srv.Shutdown(context.Background())
    }()
    return srv.ListenAndServe()
}

// Helper for tests
func (s *Server) MustMigrate(ctx context.Context) {
    if err := s.db.Migrate(ctx); err != nil {
        panic(err)
    }
}

// Errors
var ErrUnauthorized = errors.New("unauthorized")

// support functions
func promHandler() http.Handler { return promhttpHandler }
func rateFromEnv(key string, def float64) float64 {
    v := os.Getenv(key)
    if v == "" { return def }
    // simple parse, ignore error
    f, err := strconv.ParseFloat(v, 64)
    if err != nil { return def }
    return f
}

func (s *Server) dbTimeoutMS() int { return getenvInt("KUBEOP_DB_TIMEOUT_MS", 2000) }
func getenvInt(k string, def int) int {
    v := os.Getenv(k)
    if v == "" { return def }
    n, _ := strconv.Atoi(v)
    return n
}

// versionFull fetches version info from internal/version
func versionFull() struct{ Version, Build string } {
    return struct{ Version, Build string }{Version: version.Version, Build: version.Build}
}
