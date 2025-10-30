package auth

// Helpers to validate tenant/project scoped RBAC based on Claims.

func IsAdmin(c *Claims) bool { return c != nil && c.Role == "admin" }

// Scope format: "tenant:<id>" or "project:<id>"
func IsTenant(c *Claims, tenantID string) bool {
    if c == nil || c.Role != "tenant" { return false }
    return c.Scope == "tenant:"+tenantID
}

func IsProject(c *Claims, projectID string) bool {
    if c == nil || c.Role != "project" { return false }
    return c.Scope == "project:"+projectID
}

