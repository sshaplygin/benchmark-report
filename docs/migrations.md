# Consumer migration proposals

These patches replace repository-specific report generation and publication with the shared actions. They retain benchmark execution in each consumer. Review each patch against its recorded base commit before applying it elsewhere.

| Consumer | Proposal | Captured comparison |
| --- | --- | --- |
| go-socket.io | [Patch, behavior changes, verification, rollback](../migrations/go-socket.io/README.md) | PR 15: 13 timing identities, 39 metric identities |
| ytsaurus-rs | [Patch, behavior changes, verification, rollback](../migrations/ytsaurus-rs/README.md) | Four Criterion suites: 28 timing identities |

Both proposals pin the generator and publisher to `d64cc0a04a3ceba58f2492a3cf87b99dce3b9d0c`. That generator maps to binary version `0.1.0`. Its release assets are not published, so the proposed workflows cannot yet install the generator. Local verification uses candidate binaries built from the reviewed source.

## Rollout prerequisites

1. Complete release review, select the project license, and obtain authorization to publish the binary assets required by the [installer](report-action.md).
2. Obtain authorization to open consumer PRs, execute their workflows, and publish trial comments. The prepared patches and local checks do not perform those actions.
3. Recheck each consumer's current base, apply its patch in a clean checkout, and run its documented checks. If upstream changed, review the resulting diff again before opening its PR.
4. Run each consumer workflow twice on an eligible same-repository PR. Inspect the full artifact and summary, then confirm the second publication updates the comment with the selected header. Confirm fork and Dependabot runs retain reports without attempting comment publication.
5. Accept each migration only after its [Stage 7 review criteria](implementation-plan.md#stage-7-consumer-migration-trial) pass. Keep the legacy scripts available until then; use the consumer's rollback instructions if the trial fails.

The patches use new sticky headers. Handling older comments follows [the existing-comment policy](publication.md#migrating-existing-comments); no cleanup or ID adoption is performed by these proposals.

Local verification covers archived numeric results, input completeness, configuration, workflow syntax, and patch reversal. It cannot establish live consumer permissions or comment updates. Acceptance status and evidence belong in the implementation PR's completion record.
