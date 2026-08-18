# Authentication Samples

This directory contains Perses CRD samples for the various operator client authentication
mechanisms. These control how the **operator authenticates to the Perses API** it manages — a
separate concern from how human users log into Perses.

## Overview

The operator needs to authenticate to the Perses instance it manages in order to reconcile
dashboards, datasources, and other resources. This is configured under `spec.client` on the `Perses`
CR. Three mechanisms are supported:

| Sample                                              | Mechanism                              | API version |
|-----------------------------------------------------|----------------------------------------|-------------|
| `perses.dev_v1alpha1_perses_client_k8s.yaml`        | Kubernetes ServiceAccount bearer token | v1alpha1    |
| `perses.dev_v1alpha1_perses_client_basic_auth.yaml` | HTTP Basic Auth (username + password)  | v1alpha1    |
| `perses.dev_v1alpha2_perses_client_oauth.yaml`      | OAuth 2.0 client_credentials flow      | v1alpha2    |

---

## `spec.client.oauth` — OAuth 2.0 client_credentials (v1alpha2)

**Sample:** [`perses.dev_v1alpha2_perses_client_oauth.yaml`](perses.dev_v1alpha2_perses_client_oauth.yaml)

### When to use

Use this when Perses has authentication enabled (`spec.config.security.enable_auth: true`) with an
external OIDC/OAuth provider. The operator authenticates with the same non-interactive
`client_credentials` flow that `percli login` uses, so tokens are always minted by Perses itself —
there is no custom token signing on the operator side.

### How it works

The operator's REST client performs an OAuth 2.0 `client_credentials` grant against Perses's **own**
native provider token endpoint (`tokenURL`), *not* the external identity provider directly. Perses
forwards the client credentials to the identity provider, reads the resulting user info, syncs the
user, and issues its own Perses access/refresh token. The client uses that token on every API call
and refreshes it automatically when it expires.

The `tokenURL` points at Perses and has the form:

```
<perses-url>/api/auth/providers/<oidc|oauth>/<slug_id>/token
```

For example, for an in-cluster Perses with an OIDC provider whose `slug_id` is `oidc-provider`:

```
http://perses.perses-system.svc.cluster.local:8080/api/auth/providers/oidc/oidc-provider/token
```

### Credential sources

`spec.client.oauth` supports multiple source types for the client ID and secret:

#### `type: secret` (recommended)

```yaml
spec:
  client:
    oauth:
      type: secret
      name: perses-config
      namespace: perses-system
      clientIDPath: OPERATOR_CLIENT_ID
      clientSecretPath: OPERATOR_CLIENT_SECRET
      tokenURL: http://perses.perses-system.svc.cluster.local:8080/api/auth/providers/oidc/oidc-provider/token
```

#### `type: configmap`

Same as `secret` but reads the client ID/secret from a ConfigMap. Suitable for development
environments only — do not store secrets in ConfigMaps in production.

#### `type: file`

Reads the client ID from `clientIDPath` and the client secret from `clientSecretPath` directly
from files mounted into the operator pod (e.g. via `spec.volumeMounts`). Both paths are required.
This avoids storing credentials in Kubernetes Secrets/ConfigMaps and is useful when the operator
runs alongside an external secrets manager that populates files.

```yaml
spec:
  client:
    oauth:
      type: file
      clientIDPath: /etc/perses-creds/oauth-client-id
      clientSecretPath: /etc/perses-creds/oauth-client-secret
      tokenURL: http://perses.perses-system.svc.cluster.local:8080/api/auth/providers/oidc/oidc-provider/token
```

Optional fields: `scopes` (requested OAuth scopes) and `authStyle` (how the client ID/secret are
sent; `0` auto-detects).

### Mutual exclusivity

`kubernetesAuth` and `oauth` (or `basicAuth`) cannot be enabled at the same time on
`spec.client`. If `kubernetesAuth.enable: true` is set alongside an `oauth` or `basicAuth` block,
the CRD admission webhook rejects the resource at creation time. In the operator logs, a conflicting
configuration produces:

```
kubernetesAuth and oauth are mutually exclusive; both cannot be enabled simultaneously
```

### The shared `perses-config` Secret

The sample keeps all shared credentials and provisioning resources in a single Secret:

| Key                               | Purpose                                                              |
|-----------------------------------|----------------------------------------------------------------------|
| `CLIENT_ID`                       | Human OIDC client ID injected via `spec.env` into the Perses server  |
| `CLIENT_SECRET`                   | Human OIDC client secret mounted as a file for `client_secret_file`  |
| `OPERATOR_CLIENT_ID`              | Operator's OAuth client ID for the `client_credentials` grant        |
| `OPERATOR_CLIENT_SECRET`          | Operator's OAuth client secret for the `client_credentials` grant    |
| `ENCRYPTION_KEY`                  | Key the Perses server uses to sign its own tokens (server-side only) |
| `OPERATOR_ROLE_PROV.yaml`         | Provisioning GlobalRole granting full CRUD on all resource types     |
| `OPERATOR_ROLE_BINDING_PROV.yaml` | Provisioning GlobalRoleBinding binding the role to the user          |
| `CLICKHOUSE_AUTH_SEC.yaml`        | Datasource proxy secret for Clickhouse without auth                  |

Unlike the previous JWT setup, the `ENCRYPTION_KEY` is **not** shared with the operator — it is used
only by the Perses server (`security.encryption_key_file`) to sign the tokens it issues.

### Granting the operator permissions

Perses maps the operator's `client_credentials` identity (from the IdP user info) to a Perses user.
That user needs permissions — the operator cannot grant them to itself. The sample provisions three
resources via `spec.provisioning`:

1. **User** — links the operator's identity (`subject` / `email`) to the OIDC provider. Perses
   currently also requires a native password, so a dummy value is set.
2. **GlobalRole** — the permissions the operator needs to reconcile dashboards, datasources, etc.
3. **GlobalRoleBinding** — binds the role to the user.

The `subject` / `email` in the User's `oauthProviders` entry must match what your identity provider
returns for the operator's `client_credentials` identity, and the provider `slug_id` in `tokenURL`
must match the Perses server's OIDC provider configuration.

### RBAC

No additional RBAC is required. The operator's ServiceAccount already holds a `ClusterRoleBinding`
to a `ClusterRole` that grants `get` on `secrets` and `configmaps` in any namespace
(`config/rbac/role.yaml`).

### Credential rotation

The operator caches the Perses client for 5 minutes. After rotating the operator's client secret in
the Secret, the new value is picked up when the cache entry expires — or immediately if you bump the
Perses CR (e.g. add an annotation), which changes the CR generation and invalidates the cache. The
Perses-issued token itself is refreshed automatically by the client as it expires.

---

## `spec.client.kubernetesAuth` — Kubernetes ServiceAccount token (v1alpha1)

**Sample:** [`perses.dev_v1alpha1_perses_client_k8s.yaml`](perses.dev_v1alpha1_perses_client_k8s.yaml)

The operator reads its own ServiceAccount token from
`/var/run/secrets/kubernetes.io/serviceaccount/token` and sends it as a Bearer token. Perses
validates it via the Kubernetes `TokenReview` API.

Requires both `authentication.providers.kubernetes.enable: true` **and**
`authorization.provider.kubernetes.enable: true` in `spec.config.security`. This is mutually
exclusive with external OIDC providers.

---

## `spec.client.basicAuth` — HTTP Basic Auth (v1alpha1)

**Sample:** [`perses.dev_v1alpha1_perses_client_basic_auth.yaml`](perses.dev_v1alpha1_perses_client_basic_auth.yaml)

The operator authenticates with a username and password. Requires `enable_native: true` in
`spec.config.security.authentication.providers`.
