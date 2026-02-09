# API Reference

## Packages
- [perses.dev/v1alpha2](#persesdevv1alpha2)


## perses.dev/v1alpha2

Package v1alpha2 contains API Schema definitions for the v1alpha2 API group

### Resource Types
- [Perses](#perses)
- [PersesDashboard](#persesdashboard)
- [PersesDatasource](#persesdatasource)
- [PersesGlobalDatasource](#persesglobaldatasource)



#### BasicAuth







_Appears in:_
- [Client](#client)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[SecretSourceType](#secretsourcetype)_ | Type source type of secret |  | Enum: [secret configmap file] <br />Required: \{\} <br /> |
| `name` _string_ | Name of basic auth k8s resource (when type is secret or configmap) |  | Optional: \{\} <br /> |
| `namespace` _string_ | Namespace of certificate k8s resource (when type is secret or configmap) |  | Optional: \{\} <br /> |
| `username` _string_ | Username for basic auth |  | MinLength: 1 <br />Required: \{\} <br /> |
| `password_path` _string_ | Path to password |  | MinLength: 1 <br />Required: \{\} <br /> |


#### Certificate







_Appears in:_
- [TLS](#tls)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[SecretSourceType](#secretsourcetype)_ | Type source type of secret |  | Enum: [secret configmap file] <br />Required: \{\} <br /> |
| `name` _string_ | Name of basic auth k8s resource (when type is secret or configmap) |  | Optional: \{\} <br /> |
| `namespace` _string_ | Namespace of certificate k8s resource (when type is secret or configmap) |  | Optional: \{\} <br /> |
| `certPath` _string_ | Path to Certificate |  | MinLength: 1 <br />Required: \{\} <br /> |
| `privateKeyPath` _string_ | Path to Private key certificate |  | Optional: \{\} <br /> |


#### Client







_Appears in:_
- [DatasourceSpec](#datasourcespec)
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `basicAuth` _[BasicAuth](#basicauth)_ | BasicAuth basic auth config for perses client |  | Optional: \{\} <br /> |
| `oauth` _[OAuth](#oauth)_ | OAuth configuration for perses client |  | Optional: \{\} <br /> |
| `tls` _[TLS](#tls)_ | TLS the equivalent to the tls_config for perses client |  | Optional: \{\} <br /> |
| `kubernetesAuth` _[KubernetesAuth](#kubernetesauth)_ | KubernetesAuth configuration for perses client |  | Optional: \{\} <br /> |


#### Dashboard



Dashboard represents the Perses dashboard configuration including
display settings, datasources, variables, panels, layouts, and time ranges.



_Appears in:_
- [PersesDashboardSpec](#persesdashboardspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `display` _[Display](#display)_ |  |  |  |
| `datasources` _object (keys:string, values:[DatasourceSpec](#datasourcespec))_ | Datasources is an optional list of datasource definition. |  |  |
| `variables` _Variable array_ |  |  |  |
| `panels` _object (keys:string, values:[Panel](#panel))_ |  |  |  |
| `layouts` _Layout array_ |  |  |  |
| `duration` _[Duration](#duration)_ | Duration is the default time range to use when getting data to fill the dashboard |  | Format: duration <br />Pattern: `^(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?$` <br />Type: string <br /> |
| `refreshInterval` _[Duration](#duration)_ | RefreshInterval is the default refresh interval to use when landing on the dashboard |  | Format: duration <br />Pattern: `^(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?$` <br />Type: string <br /> |


#### Datasource



Datasource represents the Perses datasource configuration including
display metadata, default flag, and plugin-specific settings.



_Appears in:_
- [DatasourceSpec](#datasourcespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `display` _[Display](#display)_ |  |  |  |
| `default` _boolean_ |  |  |  |
| `plugin` _[Plugin](#plugin)_ | Plugin will contain the datasource configuration.<br />The data typed is available in Cue. |  |  |


#### DatasourceSpec



DatasourceSpec defines the desired state of a Perses datasource



_Appears in:_
- [PersesDatasource](#persesdatasource)
- [PersesGlobalDatasource](#persesglobaldatasource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `config` _[Datasource](#datasource)_ | Perses datasource configuration |  | Required: \{\} <br /> |
| `client` _[Client](#client)_ | Client authentication and TLS configuration for the datasource |  | Optional: \{\} <br /> |
| `instanceSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#labelselector-v1-meta)_ | InstanceSelector selects Perses instances where this datasource will be created |  | Optional: \{\} <br /> |


#### KubernetesAuth







_Appears in:_
- [Client](#client)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ | Enable kubernetes auth for perses client |  |  |


#### Metadata



Metadata to add to deployed pods



_Appears in:_
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `labels` _object (keys:string, values:string)_ | Labels are key/value pairs attached to pods |  |  |
| `annotations` _object (keys:string, values:string)_ | Annotations are key/value pairs attached to pods for non-identifying metadata |  |  |


#### OAuth







_Appears in:_
- [Client](#client)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[SecretSourceType](#secretsourcetype)_ | Type source type of secret |  | Enum: [secret configmap file] <br />Required: \{\} <br /> |
| `name` _string_ | Name of basic auth k8s resource (when type is secret or configmap) |  | Optional: \{\} <br /> |
| `namespace` _string_ | Namespace of certificate k8s resource (when type is secret or configmap) |  | Optional: \{\} <br /> |
| `clientIDPath` _string_ | Path to client id |  | Optional: \{\} <br /> |
| `clientSecretPath` _string_ | Path to client secret |  | Optional: \{\} <br /> |
| `tokenURL` _string_ | TokenURL is the resource server's token endpoint<br />URL. This is a constant specific to each server. |  | MinLength: 1 <br />Required: \{\} <br /> |
| `scopes` _string array_ | Scope specifies optional requested permissions. |  | Optional: \{\} <br /> |
| `endpointParams` _object (keys:string, values:string array)_ | EndpointParams specifies additional parameters for requests to the token endpoint. |  | Optional: \{\} <br /> |
| `authStyle` _integer_ | AuthStyle optionally specifies how the endpoint wants the<br />client ID & client secret sent. The zero value means to<br />auto-detect. |  | Optional: \{\} <br /> |


#### Perses



Perses is the Schema for the perses API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `perses.dev/v1alpha2` | | |
| `kind` _string_ | `Perses` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PersesSpec](#persesspec)_ |  |  |  |
| `status` _[PersesStatus](#persesstatus)_ |  |  |  |


#### PersesConfig



PersesConfig represents the Perses server configuration including
API, security, database, provisioning, and plugin settings.



_Appears in:_
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `api_prefix` _string_ | Use it in case you want to prefix the API path. |  |  |
| `security` _[Security](#security)_ | Security contains any configuration that changes the API behavior like the endpoints exposed or if the permissions are activated. |  |  |
| `database` _[Database](#database)_ | Database contains the different configuration depending on the database you want to use |  |  |
| `schemas` _[Schemas](#schemas)_ | Schemas contain the configuration to get access to the CUE schemas<br />DEPRECATED.<br />Please remove it from your config. |  |  |
| `dashboard` _[DashboardConfig](#dashboardconfig)_ | Dashboard contains the configuration for the dashboard feature. |  |  |
| `provisioning` _[ProvisioningConfig](#provisioningconfig)_ | Provisioning contains the provisioning config that can be used if you want to provide default resources. |  |  |
| `datasource` _[DatasourceConfig](#datasourceconfig)_ | Datasource contains the configuration for the datasource. |  |  |
| `variable` _[VariableConfig](#variableconfig)_ | Variable contains the configuration for the variable. |  |  |
| `ephemeral_dashboards_cleanup_interval` _[Duration](#duration)_ | EphemeralDashboardsCleanupInterval is the interval at which the ephemeral dashboards are cleaned up<br />DEPRECATED.<br />Please use the config EphemeralDashboard instead. |  | Format: duration <br />Pattern: `^(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?$` <br />Type: string <br /> |
| `ephemeral_dashboard` _[EphemeralDashboard](#ephemeraldashboard)_ | EphemeralDashboard contains the config about the ephemeral dashboard feature |  |  |
| `frontend` _[Frontend](#frontend)_ | Frontend contains any config that will be used by the frontend itself. |  |  |
| `plugin` _[Plugin](#plugin)_ | Plugin contains the config for runtime plugins. |  |  |


#### PersesDashboard



PersesDashboard is the Schema for the persesdashboards API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `perses.dev/v1alpha2` | | |
| `kind` _string_ | `PersesDashboard` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PersesDashboardSpec](#persesdashboardspec)_ |  |  |  |
| `status` _[PersesDashboardStatus](#persesdashboardstatus)_ |  |  |  |


#### PersesDashboardSpec



PersesDashboardSpec defines the desired state of PersesDashboard



_Appears in:_
- [PersesDashboard](#persesdashboard)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `config` _[Dashboard](#dashboard)_ | Perses dashboard configuration |  | Required: \{\} <br /> |
| `instanceSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#labelselector-v1-meta)_ | InstanceSelector selects Perses instances where this dashboard will be created |  | Optional: \{\} <br /> |


#### PersesDashboardStatus



PersesDashboardStatus defines the observed state of PersesDashboard



_Appears in:_
- [PersesDashboard](#persesdashboard)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest observations of the PersesDashboard resource state |  |  |


#### PersesDatasource



PersesDatasource is the Schema for the PersesDatasources API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `perses.dev/v1alpha2` | | |
| `kind` _string_ | `PersesDatasource` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[DatasourceSpec](#datasourcespec)_ |  |  |  |
| `status` _[PersesDatasourceStatus](#persesdatasourcestatus)_ |  |  |  |


#### PersesDatasourceStatus



PersesDatasourceStatus defines the observed state of PersesDatasource



_Appears in:_
- [PersesDatasource](#persesdatasource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest observations of the PersesDatasource resource state |  |  |


#### PersesGlobalDatasource



PersesGlobalDatasource is the Schema for the PersesGlobalDatasources API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `perses.dev/v1alpha2` | | |
| `kind` _string_ | `PersesGlobalDatasource` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[DatasourceSpec](#datasourcespec)_ |  |  |  |
| `status` _[PersesGlobalDatasourceStatus](#persesglobaldatasourcestatus)_ |  |  |  |


#### PersesGlobalDatasourceStatus



PersesGlobalDatasourceStatus defines the observed state of PersesGlobalDatasource



_Appears in:_
- [PersesGlobalDatasource](#persesglobaldatasource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest observations of the PersesGlobalDatasource resource state |  |  |


#### PersesService



PersesService defines service configuration for Perses



_Appears in:_
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Kubernetes service |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations attached to the service for non-identifying metadata |  | Optional: \{\} <br /> |


#### PersesSpec



PersesSpec defines the desired state of Perses



_Appears in:_
- [Perses](#perses)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `client` _[Client](#client)_ | Perses client configuration |  | Optional: \{\} <br /> |
| `config` _[PersesConfig](#persesconfig)_ | Perses server configuration |  |  |
| `args` _string array_ | Args extra arguments to pass to perses |  |  |
| `containerPort` _integer_ |  | 8080 | Maximum: 65535 <br />Minimum: 1 <br /> |
| `replicas` _integer_ |  |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#resourcerequirements-v1-core)_ | Resources defines the compute resources configured for the container. |  | Optional: \{\} <br /> |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector constrains pods to nodes with matching labels |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#toleration-v1-core) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#affinity-v1-core)_ |  |  | Optional: \{\} <br /> |
| `image` _string_ | Image specifies the container image that should be used for the Perses deployment. |  | Optional: \{\} <br /> |
| `service` _[PersesService](#persesservice)_ | service specifies the service configuration for the perses instance |  | Optional: \{\} <br /> |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#probe-v1-core)_ |  |  | Optional: \{\} <br /> |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#probe-v1-core)_ |  |  | Optional: \{\} <br /> |
| `tls` _[TLS](#tls)_ | tls specifies the tls configuration for the perses instance |  | Optional: \{\} <br /> |
| `storage` _[StorageConfiguration](#storageconfiguration)_ | Storage configuration used by the StatefulSet | \{ size:1Gi \} | Optional: \{\} <br /> |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the service account to use for the perses deployment or statefulset. |  | Optional: \{\} <br /> |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#podsecuritycontext-v1-core)_ | PodSecurityContext holds pod-level security attributes and common container settings.<br />If not specified, defaults to fsGroup: 65534 to ensure proper volume permissions for the nobody user. |  | Optional: \{\} <br /> |
| `logLevel` _string_ | LogLevel defines the log level for Perses. |  | Enum: [panic fatal error warning info debug trace] <br />Optional: \{\} <br /> |
| `logMethodTrace` _boolean_ | LogMethodTrace when true, includes the calling method as a field in the log.<br />It can be useful to see immediately where the log comes from. |  | Optional: \{\} <br /> |


#### PersesStatus



PersesStatus defines the observed state of Perses



_Appears in:_
- [Perses](#perses)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest observations of the Perses resource state |  |  |


#### SecretSource



SecretSource configuration for a perses secret source



_Appears in:_
- [BasicAuth](#basicauth)
- [Certificate](#certificate)
- [OAuth](#oauth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[SecretSourceType](#secretsourcetype)_ | Type source type of secret |  | Enum: [secret configmap file] <br />Required: \{\} <br /> |
| `name` _string_ | Name of basic auth k8s resource (when type is secret or configmap) |  | Optional: \{\} <br /> |
| `namespace` _string_ | Namespace of certificate k8s resource (when type is secret or configmap) |  | Optional: \{\} <br /> |


#### SecretSourceType

_Underlying type:_ _string_

SecretSourceType types of secret sources in k8s



_Appears in:_
- [BasicAuth](#basicauth)
- [Certificate](#certificate)
- [OAuth](#oauth)
- [SecretSource](#secretsource)

| Field | Description |
| --- | --- |
| `secret` |  |
| `configmap` |  |
| `file` |  |


#### StorageConfiguration



StorageConfiguration is the configuration used to create and reconcile PVCs



_Appears in:_
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `storageClass` _string_ | StorageClass to use for PVCs.<br />If not specified, will use the default storage class |  | Optional: \{\} <br /> |
| `size` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#quantity-resource-api)_ | Size of the storage.<br />cannot be decreased. |  | Optional: \{\} <br /> |


#### TLS







_Appears in:_
- [Client](#client)
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ | Enable TLS connection to perses |  |  |
| `caCert` _[Certificate](#certificate)_ | CaCert to verify the perses certificate |  | Optional: \{\} <br /> |
| `userCert` _[Certificate](#certificate)_ | UserCert client cert/key for mTLS |  | Optional: \{\} <br /> |
| `insecureSkipVerify` _boolean_ | InsecureSkipVerify skip verify of perses certificate |  | Optional: \{\} <br /> |


