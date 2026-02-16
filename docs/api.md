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
| `type` _[SecretSourceType](#secretsourcetype)_ | Type specifies the source type for secret data (secret, configmap, or file) |  | Enum: [secret configmap file] <br />Required: \{\} <br /> |
| `name` _string_ | Name is the name of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | Optional: \{\} <br /> |
| `namespace` _string_ | Namespace is the namespace of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | Optional: \{\} <br /> |
| `username` _string_ | Username is the username credential for basic authentication |  | MinLength: 1 <br />Required: \{\} <br /> |
| `passwordPath` _string_ | PasswordPath specifies the key name within the secret/configmap or filesystem path<br />(depending on SecretSource.Type) where the password is stored |  | MinLength: 1 <br />Required: \{\} <br /> |


#### Certificate







_Appears in:_
- [TLS](#tls)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[SecretSourceType](#secretsourcetype)_ | Type specifies the source type for secret data (secret, configmap, or file) |  | Enum: [secret configmap file] <br />Required: \{\} <br /> |
| `name` _string_ | Name is the name of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | Optional: \{\} <br /> |
| `namespace` _string_ | Namespace is the namespace of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | Optional: \{\} <br /> |
| `certPath` _string_ | CertPath specifies the key name within the secret/configmap or filesystem path<br />(depending on SecretSource.Type) where the certificate is stored |  | MinLength: 1 <br />Required: \{\} <br /> |
| `privateKeyPath` _string_ | PrivateKeyPath specifies the key name within the secret/configmap or filesystem path<br />(depending on SecretSource.Type) where the private key is stored<br />Required for client certificates (UserCert), optional for CA certificates (CaCert) |  | Optional: \{\} <br /> |


#### Client







_Appears in:_
- [DatasourceSpec](#datasourcespec)
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `basicAuth` _[BasicAuth](#basicauth)_ | BasicAuth provides username/password authentication configuration for the Perses client |  | Optional: \{\} <br /> |
| `oauth` _[OAuth](#oauth)_ | OAuth provides OAuth 2.0 authentication configuration for the Perses client |  | Optional: \{\} <br /> |
| `tls` _[TLS](#tls)_ | TLS provides TLS/SSL configuration for secure connections to Perses |  | Optional: \{\} <br /> |
| `kubernetesAuth` _[KubernetesAuth](#kubernetesauth)_ | KubernetesAuth enables Kubernetes native authentication for the Perses client |  | Optional: \{\} <br /> |


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
| `config` _[Datasource](#datasource)_ | Config specifies the Perses datasource configuration |  | Required: \{\} <br /> |
| `client` _[Client](#client)_ | Client specifies authentication and TLS configuration for the datasource |  | Optional: \{\} <br /> |
| `instanceSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#labelselector-v1-meta)_ | InstanceSelector selects Perses instances where this datasource will be created |  | Optional: \{\} <br /> |


#### KubernetesAuth







_Appears in:_
- [Client](#client)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ | Enable determines whether Kubernetes authentication is enabled for the Perses client |  | Optional: \{\} <br /> |


#### Metadata



Metadata to add to deployed pods



_Appears in:_
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `labels` _object (keys:string, values:string)_ | Labels are key/value pairs attached to pods |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations are key/value pairs attached to pods for non-identifying metadata |  | Optional: \{\} <br /> |


#### OAuth







_Appears in:_
- [Client](#client)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[SecretSourceType](#secretsourcetype)_ | Type specifies the source type for secret data (secret, configmap, or file) |  | Enum: [secret configmap file] <br />Required: \{\} <br /> |
| `name` _string_ | Name is the name of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | Optional: \{\} <br /> |
| `namespace` _string_ | Namespace is the namespace of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | Optional: \{\} <br /> |
| `clientIDPath` _string_ | ClientIDPath specifies the key name within the secret/configmap or filesystem path<br />(depending on SecretSource.Type) where the OAuth client ID is stored |  | Optional: \{\} <br /> |
| `clientSecretPath` _string_ | ClientSecretPath specifies the key name within the secret/configmap or filesystem path<br />(depending on SecretSource.Type) where the OAuth client secret is stored |  | Optional: \{\} <br /> |
| `tokenURL` _string_ | TokenURL is the OAuth 2.0 provider's token endpoint URL<br />This is a constant specific to each OAuth provider |  | MinLength: 1 <br />Required: \{\} <br /> |
| `scopes` _string array_ | Scopes specifies optional requested permissions for the OAuth token |  | Optional: \{\} <br /> |
| `endpointParams` _object (keys:string, values:string array)_ | EndpointParams specifies additional parameters to include in requests to the token endpoint |  | Optional: \{\} <br /> |
| `authStyle` _integer_ | AuthStyle specifies how the endpoint wants the client ID and client secret sent<br />The zero value means to auto-detect |  | Optional: \{\} <br /> |


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
| `config` _[Dashboard](#dashboard)_ | Config specifies the Perses dashboard configuration |  | Required: \{\} <br /> |
| `instanceSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#labelselector-v1-meta)_ | InstanceSelector selects Perses instances where this dashboard will be created |  | Optional: \{\} <br /> |


#### PersesDashboardStatus



PersesDashboardStatus defines the observed state of PersesDashboard



_Appears in:_
- [PersesDashboard](#persesdashboard)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest observations of the PersesDashboard resource state |  | Optional: \{\} <br /> |


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
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest observations of the PersesDatasource resource state |  | Optional: \{\} <br /> |


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
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest observations of the PersesGlobalDatasource resource state |  | Optional: \{\} <br /> |


#### PersesService



PersesService defines service configuration for Perses



_Appears in:_
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the Kubernetes Service resource<br />If not specified, a default name will be generated |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations are key/value pairs attached to the Service for non-identifying metadata |  | Optional: \{\} <br /> |


#### PersesSpec



PersesSpec defines the desired state of Perses



_Appears in:_
- [Perses](#perses)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `client` _[Client](#client)_ | Client specifies the Perses client configuration |  | Optional: \{\} <br /> |
| `config` _[PersesConfig](#persesconfig)_ | Config specifies the Perses server configuration |  | Optional: \{\} <br /> |
| `args` _string array_ | Args are extra command-line arguments to pass to the Perses server |  | Optional: \{\} <br /> |
| `containerPort` _integer_ | ContainerPort is the port on which the Perses server listens for HTTP requests |  | Maximum: 65535 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of desired pod replicas for the Perses deployment |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#resourcerequirements-v1-core)_ | Resources defines the compute resources configured for the container |  | Optional: \{\} <br /> |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector constrains pods to nodes with matching labels |  | Optional: \{\} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#toleration-v1-core) array_ | Tolerations allow pods to schedule onto nodes with matching taints |  | Optional: \{\} <br /> |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#affinity-v1-core)_ | Affinity specifies the pod's scheduling constraints |  | Optional: \{\} <br /> |
| `image` _string_ | Image specifies the container image that should be used for the Perses deployment |  | Optional: \{\} <br /> |
| `service` _[PersesService](#persesservice)_ | Service specifies the service configuration for the Perses instance |  | Optional: \{\} <br /> |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#probe-v1-core)_ | LivenessProbe specifies the liveness probe configuration for the Perses container |  | Optional: \{\} <br /> |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#probe-v1-core)_ | ReadinessProbe specifies the readiness probe configuration for the Perses container |  | Optional: \{\} <br /> |
| `tls` _[TLS](#tls)_ | TLS specifies the TLS configuration for the Perses instance |  | Optional: \{\} <br /> |
| `storage` _[StorageConfiguration](#storageconfiguration)_ | Storage configuration used by the StatefulSet | \{ size:1Gi \} | Optional: \{\} <br /> |
| `serviceAccountName` _string_ | ServiceAccountName is the name of the ServiceAccount to use for the Perses deployment or statefulset |  | Optional: \{\} <br /> |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#podsecuritycontext-v1-core)_ | PodSecurityContext holds pod-level security attributes and common container settings<br />If not specified, defaults to fsGroup: 65534 to ensure proper volume permissions for the nobody user |  | Optional: \{\} <br /> |
| `logLevel` _string_ | LogLevel defines the log level for Perses |  | Enum: [panic fatal error warning info debug trace] <br />Optional: \{\} <br /> |
| `logMethodTrace` _boolean_ | LogMethodTrace when true, includes the calling method as a field in the log<br />It can be useful to see immediately where the log comes from |  | Optional: \{\} <br /> |
| `provisioning` _[Provisioning](#provisioning)_ | Provisioning configuration for provisioning secrets |  | Optional: \{\} <br /> |


#### PersesStatus



PersesStatus defines the observed state of Perses



_Appears in:_
- [Perses](#perses)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest observations of the Perses resource state |  | Optional: \{\} <br /> |
| `provisioning` _[SecretVersion](#secretversion) array_ | Provisioning contains the versions of provisioning secrets currently in use |  | Optional: \{\} <br /> |


#### Provisioning



Provisioning configuration for provisioning secrets



_Appears in:_
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretRefs` _[ProvisioningSecret](#provisioningsecret) array_ | SecretRefs is a list of references to Kubernetes secrets used for provisioning sensitive data. |  | Optional: \{\} <br /> |


#### ProvisioningSecret







_Appears in:_
- [Provisioning](#provisioning)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referent.<br />This field is effectively required, but due to backwards compatibility is<br />allowed to be empty. Instances of this type with an empty value here are<br />almost certainly wrong.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names |  | Optional: \{\} <br /> |
| `key` _string_ | The key of the secret to select from.  Must be a valid secret key. |  |  |
| `optional` _boolean_ | Specify whether the Secret or its key must be defined |  | Optional: \{\} <br /> |


#### SecretSource



SecretSource configuration for a perses secret source



_Appears in:_
- [BasicAuth](#basicauth)
- [Certificate](#certificate)
- [OAuth](#oauth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[SecretSourceType](#secretsourcetype)_ | Type specifies the source type for secret data (secret, configmap, or file) |  | Enum: [secret configmap file] <br />Required: \{\} <br /> |
| `name` _string_ | Name is the name of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | Optional: \{\} <br /> |
| `namespace` _string_ | Namespace is the namespace of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | Optional: \{\} <br /> |


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


#### SecretVersion



SecretVersion represents a secret version



_Appears in:_
- [PersesStatus](#persesstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the provisioning secret |  | Required: \{\} <br /> |
| `version` _string_ | Version is the resource version of the provisioning secret |  | Required: \{\} <br /> |


#### StorageConfiguration



StorageConfiguration is the configuration used to create and reconcile PVCs



_Appears in:_
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `storageClass` _string_ | StorageClass specifies the StorageClass to use for PersistentVolumeClaims<br />If not specified, the default StorageClass will be used |  | Optional: \{\} <br /> |
| `size` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#quantity-resource-api)_ | Size specifies the storage capacity for the PersistentVolumeClaim<br />Once set, the size cannot be decreased (only increased if the StorageClass supports volume expansion) |  | Optional: \{\} <br /> |


#### TLS







_Appears in:_
- [Client](#client)
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ | Enable determines whether TLS is enabled for connections to Perses |  | Optional: \{\} <br /> |
| `caCert` _[Certificate](#certificate)_ | CaCert specifies the CA certificate to verify the Perses server's certificate |  | Optional: \{\} <br /> |
| `userCert` _[Certificate](#certificate)_ | UserCert specifies the client certificate and key for mutual TLS (mTLS) authentication |  | Optional: \{\} <br /> |
| `insecureSkipVerify` _boolean_ | InsecureSkipVerify determines whether to skip verification of the Perses server's certificate<br />Setting this to true is insecure and should only be used for testing |  | Optional: \{\} <br /> |


