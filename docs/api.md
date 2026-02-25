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
| `type` _[SecretSourceType](#secretsourcetype)_ | type specifies the source type for secret data (secret, configmap, or file) |  | Enum: [secret configmap file] <br />Required: \{\} <br /> |
| `name` _string_ | name is the name of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `namespace` _string_ | namespace is the namespace of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `username` _string_ | username is the username credential for basic authentication |  | MinLength: 1 <br />Required: \{\} <br /> |
| `passwordPath` _string_ | passwordPath specifies the key name within the secret/configmap or filesystem path<br />(depending on SecretSource.Type) where the password is stored |  | MinLength: 1 <br />Required: \{\} <br /> |


#### Certificate







_Appears in:_
- [TLS](#tls)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[SecretSourceType](#secretsourcetype)_ | type specifies the source type for secret data (secret, configmap, or file) |  | Enum: [secret configmap file] <br />Required: \{\} <br /> |
| `name` _string_ | name is the name of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `namespace` _string_ | namespace is the namespace of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `certPath` _string_ | certPath specifies the key name within the secret/configmap or filesystem path<br />(depending on SecretSource.Type) where the certificate is stored |  | MinLength: 1 <br />Required: \{\} <br /> |
| `privateKeyPath` _string_ | privateKeyPath specifies the key name within the secret/configmap or filesystem path<br />(depending on SecretSource.Type) where the private key is stored<br />Required for client certificates (UserCert), optional for CA certificates (CaCert) |  | Optional: \{\} <br /> |


#### Client







_Appears in:_
- [DatasourceSpec](#datasourcespec)
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `basicAuth` _[BasicAuth](#basicauth)_ | basicAuth provides username/password authentication configuration for the Perses client |  | Optional: \{\} <br /> |
| `oauth` _[OAuth](#oauth)_ | oauth provides OAuth 2.0 authentication configuration for the Perses client |  | Optional: \{\} <br /> |
| `tls` _[TLS](#tls)_ | tls provides TLS/SSL configuration for secure connections to Perses |  | Optional: \{\} <br /> |
| `kubernetesAuth` _[KubernetesAuth](#kubernetesauth)_ | kubernetesAuth enables Kubernetes native authentication for the Perses client |  | Optional: \{\} <br /> |


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
| `config` _[Datasource](#datasource)_ | config specifies the Perses datasource configuration |  | Required: \{\} <br /> |
| `client` _[Client](#client)_ | client specifies authentication and TLS configuration for the datasource |  | Optional: \{\} <br /> |
| `instanceSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#labelselector-v1-meta)_ | instanceSelector selects Perses instances where this datasource will be created |  | Optional: \{\} <br /> |


#### KubernetesAuth







_Appears in:_
- [Client](#client)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ | enable determines whether Kubernetes authentication is enabled for the Perses client |  | Optional: \{\} <br /> |


#### Metadata



Metadata to add to deployed pods



_Appears in:_
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `labels` _object (keys:string, values:string)_ | labels are key/value pairs attached to pods |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | annotations are key/value pairs attached to pods for non-identifying metadata |  | Optional: \{\} <br /> |


#### OAuth







_Appears in:_
- [Client](#client)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[SecretSourceType](#secretsourcetype)_ | type specifies the source type for secret data (secret, configmap, or file) |  | Enum: [secret configmap file] <br />Required: \{\} <br /> |
| `name` _string_ | name is the name of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `namespace` _string_ | namespace is the namespace of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `clientIDPath` _string_ | clientIDPath specifies the key name within the secret/configmap or filesystem path<br />(depending on SecretSource.Type) where the OAuth client ID is stored |  | Optional: \{\} <br /> |
| `clientSecretPath` _string_ | clientSecretPath specifies the key name within the secret/configmap or filesystem path<br />(depending on SecretSource.Type) where the OAuth client secret is stored |  | Optional: \{\} <br /> |
| `tokenURL` _string_ | tokenURL is the OAuth 2.0 provider's token endpoint URL<br />This is a constant specific to each OAuth provider |  | MinLength: 1 <br />Required: \{\} <br /> |
| `scopes` _string array_ | scopes specifies optional requested permissions for the OAuth token |  | Optional: \{\} <br /> |
| `endpointParams` _object (keys:string, values:string array)_ | endpointParams specifies additional parameters to include in requests to the token endpoint |  | Optional: \{\} <br /> |
| `authStyle` _integer_ | authStyle specifies how the endpoint wants the client ID and client secret sent<br />The zero value means to auto-detect |  | Optional: \{\} <br /> |


#### Perses



Perses is the Schema for the perses API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `perses.dev/v1alpha2` | | |
| `kind` _string_ | `Perses` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[PersesSpec](#persesspec)_ | spec is the desired state of the Perses resource |  | Optional: \{\} <br /> |
| `status` _[PersesStatus](#persesstatus)_ | status is the observed state of the Perses resource |  | Optional: \{\} <br /> |


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
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[PersesDashboardSpec](#persesdashboardspec)_ | spec is the desired state of the PersesDashboard resource |  | Required: \{\} <br /> |
| `status` _[PersesDashboardStatus](#persesdashboardstatus)_ | status is the observed state of the PersesDashboard resource |  | Optional: \{\} <br /> |


#### PersesDashboardSpec



PersesDashboardSpec defines the desired state of PersesDashboard



_Appears in:_
- [PersesDashboard](#persesdashboard)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `config` _[Dashboard](#dashboard)_ | config specifies the Perses dashboard configuration |  | Required: \{\} <br /> |
| `instanceSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#labelselector-v1-meta)_ | instanceSelector selects Perses instances where this dashboard will be created |  | Optional: \{\} <br /> |


#### PersesDashboardStatus



PersesDashboardStatus defines the observed state of PersesDashboard



_Appears in:_
- [PersesDashboard](#persesdashboard)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | conditions represent the latest observations of the PersesDashboard resource state |  | Optional: \{\} <br /> |


#### PersesDatasource



PersesDatasource is the Schema for the PersesDatasources API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `perses.dev/v1alpha2` | | |
| `kind` _string_ | `PersesDatasource` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[DatasourceSpec](#datasourcespec)_ | spec is the desired state of the PersesDatasource resource |  | Required: \{\} <br /> |
| `status` _[PersesDatasourceStatus](#persesdatasourcestatus)_ | status is the observed state of the PersesDatasource resource |  | Optional: \{\} <br /> |


#### PersesDatasourceStatus



PersesDatasourceStatus defines the observed state of PersesDatasource



_Appears in:_
- [PersesDatasource](#persesdatasource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | conditions represent the latest observations of the PersesDatasource resource state |  | Optional: \{\} <br /> |


#### PersesGlobalDatasource



PersesGlobalDatasource is the Schema for the PersesGlobalDatasources API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `perses.dev/v1alpha2` | | |
| `kind` _string_ | `PersesGlobalDatasource` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[DatasourceSpec](#datasourcespec)_ | spec is the desired state of the PersesGlobalDatasource resource |  | Required: \{\} <br /> |
| `status` _[PersesGlobalDatasourceStatus](#persesglobaldatasourcestatus)_ | status is the observed state of the PersesGlobalDatasource resource |  | Optional: \{\} <br /> |


#### PersesGlobalDatasourceStatus



PersesGlobalDatasourceStatus defines the observed state of PersesGlobalDatasource



_Appears in:_
- [PersesGlobalDatasource](#persesglobaldatasource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | conditions represent the latest observations of the PersesGlobalDatasource resource state |  | Optional: \{\} <br /> |


#### PersesService



PersesService defines service configuration for Perses



_Appears in:_
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name is the name of the Kubernetes Service resource<br />If not specified, a default name will be generated |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | annotations are key/value pairs attached to the Service for non-identifying metadata |  | Optional: \{\} <br /> |


#### PersesSpec



PersesSpec defines the desired state of Perses



_Appears in:_
- [Perses](#perses)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[Metadata](#metadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `client` _[Client](#client)_ | client specifies the Perses client configuration |  | Optional: \{\} <br /> |
| `config` _[PersesConfig](#persesconfig)_ | config specifies the Perses server configuration |  | Optional: \{\} <br /> |
| `args` _string array_ | args are extra command-line arguments to pass to the Perses server |  | Optional: \{\} <br /> |
| `containerPort` _integer_ | containerPort is the port on which the Perses server listens for HTTP requests |  | Maximum: 65535 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `replicas` _integer_ | replicas is the number of desired pod replicas for the Perses deployment |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#resourcerequirements-v1-core)_ | resources defines the compute resources configured for the container |  | Optional: \{\} <br /> |
| `nodeSelector` _object (keys:string, values:string)_ | nodeSelector constrains pods to nodes with matching labels |  | Optional: \{\} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#toleration-v1-core) array_ | tolerations allow pods to schedule onto nodes with matching taints |  | Optional: \{\} <br /> |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#affinity-v1-core)_ | affinity specifies the pod's scheduling constraints |  | Optional: \{\} <br /> |
| `image` _string_ | image specifies the container image that should be used for the Perses deployment |  | Optional: \{\} <br /> |
| `service` _[PersesService](#persesservice)_ | service specifies the service configuration for the Perses instance |  | Optional: \{\} <br /> |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#probe-v1-core)_ | livenessProbe specifies the liveness probe configuration for the Perses container |  | Optional: \{\} <br /> |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#probe-v1-core)_ | readinessProbe specifies the readiness probe configuration for the Perses container |  | Optional: \{\} <br /> |
| `tls` _[TLS](#tls)_ | tls specifies the TLS configuration for the Perses instance |  | Optional: \{\} <br /> |
| `storage` _[StorageConfiguration](#storageconfiguration)_ | storage configuration used by the StatefulSet |  | Optional: \{\} <br /> |
| `serviceAccountName` _string_ | serviceAccountName is the name of the ServiceAccount to use for the Perses deployment or statefulset |  | Optional: \{\} <br /> |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#podsecuritycontext-v1-core)_ | podSecurityContext holds pod-level security attributes and common container settings<br />If not specified, defaults to fsGroup: 65534 to ensure proper volume permissions for the nobody user |  | Optional: \{\} <br /> |
| `logLevel` _string_ | logLevel defines the log level for Perses |  | Enum: [panic fatal error warning info debug trace] <br />Optional: \{\} <br /> |
| `logMethodTrace` _boolean_ | logMethodTrace when true, includes the calling method as a field in the log<br />It can be useful to see immediately where the log comes from |  | Optional: \{\} <br /> |
| `provisioning` _[Provisioning](#provisioning)_ | provisioning configuration for provisioning secrets |  | Optional: \{\} <br /> |


#### PersesStatus



PersesStatus defines the observed state of Perses



_Appears in:_
- [Perses](#perses)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | conditions represent the latest observations of the Perses resource state |  | Optional: \{\} <br /> |
| `provisioning` _[SecretVersion](#secretversion) array_ | provisioning contains the versions of provisioning secrets currently in use |  | Optional: \{\} <br /> |


#### Provisioning



Provisioning configuration for provisioning secrets



_Appears in:_
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretRefs` _[ProvisioningSecret](#provisioningsecret) array_ | secretRefs is a list of references to Kubernetes secrets used for provisioning sensitive data. |  | Optional: \{\} <br /> |


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
| `type` _[SecretSourceType](#secretsourcetype)_ | type specifies the source type for secret data (secret, configmap, or file) |  | Enum: [secret configmap file] <br />Required: \{\} <br /> |
| `name` _string_ | name is the name of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `namespace` _string_ | namespace is the namespace of the Kubernetes Secret or ConfigMap resource<br />Required when Type is "secret" or "configmap", ignored when Type is "file" |  | MinLength: 1 <br />Optional: \{\} <br /> |


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
| `name` _string_ | name is the name of the provisioning secret |  | MinLength: 1 <br />Required: \{\} <br /> |
| `version` _string_ | version is the resource version of the provisioning secret |  | MinLength: 1 <br />Required: \{\} <br /> |


#### StorageConfiguration



StorageConfiguration is the configuration used to create and reconcile PVCs



_Appears in:_
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#emptydirvolumesource-v1-core)_ | emptyDir to use for ephemeral storage.<br />When set, data will be lost when the pod is deleted or restarted.<br />Mutually exclusive with PersistentVolumeClaimTemplate. |  | Optional: \{\} <br /> |
| `pvcTemplate` _[PersistentVolumeClaimSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#persistentvolumeclaimspec-v1-core)_ | pvcTemplate is the template for PVCs that will be created.<br />Mutually exclusive with EmptyDir. |  | Optional: \{\} <br /> |


#### TLS







_Appears in:_
- [Client](#client)
- [PersesSpec](#persesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ | enable determines whether TLS is enabled for connections to Perses |  | Optional: \{\} <br /> |
| `caCert` _[Certificate](#certificate)_ | caCert specifies the CA certificate to verify the Perses server's certificate |  | Optional: \{\} <br /> |
| `userCert` _[Certificate](#certificate)_ | userCert specifies the client certificate and key for mutual TLS (mTLS) authentication |  | Optional: \{\} <br /> |
| `insecureSkipVerify` _boolean_ | insecureSkipVerify determines whether to skip verification of the Perses server's certificate<br />Setting this to true is insecure and should only be used for testing |  | Optional: \{\} <br /> |


