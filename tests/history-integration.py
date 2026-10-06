#!/usr/bin/env python3
"""Exercise the unmodified pinned upstream action locally, without credentials."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile
from decimal import Decimal

PIN = "4322e5726e6334590d251fc4f92bec0efafc45dc"
ROOT = Path(__file__).resolve().parents[1]


def run(args, *, cwd=ROOT, env=None, success=True):
    result = subprocess.run(args, cwd=cwd, env=env, text=True, capture_output=True)
    if success and result.returncode:
        raise AssertionError(f"{args}:\n{result.stdout}\n{result.stderr}")
    return result


def event(path, revision):
    path.write_text(json.dumps({
        "ref": "refs/heads/main",
        "repository": {"name": "disposable", "html_url": "https://example.invalid/fixture/disposable"},
        "head_commit": {"id": revision, "message": "Synthetic history integration fixture",
                        "timestamp": "2026-10-06T00:00:00Z", "url": "https://example.invalid/commit/" + revision,
                        "author": {"name": "Fixture", "username": "fixture", "email": "fixture@example.invalid"},
                        "committer": {"name": "Fixture", "username": "fixture", "email": "fixture@example.invalid"}}
    }))


def upstream_env(directory, payload, output, history, profile, tool):
    # Whitelist prevents inherited repository tokens and runner credentials.
    env = {k: os.environ[k] for k in ("PATH", "HOME", "TMPDIR", "SYSTEMROOT") if k in os.environ}
    env.update({"GITHUB_EVENT_PATH": str(payload), "GITHUB_EVENT_NAME": "push",
                "GITHUB_REPOSITORY": "fixture/disposable", "GITHUB_WORKSPACE": str(directory),
                "GITHUB_REF": "refs/heads/main", "GITHUB_SHA": json.loads(payload.read_text())["head_commit"]["id"],
                "GITHUB_ACTOR": "fixture", "GITHUB_SERVER_URL": "https://example.invalid",
                "INPUT_TOOL": tool, "INPUT_NAME": profile, "INPUT_OUTPUT-FILE-PATH": str(output),
                "INPUT_GH-PAGES-BRANCH": "gh-pages", "INPUT_BENCHMARK-DATA-DIR-PATH": str(directory / "bench"),
                "INPUT_EXTERNAL-DATA-JSON-PATH": str(history), "INPUT_AUTO-PUSH": "false",
                "INPUT_SAVE-DATA-FILE": "true", "INPUT_SKIP-FETCH-GH-PAGES": "true",
                "INPUT_COMMENT-ALWAYS": "false", "INPUT_COMMENT-ON-ALERT": "false",
                "INPUT_SUMMARY-ALWAYS": "false", "INPUT_FAIL-ON-ALERT": "false",
                "INPUT_ALERT-THRESHOLD": "200%"})
    return env


def normalize(binary, directory, label, revision, toolchain="go1.25.0", parser="go", log="go-first.txt"):
    manifest = directory / (label + "-manifest.json")
    manifest.write_text(json.dumps({"schema_version": 1, "revision": revision,
        "environment": {"toolchain": toolchain, "os": "linux", "arch": "amd64", "runner": "synthetic-runner"},
        "expected_suites": ["fixture"], "suites": [{"id": "fixture", "parser": {"name": parser, "version": "1"},
        "command": "synthetic fixture; no workload executed", "files": [str(ROOT / "testdata/history" / log)]}]}))
    normalized = directory / (label + "-normalized.json")
    run([str(binary), "normalize", "--parser", parser, "--manifest", str(manifest), "--out", str(normalized)])
    return normalized


def export(binary, directory, normalized):
    cfg = directory / "config.json"
    cfg.write_text(json.dumps({"schema_version": 1, "history": {"enabled": True,
                           "metrics": ["time", "bytes", "allocations", "throughput"]}}))
    result = run([str(binary), "export", "--input", str(normalized), "--config", str(cfg),
                  "--output-dir", str(directory / "exports")])
    result = json.loads(result.stdout)
    return {k: Path(v) for k, v in result["files"].items()}


def numeric_boundaries(binary, directory, upstream, payload):
    numbers = [("0.1", True), ("61.145", True), ("9007199254740992", True),
               ("9007199254740993", False), ("0.10000000000000000001", False),
               ("5e-324", True), ("2e-324", False), ("1e309", False), ("1.7976931348623157e308", True)]
    js = """const fs=require('fs');const {BenchmarkResults}=require(process.argv[1]);
const rows=BenchmarkResults.parse(JSON.parse(fs.readFileSync(process.argv[2],'utf8')));
process.stdout.write(JSON.stringify(rows[0].value));"""
    for i, (text, accepted) in enumerate(numbers):
        canonical = format(Decimal(text), "f")
        raw = directory / f"number-{i}.txt"
        raw.write_text(f"pkg: numeric\nBenchmarkNumber-4 10 {canonical} ns/op\n")
        manifest = directory / f"number-{i}-manifest.json"
        manifest.write_text(json.dumps({"schema_version": 1, "revision": "a" * 40,
            "environment": {"toolchain": "go1.25.0", "os": "linux", "arch": "amd64", "runner": "synthetic-runner"},
            "expected_suites": ["numeric"], "suites": [{"id": "numeric", "parser": {"name": "go", "version": "1"},
            "command": "synthetic fixture", "files": [str(raw)]}]}))
        normalized = directory / f"number-{i}-normalized.json"
        run([str(binary), "normalize", "--parser", "go", "--manifest", str(manifest), "--out", str(normalized)])
        cfg = directory / "numeric-config.json"
        cfg.write_text('{"schema_version":1,"history":{"enabled":true}}')
        out = directory / f"number-{i}-export"
        result = run([str(binary), "export", "--input", str(normalized), "--config", str(cfg),
                      "--output-dir", str(out)], success=False)
        assert (result.returncode == 0) == accepted, (text, result.stdout, result.stderr)
        if accepted:
            smaller = Path(json.loads(result.stdout)["files"]["smaller"])
            parsed = run(["node", "-e", js, str(upstream / "dist/src/extract.js"), str(smaller)])
            assert Decimal(parsed.stdout) == Decimal(text), (text, parsed.stdout)
        else:
            assert not list(out.glob("*.json")), (text, "failed export wrote output")
    print("binary64 boundaries: exporter rejection and real upstream JSON parser round trips verified")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--upstream", type=Path, required=True, help="checkout of the exact pinned upstream commit")
    args = parser.parse_args()
    upstream = args.upstream.resolve()
    assert run(["git", "rev-parse", "HEAD"], cwd=upstream).stdout.strip() == PIN, "upstream checkout has wrong pin"
    assert not run(["git", "status", "--porcelain"], cwd=upstream).stdout.strip(), "upstream checkout is modified"
    assert (upstream / "dist/src/index.js").is_file()
    with tempfile.TemporaryDirectory(prefix="benchreport-history-") as tmp:
        directory = Path(tmp)
        run(["git", "init", "--quiet", "--initial-branch=main"], cwd=directory)
        assert not run(["git", "remote"], cwd=directory).stdout.strip()
        binary = directory / "benchreport"
        run(["go", "build", "-o", str(binary), "./cmd/benchreport"])
        payload = directory / "event.json"
        history = directory / "history.json"
        cases = [("first", "a" * 40, "go1.25.0", "go", "go-first.txt", "go-linux-amd64-go1.25-median"),
                 ("second", "b" * 40, "go1.25.0", "go", "go-second.txt", "go-linux-amd64-go1.25-median"),
                 ("new-profile", "c" * 40, "go1.26.0", "go", "go-second.txt", "go-linux-amd64-go1.26-median"),
                 ("criterion", "d" * 40, "rust1.94.0", "criterion", "criterion.txt", "criterion-linux-amd64-rust1.94-point")]
        expected = {}
        for label, revision, toolchain, adapter, log, profile in cases:
            normalized = normalize(binary, directory, label, revision, toolchain, adapter, log)
            files = export(binary, directory, normalized)
            measurements = {m["key"]: m for m in json.loads(normalized.read_text())["measurements"]}
            emitted = set()
            for direction, output in files.items():
                for row in json.loads(output.read_text(), parse_float=Decimal):
                    source = measurements[row["name"]]
                    assert Decimal(row["value"]) == Decimal(source["estimate"])
                    assert row["unit"] == source["definition"]["unit"]
                    assert direction == ("smaller" if source["definition"]["direction"] == "lower" else "bigger")
                    emitted.add(row["name"])
            assert emitted == set(measurements), "history export silently omitted a normalized measurement"
            if "bigger" not in files:
                assert not (directory / "exports/benchmark-bigger.json").exists(), "obsolete bigger-direction file retained"
            event(payload, revision)
            for direction, output in files.items():
                series = profile + "-" + direction
                tool = "customSmallerIsBetter" if direction == "smaller" else "customBiggerIsBetter"
                env = upstream_env(directory, payload, output, history, series, tool)
                result = run(["node", str(upstream / "dist/src/index.js")], cwd=directory, env=env)
                assert "was run successfully" in result.stdout
                expected.setdefault(series, []).append(json.loads(output.read_text(), parse_float=Decimal))
                stored = json.loads(history.read_text(), parse_float=Decimal)["entries"][series]
                assert len(stored) == len(expected[series])
                assert stored[-1]["commit"]["id"] == revision
                assert stored[-1]["benches"] == expected[series][-1], (series, stored[-1])
                assert all("extra" in row for row in stored[-1]["benches"])
                if adapter == "criterion":
                    assert "range" in stored[-1]["benches"][0]
        stored = json.loads(history.read_text())["entries"]
        assert len(stored["go-linux-amd64-go1.25-median-smaller"]) == 2
        assert len(stored["go-linux-amd64-go1.25-median-bigger"]) == 2
        assert len(stored["go-linux-amd64-go1.26-median-smaller"]) == 1
        assert len(stored["criterion-linux-amd64-rust1.94-point-smaller"]) == 1
        assert not run(["git", "remote"], cwd=directory).stdout.strip()
        numeric_boundaries(binary, directory, upstream, payload)
    print(f"upstream {PIN}: two append runs, direction/profile isolation, range/extra acceptance passed")


if __name__ == "__main__":
    main()
