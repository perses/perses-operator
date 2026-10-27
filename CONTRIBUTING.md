# Contributing

As Perses operator is still a work in progress, the contribution process is still evolving.

We are using GitHub as our main development and discussion forum.

- All PRs should go there.
- We use pull requests and issues for tracking the development of features that are either uncontroversial and/or small
  and don't need much up-front discussion.
- If you are thinking about contributing something more involved, you can use
  the [GitHub discussions](https://github.com/perses/perses-operator/discussions) feature for design discussions before sending a
  pull request or creating a feature request issue.
- Be sure to add [DCO signoffs](https://github.com/probot/dco#how-it-works) to all of your commits.

If you are unsure about what to do, and you are eager to contribute, you can reach us on the development
channel [#perses-dev](https://cloud-native.slack.com/messages/C07KQR95WBE) on [CNCF slack](https://slack.cncf.io/).

- For setting up your development environment, see the [Developer Guide](docs/dev.md).
- For running tests, see the [Testing Guide](docs/testing.md).

## Opening a PR

To help during the release process, we created a script that generates the changelog based on the git history.

To make it work correctly, commit or PR's title should follow the following naming convention:

`[<catalog_entry>] <commit message>`

where `catalog_entry` can be :

- `FEATURE`
- `ENHANCEMENT`
- `BUGFIX`
- `BREAKINGCHANGE`
- `DOC`
- `IGNORE` - Changes that should not generate entries in the changelog. Primarily used for internal tooling changes that
  do not impact consumers.

This catalog entry will indicate the purpose of your PR.

PRs targeting `main` are merged through the merge queue using squash and merge.
Keep each PR focused on a single fix or feature so its commits become one commit
in the history used to generate the changelog.

Release preparation PRs follow the same process. As described in the
[release guide](RELEASE.md), create the release tag on `main` after the PR is merged.
