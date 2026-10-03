# Report publication

The root [composite action](../action.yml) accepts an existing UTF-8 Markdown file. It appends that file to the job summary and uses `marocchino/sticky-pull-request-comment` to create or update a PR comment. It does not run benchmarks or require the planned Go CLI.

## Inputs and outputs

| Input | Default | Behavior |
| --- | --- | --- |
| `report-path` | Required | One regular file; not a glob and not a symlink |
| `header` | `benchmark-report` | Stable identity for this report; 1 to 100 ASCII letters, digits, dots, underscores, colons, or hyphens |
| `publish` | `true` | Request comment publication when the event is eligible |
| `summary` | `true` | Append the report to the current job summary |
| `github-token` | `github.token` | Token used only by the upstream comment action |

Boolean inputs accept only `true` or `false`. Missing, blank, or invalid UTF-8 reports fail even if publication is disabled. For publication, the report plus upstream marker must fit a 60,000-byte UTF-8 budget. Oversized files fail with a request for a shorter report linked to its full artifact. Summary-only execution has no wrapper-imposed comment size limit.

The action copies the file to a generated temporary filename before passing it upstream. This preserves exact file selection because the dependency's `path` input accepts glob patterns.

| Output | Meaning |
| --- | --- |
| `publication` | On success: `completed`, `disabled`, `fork`, `dependabot`, or `unsupported-event`. `completed` includes an unchanged comment that required no write. Outputs are not a success signal if the action fails. |
| `previous-comment-id` | Existing ID returned by the dependency, if found |
| `created-comment-id` | Newly created ID returned by the dependency, if created |

## Events and permissions

Comment publication is limited to `pull_request` events from the same repository, excluding Dependabot. Forks, Dependabot, push events, and `pull_request_target` skip the comment but can still receive a summary. `publish: false` explicitly selects summary-only behavior. A read-only token on an otherwise eligible event is an error from the dependency, not a successful skip.

The caller grants `pull-requests: write` only to the publishing job. Benchmark execution belongs in a separate job with read-only repository permissions. Download the completed report artifact into the publishing job and invoke this action from a pinned revision. Do not execute a PR checkout in that job. Upload complete reports and raw inputs in the producing workflow; this action does not upload artifacts.

Set workflow concurrency per PR and header, with `cancel-in-progress: true`. This reduces overlapping updates but does not provide an atomic freshness check. Do not manually rerun superseded revisions expecting the action to preserve a newer comment.

## Dependency contract

The action pins [sticky-pull-request-comment v3.0.5](https://github.com/marocchino/sticky-pull-request-comment/tree/5770ad5eb8f42dd2c4f34da00c94c5381e49af88) to commit `5770ad5eb8f42dd2c4f34da00c94c5381e49af88`. Its Node 24 runtime requires a compatible Actions runner. The wrapper itself uses Bash and standard utilities available on GitHub-hosted Linux and macOS runners.

It passes `header`, the prepared file path, the token, `skip_unchanged: true`, and `ignore_empty: false`. Append, recreate, hide, and delete modes remain disabled. Lookup, pagination, author matching, and API error handling belong to the dependency. The first matching non-minimized comment by the authenticated author is selected; duplicate matches are not diagnosed by this wrapper. See the pinned [lookup implementation](https://github.com/marocchino/sticky-pull-request-comment/blob/5770ad5eb8f42dd2c4f34da00c94c5381e49af88/src/comment.ts).

Updating the pin requires review of marker format, author matching, input/output names, runtime requirements, and error behavior. Do not copy the upstream implementation into this repository.

## Migrating existing comments

For the default header, the dependency appends this exact marker:

```html
<!-- Sticky Pull Request Commentbenchmark-report -->
```

The existing consumer markers, `<!-- go-socket.io:benchmark-comparison -->` and `<!-- ytsaurus-rs:criterion-benchmark-comparison -->`, do not match this format. Passing their text as `header` does not adopt an old comment.

The default migration stops the previous publisher and creates one new managed comment on each already-open PR. Subsequent runs update that new comment. Historical comments remain untouched. New PRs receive only the managed comment.

If retaining an existing comment ID is required, make a one-time authorized edit through the original bot identity to append the exact sticky marker for the chosen header before switching publishers. Keep the token identity and header stable afterward. This migration is not an ongoing feature of the Go generator or wrapper.

## Verification

Run `bash tests/prepare-comment.sh` to check file validation, literal filenames, event eligibility, summary behavior, UTF-8, and comment size handling without GitHub requests. CI also exercises the composite action in summary-only mode on Linux and macOS.

Live create/update behavior must be checked in an explicitly authorized trial PR before release. No live comment trial is implied by the local checks.
