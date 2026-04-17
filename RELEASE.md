# Perses Operator Release

## 1. Prepare your release

- fetch the latest changes from `main`

- Create a release branch named `release/v<major>.<minor>` from the `main` branch.

> ⚠️ Release candidates and patch releases for any given major or minor release happen in the same `release/v<major>.<minor>` branch. Do not create `release/<version>` for patch or release candidate releases.

- Create a branch based on the release branch you just created in the step above in your fork. The branch should use the naming pattern `<yourname>/release-v<major>.<minor>.<patch>`.
- Update the file `VERSION` with the new version to be created.
- Generate `CHANGELOG.md` updates based on git history:

  ```bash
  make generate-changelog
  ```

- Regenerate bundle, jsonnet, and installer files, and verify they are up to date:

  ```bash
  make bundle-check
  make installer-check
  ```

- Review the generated `CHANGELOG.md` for valid output. Things to check include:
  - Entries in the `CHANGELOG.md` are meant to be in this order:
    * `[FEATURE]`
    * `[ENHANCEMENT]`
    * `[BUGFIX]`
    * `[BREAKINGCHANGE]`
    * `[DOC]`
  - Entries that map to a pull request should include a pull request number.
  - As we have many libraries we publish, it's better if you also put a clear indication about what library is affected by
    these changes.
  - Consumers understand how to handle breaking changes either through the messaging in the changelog or through the linked pull requests.
- Push the branch to Github and create a pull request with the release branch as the base. This gives others the opportunity to chime in on the release,
  in general, and on the addition to the changelog, in particular.
  - It's also helpful to drop a link to the release PR in #perses-dev on the CNCF Slack to get extra visibility.
- Address any necessary feedback.
- Once the pull request is approved, merge it into the release branch.

## 2. Create release tag and validate release

- Pull down the latest updates to the release branch on your local machine to ensure you have the updates from the previous step.
- Tag the new release via the following commands:

  ```bash
  git checkout release/v<major>.<minor>
  export GIT_REMOTE_UPSTREAM=origin # change if your upstream remote differs
  make tag
  git push $GIT_REMOTE_UPSTREAM v<major>.<minor>.<patch>
  ```

Once a tag is created, an automated release process for this tag is triggered via Github Actions. This automated process includes:

- Building new go binaries and docker images.
- Publishing the docker images to Docker Hub.
- Creating a new Github release that uses the changelog as the release notes and provides tarballs with the latest go binaries.

## 3. Merge the release into `main`

It can be helpful to leave the release branch up for a little while in case we need to create a patch release to address bugs or minor issues with the release you just made.

Once the release branch is no longer needed, you should open a new PR based on `main` to merge those changes. When this PR is approved, merge it into `main` :warning: **using the "merge pull request" option, not "squash and merge"** (the latter would delete the commit needed for the release tag, which can lead to problems).

## 4. Publish to OperatorHub (automated)

When the GitHub release is published, a workflow automatically creates a pull request to submit the new operator version to [k8s-operatorhub/community-operators](https://github.com/k8s-operatorhub/community-operators/pulls) (OperatorHub.io).

A maintainer should monitor the PR and address any CI feedback from the community-operators repository.

### One-time setup prerequisites

Before the automation can run, the following must be configured once:

1. **Bot account**: The `persesbot` GitHub account must have a fork of [k8s-operatorhub/community-operators](https://github.com/k8s-operatorhub/community-operators).
2. **GitHub secret**: A Personal Access Token with `repo` scope for the bot account must be added as `PERSESBOT_GITHUB_TOKEN` in the repository settings.
3. **CLA/DCO**: The bot account should sign the CNCF CLA or Linux Foundation DCO if required.

## 5. Update the Helm chart

After the release is published, update the [Perses Operator Helm chart](https://github.com/perses/helm-charts/tree/main/charts/perses-operator) in the [perses/helm-charts](https://github.com/perses/helm-charts) repository. Follow the [Bumping perses-operator Version](https://github.com/perses/helm-charts/blob/main/DEVELOPER_GUIDE.md#bumping-perses-operator-version) guide.
