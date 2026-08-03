# provider-temporalcloud

Bitovi Crossplane provider for [Temporal Cloud](https://docs.temporal.io/cloud).
Generated with [Upjet](https://github.com/crossplane/upjet) from Terraform provider
[`temporalio/temporalcloud`](https://registry.terraform.io/providers/temporalio/temporalcloud) **1.6.0**.

Package name: **`provider-temporalcloud`**  
OCI image: **`ghcr.io/bitovi/provider-temporalcloud`**  
Source: [github.com/bitovi/provider-temporal](https://github.com/bitovi/provider-temporal)  
Go module: `github.com/bitovi/provider-temporal`

## Can this run on Crossplane if it is open source?

**Yes.** Crossplane only needs to **pull an OCI provider package** and install a
`Provider` object. Licensing of this repo (Apache-2.0 template) and Temporal
Cloud credentials you supply are separate questions.

- **Package visibility is independent of the GitHub repo.** GHCR packages often
  stay **private** even when the source repo is public. Until the package is
  marked public, clusters need a pull secret (`read:packages`).
- Open-sourcing the **GitHub repo** does not by itself install the controller;
  clusters install from **GHCR** (or any registry you push to), not by cloning.

You still need a Temporal Cloud API key for managed resources to reconcile.

## Architecture docs

Reconcile / “who talks to Temporal” path: **[docs/control-plane-sync.md](docs/control-plane-sync.md)**  
(Upjet-generated controllers + Terraform provider; not a hand-written Temporal client.)

---

## Install on a Crossplane cluster

### 1. Prerequisites

- Kubernetes + Crossplane already running (any distribution)
- kubectl context set to that cluster
- A published package tag (see [Publish to GHCR](#publish-to-ghcr) if none exists yet)
- [GitHub CLI](https://cli.github.com/) (`gh`) logged in with package pull access
  (`read:packages` on an account that can read `ghcr.io/bitovi/provider-temporalcloud`)

### 2. Install the provider package (private GHCR)

You only need kubectl + `gh` and a cluster with Crossplane — **no clone of this
repo**. Replace the tag with a published version.

Create (or refresh) a docker-registry secret from your current `gh` session token
(no hard-coded PAT in the shell history beyond what `gh` already uses):

```bash
export TAG=v0.1.0

# Ensure package pull scope (re-auth if the token lacks read:packages)
gh auth status
# if needed:
#   gh auth refresh -h github.com -s read:packages

kubectl -n crossplane-system create secret docker-registry ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username="$(gh api user --jq .login)" \
  --docker-password="$(gh auth token)" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl apply -f - <<EOF
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-temporalcloud
spec:
  package: ghcr.io/bitovi/provider-temporalcloud:${TAG}
  packagePullSecrets:
    - name: ghcr-pull
EOF

kubectl get provider.pkg provider-temporalcloud -w
# wait until HEALTHY=True and INSTALLED=True
```

`gh api user --jq .login` is the GitHub username GHCR expects; `gh auth token`
is the bearer token for that session. After the token rotates (logout / refresh),
re-run the secret create/`apply` lines.

If the package is later made **public**, you can drop `packagePullSecrets`
and the secret step.

If you have this repository checked out, `examples/install.yaml` matches the
private install (set `packagePullSecrets` as in that file).

### 3. Credentials + Namespace

Namespaced objects live in a normal Kubernetes namespace first. Create that
namespace, the credentials Secret, then the Crossplane objects:

```bash
export API_KEY='your-temporal-cloud-api-key'

kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: temporal-cloud
---
apiVersion: v1
kind: Secret
metadata:
  name: temporal-cloud-creds
  namespace: temporal-cloud
type: Opaque
stringData:
  credentials: |
    {"api_key":"${API_KEY}"}
---
apiVersion: temporalcloud.m.bitovi.com/v1beta1
kind: ProviderConfig
metadata:
  name: temporal-cloud
  namespace: temporal-cloud
spec:
  credentials:
    source: Secret
    secretRef:
      name: temporal-cloud-creds
      namespace: temporal-cloud
      key: credentials
---
apiVersion: namespace.temporalcloud.m.bitovi.com/v1alpha1
kind: Namespace
metadata:
  name: my-dev
  namespace: temporal-cloud
spec:
  providerConfigRef:
    kind: ProviderConfig
    name: temporal-cloud
  forProvider:
    name: my-dev
    regions:
      - aws-us-east-1
    retentionDays: 7
    apiKeyAuth: true
  managementPolicies:
    - "*"
EOF

kubectl get namespace.namespace.temporalcloud.m.bitovi.com -n temporal-cloud -w
```

Note: Kubernetes `Namespace/temporal-cloud` is not the same kind as the Temporal
Cloud MR `Namespace` (`namespace.temporalcloud.m.bitovi.com`). The first is a
cluster namespace for packaging; the second is the managed Temporal resource.

Examples: [`examples/namespaced/`](examples/namespaced/),
[`examples/cluster/`](examples/cluster/) (cluster-scoped ProviderConfig + Namespace).

### 4. What gets installed

| TF resource | Namespaced CRD group | Cluster CRD group |
| --- | --- | --- |
| `temporalcloud_namespace` | `namespace.temporalcloud.m.bitovi.com` | `namespace.temporalcloud.bitovi.com` |

Only Namespace is enabled today (`config/external_name.go`).

---

## Publish to GHCR

Packages are published to **Bitovi GHCR only**:

```text
ghcr.io/bitovi/provider-temporalcloud:<version>
```

There is **no** publish path to Crossplane’s public registry or
`crossplane-contrib`. That existed only as leftover Upjet template defaults and
has been removed.

### From CI (recommended)

Workflow: [`.github/workflows/publish-ghcr.yml`](.github/workflows/publish-ghcr.yml)

1. Push this repo to `github.com/bitovi/provider-temporal` (or your fork under an
   org that can write `ghcr.io/bitovi/*`).
2. Ensure **Packages** write for the workflow (`GITHUB_TOKEN` / org package
   permissions allow `bitovi` packages).
3. Tag and push:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The workflow: logs into `ghcr.io` as `github.actor` with `GITHUB_TOKEN`, builds
linux package + controller image (`make build.all`), then
`make publish.artifacts` with `XPKG_REG_ORGS=ghcr.io/bitovi`.

You can also run **Actions → Publish GHCR → Run workflow** and pass a version.

After publish, packages are usually **private** by default. Install with a
`packagePullSecrets` secret via `gh auth token` (see [Install](#install-on-a-crossplane-cluster)).
Optionally mark the package **public** in GitHub → Packages for anonymous pulls.

### From your laptop

```bash
export VERSION=v0.1.0
echo "$(gh auth token)" | docker login ghcr.io -u "$(gh api user --jq .login)" --password-stdin

make build.all VERSION=$VERSION
make publish.artifacts VERSION=$VERSION \
  REGISTRY_ORGS=ghcr.io/bitovi \
  XPKG_REG_ORGS=ghcr.io/bitovi
```

Then install with the private path in the Install section (or `examples/install.yaml`).

### CI that does *not* publish

[`.github/workflows/ci.yml`](.github/workflows/ci.yml) is the PR pipeline (lint,
tests, local package deploy on kind). It uploads `_output` artifacts; it does
**not** push to GHCR. Publishing is only `publish-ghcr.yml`.

---

## Install into bitovi-platform-services

After a tag exists on GHCR:

1. Add a GitOps `Provider` (same as other providers):

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-temporalcloud
spec:
  package: ghcr.io/bitovi/provider-temporalcloud:v0.1.0
```

2. ProviderConfig wired to your Temporal API key Secret.
3. Optionally cut platform Composition from `provider-terraform` Workspace to
   this Namespace managed resource.

---

## Local development (last)

Use this when hacking on the controller without publishing.

### Prerequisites

| Tool | Notes |
| --- | --- |
| Go 1.24+ | See `go.mod` |
| Terraform CLI 1.5.x | Out-of-cluster + `make generate` |
| Docker, kubectl | Images / cluster access |
| Temporal API key | Against real Cloud |

Cluster: any k8s with Crossplane (Minikube example ok).

### Out-of-cluster controller

```bash
kubectl apply -f package/crds/   # only when not installing the package
make run                         # exports TERRAFORM_* pins from Makefile
# other terminal: apply Secret + ProviderConfig + Namespace as above
```

Or:

```bash
go build -o bin/provider ./cmd/provider
export TERRAFORM_VERSION=1.5.7
export TERRAFORM_PROVIDER_SOURCE=temporalio/temporalcloud
export TERRAFORM_PROVIDER_VERSION=1.6.0
./bin/provider --debug
```

### Regenerate after config changes

```bash
make generate
go build -o bin/provider ./cmd/provider
```

### Local in-cluster package (kind / make)

```bash
make local-deploy   # builds + installs local xpkg into a kind controlplane
```

### Pin summary

| Variable | Value |
| --- | --- |
| `TERRAFORM_PROVIDER_SOURCE` | `temporalio/temporalcloud` |
| `TERRAFORM_PROVIDER_VERSION` | `1.6.0` |
| `REGISTRY_ORGS` / `XPKG_REG_ORGS` | `ghcr.io/bitovi` |

### Troubleshooting

| Symptom | Cause |
| --- | --- |
| Provider not Healthy | Wrong/missing package on GHCR; private package without pull secret |
| Binary missing flags | Local only: export `TERRAFORM_*` or use `make run` |
| Credentials errors | Secret not JSON `api_key` / raw key |
| Image build fails | Need docker + network for Terraform + temporalcloud provider zips |

---

## License

Apache-2.0 (Upjet template heritage). Temporal Cloud and the Temporal Terraform
provider are under their own terms; your Cloud API key is yours.
