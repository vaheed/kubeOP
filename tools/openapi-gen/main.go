package main

import (
    "encoding/json"
    "fmt"
    "os"
    "regexp"
    "strings"
)

// Simple OpenAPI generator that scans internal/api/server.go Router() registrations
// and emits a minimal spec with paths and verbs. This is intentionally simple
// and meant as a stop-gap until a full OpenAPI generator is introduced.

func main() {
    b, err := os.ReadFile("internal/api/server.go")
    if err != nil { panic(err) }
    lines := strings.Split(string(b), "\n")
    pathVerbs := map[string][]string{}
    re := regexp.MustCompile(`mux\.HandleFunc\("([^"]+)",\s*s\.[\w]+`) // path
    for _, ln := range lines {
        ln = strings.TrimSpace(ln)
        m := re.FindStringSubmatch(ln)
        if len(m) == 2 {
            p := m[1]
            if _, ok := pathVerbs[p]; !ok { pathVerbs[p] = []string{} }
        }
    }
    // Build a tiny OpenAPI structure
    spec := map[string]any{
        "openapi": "3.0.0",
        "info": map[string]any{"title": "kubeOP API", "version": "v1"},
        "paths": map[string]any{},
    }
    paths := spec["paths"].(map[string]any)
    for p := range pathVerbs {
        // We don't know verbs here; include get/post as generic placeholders
        paths[p] = map[string]any{
            "get":  map[string]any{"responses": map[string]any{"200": map[string]any{"description": "ok"}}},
            "post": map[string]any{"responses": map[string]any{"200": map[string]any{"description": "ok"}}},
        }
    }
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    if err := enc.Encode(spec); err != nil { panic(err) }
    fmt.Fprintln(os.Stderr, "openapi: generated from server.go")
}

