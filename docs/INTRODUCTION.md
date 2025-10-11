Introduction

What is KubeOP

- KubeOP is an out-of-cluster control plane written in Go.
- It manages multiple Kubernetes clusters by storing their kubeconfigs (encrypted) and exposing a REST API to provision users and projects.
- It focuses on simple, namespace-scoped multi-tenancy with two modes:
  - Shared user namespace (default): one namespace per user; all that user’s projects live in it.
  - Per-project namespaces (optional): one namespace per project; each project gets its own kubeconfig.

Why KubeOP

- Centralize cluster access: store clusters once, mint namespace-scoped kubeconfigs for users/projects consistently.
- Safe defaults: opinionated quotas, Pod Security Admission labels, and optional NetworkPolicies for isolation.
- Clear API: bootstrap users, create projects, manage quotas/suspension, check status, and list clusters.

Core Capabilities

- Register clusters via `kubeconfig_b64`; credentials are encrypted at rest.
- Bootstrap users to create a namespace and receive a user-scoped kubeconfig (shared mode).
- Create projects and optionally receive a project-scoped kubeconfig (per-project mode).
- Apply defaults (ResourceQuota, LimitRange) and, in per-project mode, create SA/RBAC + NetworkPolicies.
- Simple health, readiness, and version endpoints suitable for production checks.

Quickstart

- Read the API quickstart: API_REFERENCE.md:1
- Configure environment: ENVIRONMENT.md:1
- Operational notes and running locally/with Docker: OPERATIONS.md:1

