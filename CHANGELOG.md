## 0.3.0 / 2026-03-03

- [FEATURE] Add v1alpha2 resources and conversion webhooks (#172)
- [FEATURE] Add PersesGlobalDatasource CRD (#174)
- [FEATURE] Mark old CRD versions as deprecated (#240)
- [FEATURE] Add ability to turn off the webhooks servers (#243)
- [FEATURE] Add status condition reasons for resource failures (#247)
- [FEATURE] Add logLevel and logMethodTrace fields to Perses CR (#259)
- [FEATURE] Add resources configuration to Perses (#263)
- [FEATURE] Auto-generate API reference documentation from Go types (#264)
- [FEATURE] Add custom Prometheus metrics for operator (#268)
- [FEATURE] Add provisioning secrets for Perses configuration (#274)
- [FEATURE] Add kuttl e2e tests and split unit/integration test targets (#277)
- [FEATURE] Add Prometheus alerting rules (#297)
- [FEATURE] Add e2e tests for global datasource, emptyDir storage, and resource updates (#299)
- [FEATURE] Add writable emptyDir volume for plugin storage (#301)
- [FEATURE] Support user-defined volumes and volumeMounts on Perses CR (#302)
- [FEATURE] Update to latest Perses release (#305)
- [FEATURE] Integrate kube-api-linter for API type validation (#296)
- [FEATURE] Support resource tags via perses.dev/tags annotation (#321)
- [ENHANCEMENT] Add kubebuilder validation markers for required API fields (#250)
- [ENHANCEMENT] Refactor reconcilers to avoid status update conflicts (#257)
- [ENHANCEMENT] Add kubebuilder validations for containerPort (#258)
- [ENHANCEMENT] Add API documentation to CRD types and fields (#265)
- [ENHANCEMENT] Allow to reconcile a Dashboard or datasource in a specific Perses instance (#287)
- [ENHANCEMENT] Define default Perses operand image version in a single place (#303)
- [ENHANCEMENT] Remove PERSES_IMAGE env var and use DefaultPersesImage (#312)
- [ENHANCEMENT] Migrate kustomize vars to replacements and add installer-check to CI (#317)
- [ENHANCEMENT] Rename make target install to install-crds (#214)
- [ENHANCEMENT] Normalize tags to lowercase and add integration test (#321)
- [BUGFIX] Fix setting finalizer (#197)
- [BUGFIX] Fix service label selectors reconciliation (#171)
- [BUGFIX] Set cluster scope for global datasources CR (#238)
- [BUGFIX] Correct logging issues in dashboard and datasource controllers (#254)
- [BUGFIX] Correct logging issues across controllers (#252)
- [BUGFIX] Set degraded condition to false when reconciliation succeeds (#279)
- [BUGFIX] Sync existing CRs to new Perses instances (#285)
- [BUGFIX] Fix perses securitycontext issue (#211)
- [BUGFIX] Fix goreleaser job and clean up (#316)
- [BREAKINGCHANGE] Add support for emptyDir file storage (#244). The `StorageConfiguration` fields `storageClass` and `size` have been replaced by `emptyDir` and `pvcTemplate`; existing Perses CRs using the old storage fields must be updated.
- [BREAKINGCHANGE] Rename BasicAuth field from password_path to passwordPath (#267). v1alpha2 users must update their CRs to use `passwordPath`; v1alpha1 CRs are unaffected as the conversion webhook translates automatically.
- [BREAKINGCHANGE] Refactor v1alpha2 API types to follow Kubernetes conventions (#282). Optional primitive fields are now pointers; existing v1alpha2 manifests may need to be re-applied.
- [BREAKINGCHANGE] Add CEL validation for SecretSource conditional requirements (#290). SecretSource with `type: secret` or `type: configmap` now requires `name` and `namespace`; previously accepted manifests missing these fields will be rejected at admission time.
- [DOC] Fix step order to use the conversion webhook (#249)
- [DOC] Add secrets section (#231)
- [DOC] Improve Datasource Secret documentation (#231)
- [DOC] Add openshift thanos querier instructions (#217)
- [DOC] Improve developer guide, testing docs, and README (#309)
- [DOC] Add GitHub Actions status badges and slack link to README (#307)
- [DOC] Add Overview, Project Status, CRDs, and Maintainers sections to README (#308)
- [DOC] Reorganize Documentation section in README (#314)
- [DOC] Add perses-operator logo (#324)
- [DOC] Add Helm chart installation instructions to README (#327)

## 0.2.0 / 2025-06-12

We've temporarily rolled back a CRD version upgrade due to certificate issues, but we're actively working on a conversion webhook for the next major release. This will enable seamless upgrades for both dashboard and Perses configurations. Currently, manual adjustments are needed for Perses 0.51, but the conversion webhook will mature our operator to Level 2 capabilities, automating these processes in the future.

- [BREAKINGCHANGE] bump perses version to v0.51.0 (#165)
- [ENHANCEMENT] Enable Ginkgo verbose mode in the test suite instead (#164)
- [BUGFIX] Add Perses test for storage size and reduce reconciliation errors (#160)
- [DOC] add release docs and scripts

## 0.1.12 / 2025-06-06

- [FEATURE] allow manager to pass k8s token from service account
- [BUGFIX] set defaults for storage options when not provided
- [BUGFIX] fix stateful set changes check for reconciliation
- [BUGFIX] fix service changes check for reconciliation
- [BUGFIX] fix configmap change check for reconciliation
- [BUGFIX] remove version label
- [BUGFIX] fix default operand image version
- [DOC] fix development docs namespace
- [DOC] add api and dev documentation

## 0.1.11 / 2025-05-27

- [FEATURE]: Add shortNames to Perses CRDs (#115)
- [FEATURE]: Allow configuration of StatefulSet PVC (#123)
- [FEATURE]: Allow cross namespace secret/configmap reference for Certificates (#127)
- [FEATURE]: start pprof endpoint on localhost (#130)
- [FEATURE] allow manager to pass k8s token from service account (#151)
- [FEATURE]: allow to set a service account name for operands (#148)
- [FEATURE]: Add Basic Auth and OAuth secret types to the PersesDatasource controller (#131)
- [FEATURE] Add perses-operator libsonnet (#140)
- [ENHANCEMENT] improve checkformat makefile command (#132)
- [BUGFIX]: Use secret and configmap data in client tls configuration when provided (#124)
- [BUGFIX] remove version label (#143)
- [BUGFIX] fix ConfigMap change check for reconciliation (#144)
- [BUGFIX] fix service changes check for reconciliation (#145)
- [BUGFIX] Fix stateful set changes check (#146)
- [DOC] add api and dev documentation (#110)
