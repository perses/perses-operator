# Perses Operator Release

### 1. Prepare your release

- fetch the latest changes from `main`
- Create a release branch named `release/v<major>.<minor>` from the `main` branch.
> ⚠️ Release candidates and patch releases for any given major or minor release happen in the same `release/v<major>.<minor>` branch. Do not create `release/<version>` for patch or release candidate releases.
- Create a branch based on the release branch you just created in the step above. The branch should use the naming pattern `<yourname>/release-v<major>.<minor>.<patch>`.
- Update the file `VERSION` with the new version to be created.
- Generate `CHANGELOG.md` updates based on git history:

  ```bash
  make generate-changelog
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

### 2. Create release tag and validate release

- Pull down the latest updates to the release branch on your local machine to ensure you have the updates from the previous step.
- Tag the new release via the following commands:

  ```bash
  git checkout release/v<major>.<minor>
  make tag
  git push origin v<major>.<minor>
  ```

Once a tag is created, an automated release process for this tag is triggered via Github Actions. This automated process includes:
- Building new go binaries and docker images.
- Publishing the docker images to Docker Hub.
- Creating a new Github release that uses the changelog as the release notes and provides tarballs with the latest go binaries.

### 3. Merge the release into `main`

It can be helpful to leave the release branch up for a little while in case we need to create a patch release to address bugs or minor issues with the release you just made.

Once the release branch is no longer needed, you should open a new PR based on `main` to merge those changes. When this PR is approved, merge it into `main` :warning: **using the "merge pull request" option, not "squash and merge"** (the latter would delete the commit needed for the release tag, which can lead to problems).
