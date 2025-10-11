Ingress & Load Balancers

ENV-first design keeps ingress/LB flexible and cloud-agnostic.

Environment

- `PAAS_DOMAIN`: Base domain for generated hosts, e.g., `apps.example.com`.
- `PAAS_WILDCARD_ENABLED`: When true, hostnames are generated as `{app}.{namespace}.{PAAS_DOMAIN}` if no `domain` is supplied in the request.
- `LB_DRIVER`: Pluggable LB driver (default `metallb`).
- `LB_METALLB_POOL`: Optional address-pool annotation for MetalLB.
- `MAX_LOADBALANCERS_PER_PROJECT`: Default cap for LoadBalancer Services per project (default 1). Can be overridden via project quota key `services.loadbalancers`.

Default: MetalLB

- The API sets `Service.spec.type=LoadBalancer` and, if `LB_METALLB_POOL` is set, adds annotation `metallb.universe.tf/address-pool=<pool>`.
- Other LB providers can be added later by extending the driver annotations.

Quota enforcement

- Deploy API counts existing `LoadBalancer` Services in the project namespace and denies requests exceeding the configured cap.

DNS Automation

- Recommended: Use a single Ingress controller with one LoadBalancer IP and point a wildcard DNS to it. Host routing handles per-app names.
- Built-in (optional): KubeOP can upsert A records for app hosts pointing to the Service External IP when available.
  - Set `EXTERNAL_DNS_PROVIDER=cloudflare` and provide `CF_API_TOKEN`, `CF_ZONE_ID`, or set `EXTERNAL_DNS_PROVIDER=powerdns` and provide `PDNS_API_URL`, `PDNS_API_KEY`, `PDNS_SERVER_ID`, `PDNS_ZONE`.
  - TTL by `EXTERNAL_DNS_TTL` (default 300).
  - This happens best-effort right after deployment when the LoadBalancer IP is present. If the IP is pending, DNS creation is skipped.
  - Alternative: Run `external-dns` in-cluster; annotate Ingress/Service per your provider to manage DNS records automatically.
