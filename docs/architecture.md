# Architecture

- Manager API (Postgres, KMS, JWT/RBAC)
- Operator (controller-runtime) for Tenant/Project/App/DNS/Certificate
- Admission (validation/mutation) with baseline Pod Security and policy
- E2E harness (Kind) + mocks for DNS/ACME
