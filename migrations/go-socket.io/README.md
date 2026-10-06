# go-socket.io migration proposal

Shared rollout prerequisites, action pins, and proposal boundaries are defined in [Migration proposals](../../docs/migrations.md).

Consumer: `sshaplygin/go-socket.io`, base commit `1151bad8545694607e48ea17dbc1df8289dfcb34`. The inspected local tree was clean. [changes.patch](changes.patch) changes only `.github/workflows/benchmarks.yml` and adds `.github/benchmarks/benchmark-report.json`. Existing `.github/benchmarks/main.go`, its tests, and legacy formatter files remain unchanged.

## Consumer mapping

The workflow uses the shared pinned generation and publication actions. The [report action](../../docs/report-action.md), [publication](../../docs/publication.md), and [contracts](../../docs/contracts.md) own their interfaces and safety rules.

The change detector retains its scope, incremental change-base handling, tests, skip summary, trigger, and concurrency. Measurement retains `ubuntu-24.04`, `actions/setup-go@v5` with `go-version: stable`, `GOTOOLCHAIN: local`, exact base/head revision checkouts, and this command for both sides on one runner:

```sh
go test -mod=readonly -run '^$' -bench . -benchmem -count=10 ./...
```

Manifests record the installed `go env GOVERSION`, `GOOS`, and `GOARCH`, both exact revisions, suite `go`, and that command. Existing missing-benchmark checks stay; command/pipeline failures stop comparison. The report action supplies the pinned benchstat formerly installed separately.

The consumer configuration preserves timing-only tables, grouping by package, one-decimal advisory percentages, ±20% thresholds, unlimited rows, full benchstat details, and no merge gate. All 39 measurements remain in comparison JSON; rendering selects timing rows. History stays disabled. The configuration is read from the PR checkout exclusively in the read-only computation job; it never executes in the publishing job.

The complete checksummed bundle is uploaded as `go-benchmarks` for 14 days. Failed computations retain available raw logs separately. A separate publishing job downloads Markdown without checking out or executing PR code or artifact executables, adds the complete summary once, then publishes the bounded comment with header `go-socket.io-benchmarks`. Fork and Dependabot events retain artifacts and summaries while the pinned publisher skips comments. If a gate is enabled later, its result is applied after upload and publication.

The new sticky marker differs from the legacy inline marker. Existing old comments are not modified by this proposal; rollout can leave one legacy comment alongside the new comment unless maintainers remove it manually.

## Local verification

Verification used a disposable clone, never the real consumer working tree. Commands from that clone passed:

```sh
actionlint .github/workflows/benchmarks.yml
go test ./.github/benchmarks ./.github/benchmarks/report
git diff --check
```

From a benchmark-report checkout, with `consumer` pointing to the patched disposable clone and `benchstat` pointing to the verified pinned executable:

```sh
go build -o "$TMPDIR/benchreport" ./cmd/benchreport
"$TMPDIR/benchreport" config validate --config "$consumer/.github/benchmarks/benchmark-report.json"
"$TMPDIR/benchreport" report --parser go \
  --base-manifest testdata/captured/go-pr15/base-manifest.json \
  --head-manifest testdata/captured/go-pr15/head-manifest.json \
  --config "$consumer/.github/benchmarks/benchmark-report.json" \
  --benchstat-path "$benchstat" --comment-header go-socket.io-benchmarks \
  --output-dir "$TMPDIR/go-report"
"$TMPDIR/benchreport" replay --reproduction "$TMPDIR/go-report/reproduction.json" \
  --benchstat-path "$benchstat" --output-dir "$TMPDIR/go-replay"
cmp "$TMPDIR/go-report/report.md" "$TMPDIR/go-replay/report.md"
cmp "$TMPDIR/go-report/comparison.json" "$TMPDIR/go-replay/comparison.json"
```

Use fresh destinations. The archived PR15 captures contain 13 timing identities with 10 repeats per side and 39 total metric identities. A temporary test invoked the retained legacy formatter's `measurements` and `delta` functions, exported rational estimates/deltas, and compared them against generated comparison JSON: all 13 exact base/head medians, rounded percentages, and advisory classifications matched. All 13 timing signals were below threshold. Both empty benchmark logs and missing raw files failed with exit 1, no successful stdout, and no complete bundle.

Intentional presentation changes include full `Benchmark` prefixes, canonical package labels, the shared report layout and metadata, a new sticky marker, complete machine-readable artifacts, and explicit reproduction evidence. Benchstat inputs now carry stable `base`/`head` labels instead of filename-derived column labels. These changes do not alter timing estimates or advisory signals.

## Apply and rollback after acceptance

Use a clean disposable checkout at the recorded commit first, with `proposal` set to this directory's absolute path:

```sh
test "$(git rev-parse HEAD)" = 1151bad8545694607e48ea17dbc1df8289dfcb34
test -z "$(git status --porcelain)"
git apply --check "$proposal/changes.patch"
git apply "$proposal/changes.patch"
# Re-run the checks above before proposing a consumer PR.
git apply --reverse --check "$proposal/changes.patch"
git apply --reverse "$proposal/changes.patch"
test -z "$(git status --porcelain)"
```

Apply and reverse-apply were verified to restore the disposable checkout exactly. If accepted changes are committed later, revert that migration commit instead. No legacy files are deleted, so rollback restores the previous workflow without rebuilding its formatter.
