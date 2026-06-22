## Openshift Sample Configuration

### Global vs Namespace-Scoped Datasources

The Perses Operator supports two kinds of datasource resources:

- **`PersesDatasource`** — namespace-scoped, available only within a single Perses project.
- **`PersesGlobalDatasource`** — cluster-scoped, shared across all projects.

Use a `PersesDatasource` when a datasource is specific to one team or project. Use a `PersesGlobalDatasource` when the datasource should be available organization-wide. The Loki, Tempo, and global Thanos Querier samples below use `PersesGlobalDatasource`.

### Connect Perses to an Openshift Cluster Thanos querier

Openshift provides a built-in Thanos Querier as part of its Monitoring stack. Using the pre-configured routes in OpenShift you can query Thanos from Perses.

1. Login to your openshift cluster using the `oc` CLI tool:
2. Retrieve the Thanos Querier route URL:

   ```bash
   oc get route thanos-querier -n openshift-monitoring -o jsonpath='{.spec.host}'
   ```
3. Create a Perses project or use an existing one.
4. In your Perses project, create a new Datasource with the following configuration:
   - **Name**: Openshift Thanos Querier
   - **Set as Default**: true
   - **Plugin Options**: Source: Prometheus Datasource
   - **HTTP Settings**: Select Proxy and set the URL to `https://<thanos-querier-route-url>` (obtained from step 2)
   - **Secret**: Set to `<datasource-name>-secret`. The operator automatically creates this secret; the proxy uses it for authentication. For example, if the datasource name is `thanos-querier-datasource`, set the secret to `thanos-querier-datasource-secret`.
   - Save the Datasource.
5. In your Perses project, create a Dashboard and its Panels. As the Datasource is the default, all the queries will use the Openshift Thanos Querier by default.

> [!IMPORTANT]
> If you face any issues connecting to the Thanos Querier, ensure that your Perses instance can reach the Openshift cluster network and that the token has the necessary permissions to access the monitoring data.

> [!NOTE]
> A global variant of this datasource is also available in `thanos-querier-global-datasource.yaml`. It uses `PersesGlobalDatasource` and connects via the in-cluster service URL (`thanos-querier.openshift-monitoring.svc.cluster.local:9091`) instead of the external route. Use the global variant when you want the Thanos Querier datasource available across all projects.

### Connect Perses to an OpenShift Cluster Thanos Querier (Tenancy-Aware)

OpenShift's Thanos Querier exposes a multi-tenancy endpoint on port **9092** that restricts metric queries to specific namespaces. Use this configuration when you want to scope metrics access per namespace, for example in multi-tenant environments where teams should only see their own metrics.

1. Login to your OpenShift cluster using the `oc` CLI tool.
2. The tenancy-aware Thanos Querier uses the in-cluster service URL on port **9092** (the multi-tenancy port):

   ```
   https://thanos-querier.openshift-monitoring.svc.cluster.local:9092
   ```

3. Create a Perses project or use an existing one.
4. In your Perses project, create a new Datasource with the following configuration:
   - **Name**: Thanos Querier Tenancy Datasource
   - **Set as Default**: true
   - **Plugin Options**: Source: Prometheus Datasource
   - **HTTP Settings**: Select Proxy and set the URL to `https://thanos-querier.openshift-monitoring.svc.cluster.local:9092`
   - **Query Parameters**: Add a `namespace` parameter with the value `${namespace:queryparam}`. This injects all the selected namespaces from variables in your dashboards as a query parameter, scoping all metric queries to the selected namespaces.
   - **Secret**: Set to `<datasource-name>-secret`. The operator automatically creates this secret; the proxy uses it for authentication. For example, if the datasource name is `thanos-querier-tenancy-datasource`, set the secret to `thanos-querier-tenancy-datasource-secret`.
   - Save the Datasource.
5. In your Perses project, create a Dashboard and add a **namespace** variable so users can select which namespaces to query. Panels using this datasource will automatically scope their metrics to the selected namespaces.

> [!IMPORTANT]
> If you face any issues connecting to the tenancy-aware Thanos Querier, ensure that your Perses instance can reach the `openshift-monitoring` namespace services on port 9092, and that the service account token has the necessary permissions to access metrics in the target namespaces.

> [!NOTE]
> A sample `PersesDatasource` configuration is available in `thanos-querier-tenancy-datasource.yaml`. It connects to the multi-tenancy port (9092) and uses `${namespace:queryparam}` for namespace scoping.

### Connect Perses to OpenShift Logging (Loki)

OpenShift Logging with LokiStack provides a centralized log aggregation platform. You can configure Perses to query logs from Loki using the Loki Datasource plugin.

> [!IMPORTANT]
> **Prerequisites**: OpenShift Logging with LokiStack must be deployed in the `openshift-logging` namespace. Verify by running:
>
> ```bash
> oc get pods -n openshift-logging
> ```

1. Login to your OpenShift cluster using the `oc` CLI tool.
2. Retrieve the Loki gateway service URL. The default in-cluster URL is:

   ```
   https://logging-loki-gateway-http.openshift-logging.svc.cluster.local:8080/api/logs/v1/<tenant>
   ```

   Replace `<tenant>` with one of the available Loki tenants:
   - **`application`** — logs from application workloads running in user namespaces.
   - **`infrastructure`** — logs from OpenShift platform and system components.
   - **`audit`** — Kubernetes API server and OpenShift audit logs.

3. Create a Perses project or use an existing one.
4. In your Perses project, create a new Datasource with the following configuration:
   - **Name**: Loki Datasource
   - **Set as Default**: true
   - **Plugin Options**: Source: Loki Datasource
   - **HTTP Settings**: Select Proxy and set the URL to the Loki gateway URL (from step 2), for example:
     `https://logging-loki-gateway-http.openshift-logging.svc.cluster.local:8080/api/logs/v1/application`
   - **Headers**: Add the header `X-Scope-OrgID` with the value matching your chosen tenant (e.g., `application`)
   - **Secret**: Set to `<datasource-name>-secret`. The operator automatically creates this secret; the proxy uses it for authentication. For example, if the datasource name is `loki-datasource`, set the secret to `loki-datasource-secret`.
   - Save the Datasource.
5. In your Perses project, create a Dashboard and add Panels with log queries. As the Datasource is the default, all log queries will use the Loki datasource by default.

> [!IMPORTANT]
> If you face any issues connecting to Loki, ensure that your Perses instance can reach the `openshift-logging` namespace services and that the token has the necessary permissions (`cluster-logging-application-view`, `cluster-logging-infrastructure-view`, or `cluster-logging-audit-view` cluster roles) to access the log data.

> [!NOTE]
> A sample `PersesGlobalDatasource` configuration is available in `loki-global-datasource.yaml`. It connects to the `application` tenant by default.

### Connect Perses to OpenShift Distributed Tracing (Tempo)

OpenShift Distributed Tracing with Tempo provides a backend for collecting and querying distributed traces. You can configure Perses to query traces from Tempo using the Tempo Datasource plugin.

> [!IMPORTANT]
> **Prerequisites**: The Tempo operator must be installed and a `TempoStack` or `TempoMonolithic` instance with multi-tenancy must be deployed.

1. Login to your OpenShift cluster using the `oc` CLI tool.
2. Retrieve the Tempo gateway service URL. Example URL:

   ```
   https://tempo-simplest-gateway.openshift-tracing.svc.cluster.local:8080/api/traces/v1/<tenant>/tempo
   ```

   Replace `<tenant>` with the name of your tracing tenant. The tenant name depends on your TempoStack configuration. Check your TempoStack CR for the configured tenants:

   ```bash
   oc get -n <namespace> tempostack <instance> -o jsonpath='{.items[*].spec.tenants}'
   ```

3. Create a Perses project or use an existing one.
4. In your Perses project, create a new Datasource with the following configuration:
   - **Name**: Tempo Datasource
   - **Set as Default**: true
   - **Plugin Options**: Source: Tempo Datasource
   - **HTTP Settings**: Select Proxy and set the URL to the Tempo gateway URL (from step 2), for example:
     `https://tempo-simplest-gateway.openshift-tracing.svc.cluster.local:8080/api/traces/v1/tenant1/tempo`
   - **Secret**: Set to `<datasource-name>-secret`. The operator automatically creates this secret; the proxy uses it for authentication. For example, if the datasource name is `tempo-datasource`, set the secret to `tempo-datasource-secret`.
   - Save the Datasource.
5. In your Perses project, create a Dashboard and add Panels with trace queries. As the Datasource is the default, all trace queries will use the Tempo datasource by default.

> [!IMPORTANT]
> If you face any issues connecting to Tempo, ensure that your Perses instance can reach the Tempo instance, that the Tempo gateway is accessible, and that the logged in user has the necessary permissions to access tracing data.

> [!NOTE]
> A sample `PersesGlobalDatasource` configuration is available in `tempo-global-datasource.yaml`. It connects to the `tenant1` tenant by default.
