package auth

import "testing"

func TestRBACScopes(t *testing.T) {
    c := &Claims{Role: "admin"}
    if !IsAdmin(c) { t.Fatal("admin expected") }
    c = &Claims{Role: "tenant", Scope: "tenant:abc"}
    if !IsTenant(c, "abc") { t.Fatal("tenant scope mismatch") }
    if IsTenant(c, "def") { t.Fatal("unexpected tenant allow") }
    c = &Claims{Role: "project", Scope: "project:p1"}
    if !IsProject(c, "p1") { t.Fatal("project scope mismatch") }
}

