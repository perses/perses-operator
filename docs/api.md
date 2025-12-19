# Perses Operator Documentation

This documentation provides information on how to use the Perses Operator custom resources to manage Perses installations in Kubernetes.

## Table of Contents

- [Custom Resources](#custom-resources)
  - [Perses](#perses)
  - [PersesDatasource](#persesdatasource)
  - [PersesDashboard](#persesdashboard)
- [Examples](#examples)
- [Project Management](#project-management)
- [Troubleshooting](#troubleshooting)

## Custom Resources

The Perses Operator introduces the following Custom Resource Definitions (CRDs):

### Perses

The `Perses` CRD is the main resource that deploys and configures a Perses server instance.

The Perses server instances are namespace-scoped, the operator will deploy a Perses server in the same namespace as the Perses CR. Datasources and dashboards created in other namespaces will be synchronized across all Perses servers in the cluster.

#### Specification

```yaml
apiVersion: perses.dev/v1alpha1
kind: Perses
metadata:
  name: perses-sample
  namespace: perses-dev
spec:
  # Optional configuration for the Perses client that the operator will use to connect to Perses servers
  client:
    tls:
      enable: true
      caCert:
        type: secret
        name: perses-certs
        certPath: ca.crt
      userCert:
        type: secret
        name: perses-certs
        certPath: tls.crt
        privateKeyPath: tls.key

  # Optional container image to use as the Perses server operand
  image: docker.io/perses/perses:v0.50.3

  # Optional service configuration
  service:
    name: perses-service
    annotations:
      my.service/annotation: "true"

  # A Complete Perses configuration https://perses.dev/perses/docs/configuration/configuration/
  config:
    database:
      file:
        folder: "/perses"
        extension: "yaml"
    schemas:
      panels_path: "/etc/perses/cue/schemas/panels"
      queries_path: "/etc/perses/cue/schemas/queries"
      datasources_path: "/etc/perses/cue/schemas/datasources"
      variables_path: "/etc/perses/cue/schemas/variables"
    ephemeral_dashboard:
      enable: false
      cleanup_interval: "1s"

  # Optional TLS configuration
  tls:
    enable: true
    caCert:
      type: secret
      name: perses-certs
      certPath: ca.crt
    userCert:
      type: secret
      name: perses-certs
      certPath: tls.crt
      privateKeyPath: tls.key

  replicas: 1
  containerPort: 8080

  livenessProbe:
    initialDelaySeconds: 30
    periodSeconds: 10
    timeoutSeconds: 5
    successThreshold: 1
    failureThreshold: 3

  readinessProbe:
    initialDelaySeconds: 30
    periodSeconds: 10
    timeoutSeconds: 5
    successThreshold: 1
    failureThreshold: 3
```

### PersesDatasource

The `PersesDatasource` CRD allows you to define datasources that can be used in your Perses dashboards. These datasources provide the data for visualizations and panels.

The `PersesDatasource` configurations are namespace-scoped.
They will be created under a Perses project that corresponds to the namespace where the CR is created.
For instance, a `PersesDatasource` created in the `monitoring` namespace
will be created under the `monitoring` project in the Perses server.

To configure a secret to be used for proxy authentication,
you can create a Kubernetes Secret with the necessary credentials
and reference it in the `client` field used for the datasource proxy configuration.
This will create a Perses secret in the project corresponding to the namespace where the CR is created.
The secret will be named after the Datasource name with a `-secret` suffix.
The secret must be referenced in `spec.config.spec.proxy.spec.secret`.

#### PersesGlobalDatasource

The `PersesGlobalDatasource` CRD allows you to define global datasources that are accessible across all Perses projects.
The API is the same as `PersesDatasource`, but the resources are globally scoped.
No project mapping is created and a `GlobalSecret` is created for proxy authentication if needed.

#### Specification

```yaml
apiVersion: perses.dev/v1alpha1
kind: PersesDatasource
metadata:
  name: prometheus-through-proxy
  namespace: monitoring
spec:
  config: # A complete spec of a Perses datasource: https://perses.dev/perses/docs/api/datasource/
    kind: PrometheusSource
    spec:
      default: true
      proxy:
        kind: HTTPProxy
        spec:
          url: "https://prometheus-server.monitoring.svc.cluster.local:9090"
          secret: prometheus-through-proxy-secret

  # Optional datasource proxy client configuration
  client:
    tls:
      enable: true
      caCert:
        type: secret # May be of type `secret`, `configmap` or `file`
        name: prometheus-certs # In this case the k8s secret name
        certPath: ca.crt # The key
      userCert:
        type: secret # May be of type `secret`, `configmap` or `file`
        name: prometheus-certs
        certPath: tls.crt
        privateKeyPath: tls.key
```

```yaml
apiVersion: perses.dev/v1alpha1
kind: PersesDatasource
metadata:
  name: prometheus
  namespace: monitoring
spec:
  config: # A complete spec of a Perses datasource: https://perses.dev/perses/docs/api/datasource/
    kind: PrometheusSource
    spec:
      default: true
      directUrl: "https://prometheus.demo.prometheus.io"
```

### PersesDashboard

The `PersesDashboard` CRD allows you to define Perses dashboards directly using Kubernetes resources. This enables dashboard-as-code practices and GitOps workflows in conjunction with [percli](https://perses.dev/perses/docs/cli/) and the [Perses Go SDK](https://perses.dev/perses/docs/dac/go/).

The PersesDashboard configurations are namespace-scoped.

#### Specification

```yaml
apiVersion: perses.dev/v1alpha1
kind: PersesDashboard
metadata:
  name: kubernetes-overview
  namespace: monitoring
spec: # The complete spec of a Perses dashboard: https://perses.dev/perses/docs/api/dashboard/
  display:
    name: "Kubernetes Overview"
    description: "Overview of Kubernetes cluster metrics"
  variables:
    - kind: ListVariable
      spec:
        name: job
        allowMultiple: false
        allowAllValue: false
        plugin:
          kind: PrometheusLabelValuesVariable
          spec:
            labelName: job
  panels:
    defaultTimeSeriesChart:
      kind: Panel
      spec:
        display:
          name: Default Time Series Panel
        plugin:
          kind: TimeSeriesChart
          spec: {}
        queries:
          - kind: TimeSeriesQuery
            spec:
              plugin:
                kind: PrometheusTimeSeriesQuery
                spec:
                  query: up
  layouts:
    - kind: Grid
      spec:
        display:
          title: Row 1
          collapse:
            open: true
        items:
          - x: 0
            y: 0
            width: 2
            height: 3
            content:
              "$ref": "#/spec/panels/defaultTimeSeriesChart"
  duration: 1h
```

## Project Management

The Perses operator maps Perses projects to Kubernetes namespaces. When you create a namespace in Kubernetes, it can be used as a project in Perses. This approach simplifies resource management and aligns with Kubernetes native organization principles.

When reconciling Dashboards or Datasources the Perses operator synchronizes the namespace into a Perses project across all Perses servers in the cluster.

## Secrets

Perses secrets are exclusively managed by the Perses Operator with
[`PersesDatasource`](#persesdatasource) and
[`PersesGlobalDatasource`](#persesglobaldatasource) resources
under the `client` field for proxy configuration.

The api supports three types of secret sources:

- `secret`: Kubernetes Secret
- `configmap`: Kubernetes ConfigMap
- `file`: File mounted in the perses pod

The api for these types are the same
but the keys ending in `Path` refer to a key within a secret or configmap
when using those types.

> [!WARNING]
> The `file` type is not useful in the current state
> as there is no way to mount files into the perses pod.

```yaml
apiVersion: perses.dev/v1alpha1
kind: PersesDatasource
metadata:
  name: prometheus-through-proxy
  namespace: monitoring
spec:
  config: ...
  # Optional datasource proxy client configuration
  client:
    basicAuth:
      type: secret
      name: k8s-basicauth-secret-name
      namespace: optional-namespacename # if the secret resides in another namespace
      username: "actual-username"
      password_path: "password-key-in-secret" # or an actual path if type is `file`
    oauth:
      type: secret
      name: k8s-oauth-secret-name
      # namespace: monitoring
      clientIDPath: client-id-key-in-secret
      clientSecretPath: client-secret-key-in-secret
      tokenURL: https://auth.example.com/token
      scopes:
        - read:metrics
      endpointParams:
        audience: prometheus
      authStyle: dunno
    tls:
      enable: true
      caCert:
        type: secret # May be of type `secret`, `configmap` or `file`
        name: prometheus-certs # In this case the k8s secret name
        certPath: ca.crt # The key in the secret
      userCert:
        type: secret # May be of type `secret`, `configmap` or `file`
        name: prometheus-certs
        certPath: tls.crt
        privateKeyPath: tls.key
```

> [!NOTE]
> The `basicAuth` and `oauth` fields are mutually exclusive.

## Examples

### Simple Perses Installation

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: monitoring
---
apiVersion: perses.dev/v1alpha1
kind: Perses
metadata:
  name: perses
  namespace: monitoring
spec:
  image: docker.io/perses/perses:v0.50.3
  config:
    database:
      file:
        folder: "/perses"
        extension: "yaml"
    ephemeral_dashboard:
      enable: false
      cleanup_interval: "1s"
  containerPort: 8080
```

### Creating Datasources and Dashboards

```yaml
# Create a Prometheus datasource
apiVersion: perses.dev/v1alpha1
kind: PersesDatasource
metadata:
  name: prometheus-main
  namespace: monitoring
spec:
  config:
    display:
      name: "Default Datasource"
    default: true
    plugin:
      kind: "PrometheusDatasource"
      spec:
        directUrl: "https://prometheus.demo.prometheus.io"
---
# Create a dashboard
apiVersion: perses.dev/v1alpha1
kind: PersesDashboard
metadata:
  name: perses-dashboard-sample
  namespace: monitoring
spec:
  display:
    name: perses-dashboard-sample
  panels:
    defaultTimeSeriesChart:
      kind: Panel
      spec:
        display:
          name: defaultTimeSeriesChart
        plugin:
          kind: TimeSeriesChart
          spec: {}
        queries:
          - kind: TimeSeriesQuery
            spec:
              plugin:
                kind: PrometheusTimeSeriesQuery
                spec:
                  query: up
    seriesTest:
      kind: Panel
      spec:
        display:
          name: seriesTest
        plugin:
          kind: TimeSeriesChart
          spec: {}
        queries:
          - kind: TimeSeriesQuery
            spec:
              plugin:
                kind: PrometheusTimeSeriesQuery
                spec:
                  query: rate(caddy_http_response_duration_seconds_sum[$interval])
  layouts:
    - kind: Grid
      spec:
        display:
          title: Panel Group
          collapse:
            open: true
        items:
          - x: 0
            y: 0
            width: 12
            height: 6
            content:
              $ref: "#/spec/panels/defaultTimeSeriesChart"
          - x: 12
            y: 0
            width: 12
            height: 6
            content:
              $ref: "#/spec/panels/seriesTest"
  variables:
    - kind: ListVariable
      spec:
        name: interval
        allowAllValue: false
        allowMultiple: false
        plugin:
          kind: StaticListVariable
          spec:
            values:
              - value: 1m
                label: 1m
              - value: 5m
                label: 5m
        defaultValue: 1m
  duration: 1h
```

## Troubleshooting

### Common Issues

1. **Connection issues with Perses server**:

   - Check if the Perses deployment is running correctly
   - Verify network policies allow access to the Perses service

2. **Operator not processing CRs**:

   - Check the operator logs for errors
   - Verify that the correct CRDs are installed

3. **Datasources not working**:

   - Verify the datasource URL is accessible from the Perses pods
   - Check that a proxy is correctly configured if needed
   - Check credentials if authentication is required
   - Look for errors in the Perses server logs

4. **Dashboards not appearing**:
   - Check that the dashboard is created in the correct namespace
   - Verify that referenced datasources exist and are accessible

For more detailed information or support, please visit the [Perses GitHub repository](https://github.com/perses/perses).
