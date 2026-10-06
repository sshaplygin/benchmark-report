# Report generation action

The [report action](../report/action.yml) consumes completed manifests and logs. It installs the binary version mapped by its commit, generates reports and a full reproduction bundle, and returns paths. It does not execute benchmark commands or publish comments. The root [publication action](publication.md) remains independently usable with an existing Markdown file.

No binary release is published yet. The action's download URLs become usable after a reviewed release; current CI tests substitute locally built candidate archives for the download transport and run the complete composite action. Build the CLI from source for local use meanwhile.

## Inputs and results

The action's string input and output maps are defined by [input](../schemas/report-action-inputs.schema.json) and [output](../schemas/report-action-outputs.schema.json) schemas. The pinned action commit versions these maps; they do not add a `schema_version` action input.

| Input | Use |
| --- | --- |
| `parser` | Required: `go` or `criterion` |
| `base-manifest`, `head-manifest` | Required paths to completed inputs, resolved from the caller's working directory |
| `config` | Optional reporting configuration; an omitted or empty path selects defaults |
| `output-dir` | Optional absent or empty directory; omission creates a new directory under `runner.temp` |
| `artifact-url` | Link to the uploaded complete bundle; defaults to the current workflow run's artifact listing |
| `comment-header` | Publisher header, default `benchmark-report`; must match publication so the byte budget includes the correct marker |
| `allow-environment-mismatch` | `true` or `false`, default `false`; records an explicit compatibility override |

The action returns `report-path`, `json-path`, `comparison-path`, `reproduction-path`, `artifact-path`, `comment-path`, `history-smaller-path`, `history-bigger-path`, and `gate`. Disabled formats and absent history groups return empty paths. `artifact-path` is the complete directory to upload. The [CLI contract](contracts.md#cli-contract) defines the saved documents and full replay behavior; [Configuration](configuration.md) defines selection and filenames.

`gate` is `disabled`, `passed`, or `failed`. A failed regression gate still produces all outputs and a successful generation step. Apply that status after uploading artifacts and publishing the report. Input, execution, and serialization errors fail generation immediately and provide no successful output map.

The complete Markdown and presentation JSON remain in the bundle. `comment-path` uses the [bounded-comment rules](publication.md#generated-comment-sizing). The caller uploads the complete bundle before publishing its comment and keeps the artifact available for the desired retention period. A run-page link leads to the artifact listing; supply a direct artifact URL when the caller already has one.

## Workflow

[examples/pull-request.yml](../examples/pull-request.yml) runs both revisions on one runner with one fixed toolchain, generates the report, uploads the full bundle, and publishes from a separate job. Replace its action revision placeholders and consumer-specific commands before use. The example expects `outputs.markdown: "report.md"`; update its summary path when choosing a different filename. Go is installed there to run Go benchmarks, not to execute the reporting action. A Criterion consumer retains its Rust setup and suite selection instead.

The publishing job runs no PR checkout, benchmark command, or downloaded executable. It adds the full summary once, then delegates the bounded comment to the root action with summary disabled. Fork and Dependabot events retain the summary and artifacts and skip comments. The final gate runs after publication. History publication uses the separate [primary-branch workflow](history.md).

## Binary mapping and installation

[report/VERSION](../report/VERSION) maps the action commit to version `0.1.0`. The installer selects Linux or macOS and amd64 or arm64, downloads `benchreport_VERSION_OS_ARCH.tar.gz` and its `.sha256` file from the matching GitHub release, and verifies the checksum before extraction or execution. It rejects unexpected archive paths, links, duplicate members, unsupported platforms, and archive/binary version mismatches. No fallback version or source build is selected. Benchstat is bundled at the [recorded pin](contracts.md#tools-and-boundaries).

Runtime prerequisites are Bash, curl, tar, jq, and either sha256sum or shasum, available on the supported GitHub-hosted runners. No Go or Python installation is needed for generation or publication. The root publisher additionally uses the upstream action's runtime described in [Publication](publication.md#dependency-contract).

The development packager builds one native platform archive:

```sh
go run ./cmd/package --output-dir dist
```

Archives contain both executables, version/platform metadata, checksums for members, and the license texts and notices of their dependencies. Without a project `LICENSE`, candidate archives contain `PROJECT-LICENSE-STATUS.txt`. `go run ./cmd/package --release --output-dir dist` rejects that state; choosing a project license and authorizing publication remain separate requirements. No packaging command publishes a release.

CI runs native archive installation and captured-consumer replay on `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-14`, and `macos-15-intel`. These labels cover both architectures according to the [GitHub runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners). Test harnesses use Go and Python on the build side; runtime checks reject attempts by the installer or generator wrapper to invoke them.
