package main

import (
    "bytes"
    "fmt"
    "go/ast"
    "go/parser"
    "go/token"
    "io/fs"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strings"
)

func main() {
    must(os.RemoveAll("docs"))
    must(os.MkdirAll("docs/.vitepress", 0o755))
    must(os.MkdirAll("examples/tenant-project-app", 0o755))

    // Generate VitePress config
    write("docs/.vitepress/config.ts", vitepressConfig())
    // Discover flags and envs
    flags := discoverFlags("cmd")
    envs := discoverEnvs()
    endpoints := discoverAPIEndpoints("internal/api/server.go")
    crdTypes := parseCRDTypes("internal/operator/apis/paas/v1alpha1/types.go")
    controllers := parseControllers("internal/operator/controllers/controllers.go")

    // README
    readme := renderREADME()
    write("README.md", readme)

    // Docs pages
    write("docs/getting-started.md", renderGettingStarted())
    write("docs/architecture.md", renderArchitecture())
    write("docs/crds.md", renderCRDs(crdTypes))
    write("docs/controllers.md", renderControllers(controllers))
    write("docs/config.md", renderConfig(flags, envs))
    if len(endpoints) > 0 {
        write("docs/api.md", renderAPI(endpoints))
    }
    write("docs/operations.md", renderOperations())
    write("docs/security.md", renderSecurity())
    write("docs/troubleshooting.md", renderTroubleshooting())
    write("docs/contributing.md", renderContrib())
    write("DOCS.md", "Run: cd docs && npm install && npx vitepress dev .\n")

    // Examples – pull from e2e manifest fragments in tests
    example := exampleManifests()
    write("examples/tenant-project-app/manifests.yaml", example)
}

func must(err error) { if err != nil { panic(err) } }
func write(path, s string) { must(os.MkdirAll(filepath.Dir(path), 0o755)); must(os.WriteFile(path, []byte(strings.TrimSpace(s)+"\n"), 0o644)) }

func vitepressConfig() string { return `import { defineConfig } from 'vitepress'
export default defineConfig({ title: 'kubeOP', description: 'Code-sourced docs', base: '/', themeConfig: { sidebar: [ { text: 'Getting Started', link: '/getting-started' }, { text: 'Architecture', link: '/architecture' }, { text: 'CRDs', link: '/crds' }, { text: 'Controllers', link: '/controllers' }, { text: 'Config', link: '/config' }, { text: 'API', link: '/api' }, { text: 'Operations', link: '/operations' }, { text: 'Security', link: '/security' }, { text: 'Troubleshooting', link: '/troubleshooting' }, { text: 'Contributing', link: '/contributing' } ] } })` }

func renderREADME() string { return `# kubeOP

Operator-powered multi-tenant platform for Kubernetes.

- Quickstart: see docs/getting-started.md
- CRDs and controllers: see docs/crds.md and docs/controllers.md
- Configuration (env + flags): docs/config.md
- API: docs/api.md
` }

func renderGettingStarted() string {
    return "# Getting Started\n\n" +
        "Prerequisites: Kubernetes 1.26+, kubectl, Helm 3, Docker (for Kind/Compose).\n\n" +
        "Local (Kind + Compose):\n\n" +
        "- Create cluster: make kind-up\n" +
        "- Bootstrap operator/admission: bash e2e/bootstrap.sh\n" +
        "- Start Manager + Postgres: docker compose up -d db manager\n" +
        "- Verify: curl -sf localhost:18080/healthz\n"
}

func renderArchitecture() string { return `# Architecture

- Manager API (Postgres, KMS, JWT/RBAC)
- Operator (controller-runtime) for Tenant/Project/App/DNS/Certificate
- Admission (validation/mutation) with baseline Pod Security and policy
- E2E harness (Kind) + mocks for DNS/ACME
` }

func renderCRDs(m map[string][]string) string {
    var b strings.Builder
    b.WriteString("# CRDs\n\n")
    keys := make([]string,0,len(m)); for k := range m { keys = append(keys,k) }
    sort.Strings(keys)
    for _, k := range keys {
        b.WriteString("## "+k+"\n")
        for _, f := range m[k] { b.WriteString("- "+f+"\n") }
        b.WriteString("\n")
    }
    return b.String()
}

func renderControllers(ctrls []string) string {
    return "# Controllers\n\n"+strings.Join(ctrls, "\n")+"\n"
}

func renderConfig(flags []string, envs []string) string {
    var b strings.Builder
    b.WriteString("# Configuration\n\n## CLI Flags\n\n")
    for _, f := range flags { b.WriteString("- "+f+"\n") }
    b.WriteString("\n## Environment Variables\n\n")
    for _, e := range envs { b.WriteString("- "+e+"\n") }
    return b.String()
}

func renderAPI(endpoints []string) string {
    return "# API\n\n"+strings.Join(endpoints, "\n")+"\n"
}

func renderOperations() string { return `# Operations

- Health: /healthz, Ready: /readyz, Version: /version, Metrics: /metrics
- Logs: see Kubernetes pod logs and Manager logs (docker compose)
- Artifacts: CI uploads Kind cluster resources and logs
` }

func renderSecurity() string { return `# Security

- Admission enforces image allowlist, cross-tenant guards, quotas, egress baseline
- Baseline Pod Security: no privilege escalation, non-root, read-only root FS
- Ingress isolation via NetworkPolicy; egress baseline via policy
` }

func renderTroubleshooting() string { return `# Troubleshooting

- Operator not ready: kubectl -n kubeop-system logs deploy/kubeop-operator
- Admission TLS/CABundle: check ValidatingWebhookConfiguration
- Manager DB: docker compose ps; verify KUBEOP_DB_URL
` }

func renderContrib() string { return `# Contributing

- Repo layout: cmd/, internal/, deploy/, charts/
- Dev: Go 1.24+, make kind-up, e2e/bootstrap.sh, docker compose up -d db manager
- Tests: go test ./..., make test-e2e
` }

func exampleManifests() string {
    // Use the same shape as e2e test resources
    return `apiVersion: paas.kubeop.io/v1alpha1
kind: Tenant
metadata:
  name: acme
spec:
  name: acme
---
apiVersion: paas.kubeop.io/v1alpha1
kind: Project
metadata:
  name: web
spec:
  tenantRef: acme
  name: web
---
apiVersion: paas.kubeop.io/v1alpha1
kind: App
metadata:
  name: web
  namespace: kubeop-acme-web
spec:
  type: Image
  image: docker.io/library/nginx:1.25
`
}

func discoverFlags(root string) []string {
    var out []string
    re := regexp.MustCompile(`flag\.(StringVar|BoolVar|IntVar)\(&[\w]+,\s*"([^"]+)",\s*([^,]+),\s*"([^"]*)"\)`) // name default help
    filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() { return nil }
        if !strings.HasSuffix(path, ".go") { return nil }
        b, _ := os.ReadFile(path)
        for _, m := range re.FindAllStringSubmatch(string(b), -1) {
            out = append(out, fmt.Sprintf("%s (default %s) — %s", m[2], strings.TrimSpace(m[3]), m[4]))
        }
        return nil
    })
    sort.Strings(out)
    return out
}

func discoverEnvs() []string {
    set := map[string]struct{}{}
    add := func(k string){ if strings.HasPrefix(k, "KUBEOP_") || strings.HasSuffix(k, "_HTTP_ADDR") || strings.HasSuffix(k, "_MOCK_URL") { set[k]=struct{}{} } }
    // env.example lines
    if b, err := os.ReadFile("env.example"); err == nil {
        for _, ln := range strings.Split(string(b), "\n") {
            ln = strings.TrimSpace(ln)
            if ln == "" || strings.HasPrefix(ln, "#") { continue }
            parts := strings.SplitN(ln, "=", 2)
            if len(parts) > 0 { add(parts[0]) }
        }
    }
    // grep os.Getenv patterns
    re := regexp.MustCompile(`os\.Getenv\("([A-Z0-9_]+)"\)`) 
    filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") { return nil }
        b, _ := os.ReadFile(path)
        for _, m := range re.FindAllStringSubmatch(string(b), -1) { add(m[1]) }
        return nil
    })
    var out []string
    for k := range set { out = append(out, k) }
    sort.Strings(out)
    return out
}

func discoverAPIEndpoints(path string) []string {
    b, err := os.ReadFile(path); if err != nil { return nil }
    var out []string
    for _, ln := range strings.Split(string(b), "\n") {
        ln = strings.TrimSpace(ln)
        if strings.HasPrefix(ln, "mux.HandleFunc(") || strings.HasPrefix(ln, "mux.Handle(\"") {
            out = append(out, "- "+ln)
        }
    }
    return out
}

func parseCRDTypes(path string) map[string][]string {
    fset := token.NewFileSet()
    f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
    if err != nil { return map[string][]string{} }
    out := map[string][]string{}
    ast.Inspect(f, func(n ast.Node) bool {
        ts, ok := n.(*ast.TypeSpec)
        if !ok { return true }
        st, ok := ts.Type.(*ast.StructType)
        if !ok { return true }
        name := ts.Name.Name
        if strings.HasSuffix(name, "Spec") || strings.HasSuffix(name, "Status") || name=="App" || name=="Tenant" || name=="Project" || name=="DNSRecord" || name=="Certificate" {
            var fields []string
            if st.Fields != nil {
                for _, f := range st.Fields.List {
                    var buf bytes.Buffer
                    for i, id := range f.Names { if i>0 { buf.WriteString(",") }; buf.WriteString(id.Name) }
                    if f.Tag != nil { buf.WriteString(" "+f.Tag.Value) }
                    fields = append(fields, strings.TrimSpace(buf.String()))
                }
            }
            out[name] = fields
        }
        return true
    })
    return out
}

func parseControllers(path string) []string {
    b, err := os.ReadFile(path); if err != nil { return nil }
    var out []string
    for _, ln := range strings.Split(string(b), "\n") {
        if strings.Contains(ln, "SetupWithManager") || strings.Contains(ln, "Reconcile(") {
            out = append(out, "- "+strings.TrimSpace(ln))
        }
    }
    return out
}
