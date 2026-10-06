# History integration

[examples/history.yml](../examples/history.yml) is an optional primary-branch template, not a deployed consumer workflow. It builds the CLI from the full commit selected by `BENCHREPORT_SOURCE_SHA` and expects `.github/benchmark-history.json` with history enabled and default direction filenames. Select the consumer's primary branch, benchmark command, environment, and metrics before use. Export settings are defined in [Configuration](configuration.md#history-export); JSON fields and numeric limits are defined in [Contracts](contracts.md#history-serialization-and-outputs).

The external dependency is [github-action-benchmark v1.22.2](https://github.com/benchmark-action/github-action-benchmark/tree/4322e5726e6334590d251fc4f92bec0efafc45dc), pinned to commit `4322e5726e6334590d251fc4f92bec0efafc45dc` rather than its annotated tag object. Its [inputs](https://github.com/benchmark-action/github-action-benchmark/blob/4322e5726e6334590d251fc4f92bec0efafc45dc/action.yml) select the two custom direction parsers. Retain the explicit comment, summary, and alert switches in the example.

The consumer must initialize `gh-pages` and configure GitHub Pages to serve its benchmark directory before enabling publication. Confirm that the chosen repository token can update that branch and that the selected Pages deployment mechanism runs for those updates; repository settings and branch protection may require a different permitted token or a separate deployment workflow. The local test below does not verify remote permissions or deployment. All workflows that update this history branch must share its concurrency key. GitHub concurrency prevents overlapping writers but does not guarantee queued-run ordering or preserve every pending run; this example is not a guarantee that every push becomes a history point.

The computation job has read-only repository permission. The publisher downloads only the generated direction JSON and invokes the pinned external action; it does not build the CLI or execute benchmark scripts. Both jobs reject non-primary-branch events. Series names include direction, execution environment, toolchain, and estimator; update both names when their profile changes. Renderer titles, units, and row filters are presentation settings and do not define a history profile.

## Local verification

[tests/history](../tests/history) runs the unmodified pinned action entry point in a disposable local repository with no remotes or credentials. `external-data-json-path` selects local JSON storage, `auto-push` is false, and a synthetic push payload supplies commit metadata without an API request. The fixture has two successive normalized Go runs, a separate Go toolchain profile, and a Criterion run. It verifies two appended points per original Go direction, profile isolation, exact exported values after upstream parsing/storage, and Criterion `range` plus `extra` preservation. The fixtures are synthetic benchmark text; no workload is executed and no remote history is updated.

Go 1.25 or later, Git, and Node 24 are test prerequisites. Acquire the pinned upstream distribution once; it includes its runtime dependencies, so no npm install is needed:

```sh
git clone --depth 1 --branch v1.22.2 https://github.com/benchmark-action/github-action-benchmark.git /tmp/benchreport-history-upstream
BENCHREPORT_HISTORY_UPSTREAM=/tmp/benchreport-history-upstream go test -count=1 -v ./tests/history
```

The test rejects a different upstream commit or a modified checkout. Without `BENCHREPORT_HISTORY_UPSTREAM`, the integration test skips; CI sets it explicitly on Linux and macOS. Network access is needed to acquire this dependency and any uncached Go modules, not to invoke the action against the local fixtures. Numeric probes exercise the actual upstream JavaScript parser and JSON serialization at ordinary decimals, the safe-integer boundary, subnormals, underflow, and overflow. Export rejects numbers that fail the [binary64 compatibility rule](contracts.md#history-serialization-and-outputs), retaining exact normalized-run precision independently of this external format.
