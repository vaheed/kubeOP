# Advanced delivery scenarios

This reference expands on the minimal walkthrough by covering Git-driven deployments, template registration, and OCI manifest bundles. Each scenario highlights the new delivery metadata and SBOM capture mechanisms.

## Register a template

```bash
curl -s -XPOST "$API/v1/templates" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{
        "name": "web-template",
        "kind": "deployment",
        "description": "Opinionated web workload",
        "schema": {
          "$schema": "http://json-schema.org/draft-07/schema#",
          "type": "object",
          "properties": {"image": {"type": "string"}},
          "required": ["image"]
        },
        "defaults": {"replicas": 2},
        "deliveryTemplate": "{\n  \"image\": \"{{ .values.image }}\",\n  \"ports\": [{\"containerPort\": 8080,\"servicePort\": 80}]\n}"
      }' | jq
```

The response includes the template ID used when instantiating apps from the catalog.

## Deploy from Git

```bash
curl -s -XPOST "$API/v1/projects/$PROJECT_ID/apps" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{
        "name": "git-web",
        "git": {
          "url": "git@github.com:example/platform-configs.git",
          "ref": "main",
          "path": "overlays/prod",
          "mode": "kustomize"
        }
      }' | jq
```

After the reconcile completes, inspect `/delivery` to confirm the commit hash, credential usage, and SBOM contents.

```bash
curl -s "$API/v1/projects/$PROJECT_ID/apps/$APP_ID/delivery" -H "$AUTH" | jq '.sbom'
```

The SBOM lists the hashed YAML documents produced by Kustomize, enabling tamper detection and audit trails across releases.

> **Security guardrails**
>
> Git checkouts only allow normalised relative paths. Inputs containing
> absolute prefixes, parent segments, symlinks, or invalid characters are
> rejected so rendered manifests always come from within the cloned repository.

## Deploy from an OCI manifest bundle

```bash
curl -s -XPOST "$API/v1/projects/$PROJECT_ID/apps" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{
        "name": "bundle",
        "ociBundle": {
          "ref": "oci://registry.example.com/overlays/web:1.2.0"
        }
      }' | jq
```

Use validation to preview future releases and diff SBOM digests:

```bash
curl -s "$API/v1/apps/validate" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{
        "projectId": "'$PROJECT_ID'",
        "name": "bundle",
        "ociBundle": {"ref": "oci://registry.example.com/overlays/web:1.3.0"}
      }' | jq '.sbom.aggregateDigest'
```

The aggregate digest changes whenever manifest content shifts, regardless of ordering, making it safe to pin approvals to the SBOM fingerprint.

## Canonical tenancy labels

Whether the manifests originate from Helm, raw YAML, Git overlays, or OCI bundles, kubeOP now injects the same tenancy metadata on every object it applies. Expect to see `kubeop.cluster.id`, `kubeop.project.id`, `kubeop.project.name`, `kubeop.app.id`, `kubeop.app.name`, and `kubeop.tenant.id` along with the historical `kubeop.app-id` label. These fields make it trivial to write admission policies, audit queries, or namespace reports that span delivery mechanisms.

## Clean up

```bash
curl -s -XDELETE "$API/v1/projects/$PROJECT_ID/apps/$APP_ID" -H "$AUTH"
```

Deleting the app removes the associated template bindings and delivery metadata.
