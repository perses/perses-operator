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

In the usual workflow, all PRs are squashed. There is two exceptions to this rule:

1. During the release process, the release branch is merge back in the `main` branch. To avoid to lose the commit
   message that holds the tag, this kind of PR **MUST** be merged and not squashed.

2. In case your PR contains multiple kind of changes (aka, feature, bugfix ..etc.) and you took care about having
   different commit following the convention described above, then the PR will be merged and not squashed. Like that we
   are preserving the works you did and the effort you made when creating meaningful commit.
