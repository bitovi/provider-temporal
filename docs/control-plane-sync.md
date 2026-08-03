# Where sync / Temporal control plane lives

This provider does **not** call the Temporal Cloud Ops API from hand-written Go.
Reconciliation is the usual **Upjet → Terraform CLI → `temporalio/temporalcloud`
provider** stack. Temporal only sees gRPC from that Terraform plugin.

## Short answer: did this come from Upjet?

**Mostly yes.** The scaffold and almost all sync machinery are from:

1. [crossplane/upjet-provider-template](https://github.com/crossplane/upjet-provider-template) — repo layout, `cmd/provider`, generator, package packaging.
2. [Upjet](https://github.com/crossplane/upjet) — shared controller that runs Terraform (`Observe` / `Create` / `Update` / `Delete`).
3. `make generate` — CRDs, types under `apis/`, and controllers under `internal/controller/**/zz_*.go` from the Terraform provider schema.

What is **Bitovi-owned** (not wholesale generated):

| Path | Role |
| --- | --- |
| `config/external_name.go` | which TF resources exist; provisional external-name / id seeding for Namespace |
| `config/provider.go` | root group (`*.bitovi.com`), include list, resource configurators |
| `config/{cluster,namespaced}/namespace/` | per-resource configure hooks (if any) |
| `internal/clients/temporalcloud.go` | Secret → Terraform `api_key` setup |
| examples, README, publish workflow, package meta branding | product packaging |

There is **no** second control path next to TF for “sync to Temporal” in-process.

## Data path (create / update / delete)

```text
kubectl Namespace CR
        │
        ▼
controller-runtime watch
  internal/controller/.../namespace/zz_controller.go   (generated)
        │
        ▼
crossplane managed.Reconciler
        │
        ▼
upjet tjcontroller.NewConnector + external client
  github.com/crossplane/upjet/v2/pkg/controller
        │
        ├── SetupFn: internal/clients.TerraformSetupBuilder
        │     reads ProviderConfig credentials
        │     → terraform.Setup.Configuration{"api_key": ...}
        │
        └── WorkspaceStore (local dir per MR UID)
              terraform init / apply -refresh-only / apply / destroy
                    │
                    ▼
              registry.terraform.io/temporalio/temporalcloud (pinned, e.g. 1.6.0)
                    │
                    ▼
              Temporal Cloud SaaS API (gRPC)
```

Drift polling is whatever `--poll` / `poll-interval` is on the provider process
(default 10m in `cmd/provider`). That re-runs Observe (refresh), not a custom loop.

## Files that matter for “control code”

### Process entry

- **`cmd/provider/main.go`**  
  Starts controller-runtime, wires `WorkspaceStore`, `SetupFn`, cluster + namespaced
  options, Terraform version/source flags. Template-derived; lightly adapted for Bitovi.

### Managed resource reconcilers (generated)

- **`internal/controller/namespaced/namespace/namespace/zz_controller.go`**  
  Namespaced Namespace (`namespace.temporalcloud.m.bitovi.com`).
- **`internal/controller/cluster/namespace/namespace/zz_controller.go`**  
  Cluster-scoped sibling (`namespace.temporalcloud.bitovi.com`).

Look for `tjcontroller.NewConnector(..., o.SetupFn, o.Provider.Resources["temporalcloud_namespace"], ...)`.
That is the Upjet glue: same pattern as other Upjet providers. **Do not hand-edit**
`zz_*` files; change generation config and re-run `make generate`.

### Register “which controllers exist”

- **`internal/controller/{cluster,namespaced}/zz_setup.go`** — generated setup list.
- **`internal/controller/{cluster,namespaced}/providerconfig/`** — ProviderConfig usage accounting (Crossplane, not Temporal).

### Credentials → Terraform (owned)

- **`internal/clients/temporalcloud.go`**  
  Only place in this repo that turns K8s secrets into TF provider config for
  Temporal. It does not open a Temporal client itself.

### Resource identity / observe-before-create (owned)

- **`config/external_name.go`**  
  Upjet seeds Terraform state `id` via `GetIDFn` before first create. Temporal
  expects `name.account_id`; see comments in that file. Wrong id shows up as
  failed Observe long before a real Temporal create.

### Schema / generation inputs

- **`config/schema.json`**, **`config/provider-metadata.yaml`** — captured from
  the Terraform provider (pulled during generate pipeline).
- **`config/provider.go`** — `GetProvider` / `GetProviderNamespaced`, include list
  from external-name keys.
- **`cmd/generator`** — generator entry used by `make generate`.

### Terraform provider (the actual Temporal I/O)

Not vendored as source in this repo. At runtime:

- env / Makefile: `TERRAFORM_PROVIDER_SOURCE=temporalio/temporalcloud`,
  `TERRAFORM_PROVIDER_VERSION=1.6.0` (or whatever is pinned).
- Binary lands under the workspace’s `.terraform/providers/...` after `terraform init`.

Official provider:
[registry.terraform.io/providers/temporalio/temporalcloud](https://registry.terraform.io/providers/temporalio/temporalcloud).

Anything like “how does Create Namespace map to Ops API?” is in **that** GitHub
codebase, not under `internal/` here.

## What is *not* here

- No direct Temporal Cloud SDK usage in provider Go.
- No custom HTTP/gRPC client to `saas-api.tmprl.cloud` in this tree.
- No composition / platform GitOps logic (that lives in platform-services or the
  user cluster, outside this provider).

If you need custom behavior that Terraform exposes poorly, you either extend via
Upjet config/hooks or leave Upjet and write a native Crossplane provider that
calls Temporal’s API directly. This POC is Path A: thin Upjet over the TF provider.

## Related commands

| Command | Effect |
| --- | --- |
| `make generate` | Regenerate `apis/`, `zz_*` controllers, package CRDs from schema + config |
| `make run` | Out-of-cluster process: same controllers as the packaged image |
| Provider package install | Same binary in-cluster; CRDs from the xpkg |

## Mental model

```text
  generated    owned       library           external
 ──────────   ────────   ────────────     ──────────────
  zz_controller  clients   upjet/controller   terraform CLI
  apis/*         config    upjet/terraform    temporalio plugin
  package/crds   main.go                      Temporal Cloud
```

Syncing “to Temporal” = Upjet reconciling the MR until TF state matches
`spec.forProvider`, with the Temporal plugin doing the network I/O.
