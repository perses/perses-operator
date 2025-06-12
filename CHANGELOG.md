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
