# Perses Operator Release

The release is driven by the **Release** GitHub Actions workflow
(`.github/workflows/release.yaml`). It opens a PR against `main` that bumps
`VERSION`, regenerates `CHANGELOG.md`, regenerates the OLM bundle with the
correct `replaces` chain, and regenerates the installer manifest. Publishing
runs after the PR is merged and the new `v*` tag is pushed.

## Flow overview

```
1. User: Actions ▸ Release ▸ Run workflow (tag, version-replace)
   │
   ▼
2. release.yaml            →  opens PR: chore: update VERSION to <tag>
   │                           (VERSION + CHANGELOG.md + bundle/ + bundle.yaml)
   ▼
3. User: review & merge PR into main
   │
   ▼
4. User: push v<tag> tag on main
   │
   ▼
5. ci.yaml (on v* tag)     →  images (operator/bundle/catalog) + GitHub release
   │
   ▼
6. publish-operator-hub.yaml (on release: published)
                           →  PR against k8s-operatorhub/community-operators
```

## 1. Trigger the release workflow

In GitHub: **Actions** → **Release** → **Run workflow**.

Inputs:

| Input             | Example | Meaning                                                            |
|-------------------|---------|--------------------------------------------------------------------|
| `tag`             | `0.4.0` | New version to release (no `v` prefix).                            |
| `version-replace` | `0.3.2` | Previously shipped version. Drives OLM `spec.replaces` in the CSV. |

The workflow:

- Writes `<tag>` to `VERSION`.
- Runs `make generate-changelog` — regenerates `CHANGELOG.md` for the new
  release version.
- Runs `make bundle VERSION_REPLACED=<version-replace>` — regenerates `bundle/manifests/*`
  (CSV `spec.version`, `spec.replaces: perses-operator.v<version-replace>`) and the
  jsonnet files under `jsonnet/generated` and `jsonnet/examples`.
- Runs `make build-installer` — regenerates the root `bundle.yaml` installer.
- Opens a PR titled `chore: update VERSION to <tag>` against `main` on branch
  `chore-version-<tag>`

Re-running the workflow with the same `tag` updates the existing PR branch.

> ⚠️ `version-replace` must equal the version currently shipped on OperatorHub

## 2. Review and merge the PR

- Review the diff — specifically:
  - `VERSION`
  - `CHANGELOG.md` (clean up/categorize any `UNKNOWN` entries)
  - `bundle/manifests/perses-operator.clusterserviceversion.yaml`
    (`spec.version`, `spec.replaces`)
  - CRDs and jsonnet generated files (if CRDs changed)
  - `bundle.yaml` (root installer manifest)
- Approve and merge the PR into `main`.

> Note: the PR is created using the `BOT_TOKEN` PAT (the same token used by
> `publish-operator-hub.yaml`). This is required so downstream `pull_request`
> workflows (`go.yaml`, `ci.yaml`) fire on the auto-created PR. `BOT_TOKEN`
> is a configuration prerequisite (see
> [One-time setup prerequisites](#one-time-setup-prerequisites)).

## 3. Tag the release

Pull the merged `main` and push the tag:

```bash
git fetch origin
git checkout main
git pull
git tag -a v<tag> -m "v<tag>"
git push origin v<tag>
```

> ⚠️ Do **not** use GitHub's "Create release" UI. `ci.yaml` drives `goreleaser`
> which publishes the GitHub release itself — creating one manually would race.

Pushing the `v*` tag triggers `ci.yaml`, which:

- Builds and pushes the operator container image to Docker Hub and Quay.
- Builds and pushes the bundle and catalog images (`v<tag>` tag on both registries).
- Runs `goreleaser` to publish the GitHub release with the Go binaries.

## 4. Publish to OperatorHub (automated)

When a GitHub release is published, the **Publish operator to OperatorHub**
workflow (`publish-operator-hub.yaml`) runs — calling the reusable
`operator-hub-release.yaml` workflow — and opens a pull request against
[k8s-operatorhub/community-operators](https://github.com/k8s-operatorhub/community-operators/pulls)
(OperatorHub.io).

You can also run it manually from **Actions → Create OperatorHub pull request → Run workflow**:

- **release_ref** (required): release tag to publish, for example `v0.4.0`
- **version_replaced** (optional): previous version already on OperatorHub, for example `0.1.1`. Leave empty to auto-detect the latest version in the catalog.
- **org** / **repo**: default to `k8s-operatorhub` / `community-operators`

The workflow builds the bundle from the release tag, sets `replaces` from the latest version on OperatorHub (or from `version_replaced`), and aligns both `containerImage` and the manager deployment image with the release image. If auto-detection cannot find a prior catalog version, the workflow fails and you must set `version_replaced` explicitly.

A maintainer should monitor the community-operators PR and address any CI feedback. OperatorHub reviewers listed in `bundle/ci.yaml` must approve before the PR can merge.

### One-time setup prerequisites

Before the OperatorHub automation can run, the following must be configured once:

1. **Bot account**: the `persesbot` GitHub account must have a fork of
   [k8s-operatorhub/community-operators](https://github.com/k8s-operatorhub/community-operators).
2. **GitHub secret**: a Personal Access Token (`repo` scope) for the bot account
   must be added as `BOT_TOKEN` in the repository settings.
3. **CLA/DCO**: the bot account should sign the CNCF CLA or Linux Foundation DCO
   if required.

## 5. Update the Helm chart

After the release is published, update the
[Perses Operator Helm chart](https://github.com/perses/helm-charts/tree/main/charts/perses-operator)
in the [`perses/helm-charts`](https://github.com/perses/helm-charts) repository.
Follow the
[Bumping perses-operator Version](https://github.com/perses/helm-charts/blob/main/DEVELOPER_GUIDE.md#bumping-perses-operator-version)
guide.
