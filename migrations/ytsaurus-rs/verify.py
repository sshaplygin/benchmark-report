#!/usr/bin/env python3
"""Compare archived Criterion decimals with the consumer's retained parser."""

import argparse
import importlib.util
import json
import subprocess
import sys
import tempfile
from decimal import Decimal
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--consumer", type=Path, required=True)
    parser.add_argument("--binary", type=Path, required=True)
    args = parser.parse_args()
    source = args.consumer / "scripts/compare_criterion.py"
    spec = importlib.util.spec_from_file_location("legacy_criterion", source)
    if spec is None or spec.loader is None:
        parser.error(f"cannot load retained consumer parser: {source}")
    legacy = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = legacy
    spec.loader.exec_module(legacy)
    fixture = ROOT / "testdata/captured/criterion-four-suites"
    with tempfile.TemporaryDirectory(prefix="rust-migration-oracle-") as directory:
        for side, folder in [("base", "base"), ("head", "pr")]:
            output = Path(directory) / f"{side}.json"
            subprocess.run(
                [
                    str(args.binary.resolve()),
                    "normalize",
                    "--parser",
                    "criterion",
                    "--manifest",
                    str(fixture / f"{side}-manifest.json"),
                    "--out",
                    str(output),
                ],
                check=True,
            )
            document = json.loads(output.read_text())
            matched = 0
            for suite, expected_count in [
                ("client", 12),
                ("job", 9),
                ("skiff", 3),
                ("yson", 4),
            ]:
                path = (
                    fixture
                    / f"criterion-main-vs-pr-{suite}"
                    / folder
                    / "benchmarks.txt"
                )
                before = legacy.parse_benchmarks(path.read_text(), str(path))
                after = {
                    row["identity"]["benchmark"]: row
                    for row in document["measurements"]
                    if row["identity"]["suite"] == suite
                }
                assert len(after) == expected_count, (side, suite, len(after))
                for name, value in before.items():
                    number, unit = value.display.split()
                    expected = Decimal(number) * legacy.UNIT_TO_NANOSECONDS[unit]
                    assert Decimal(after[name]["estimate"]) == expected, (
                        side,
                        suite,
                        name,
                    )
                    matched += 1
                additions = {
                    name: row["estimate"]
                    for name, row in after.items()
                    if name not in before
                }
                if suite == "client":
                    expected_inline = {
                        "base": {
                            "read/read_table/1000": "244470",
                            "read/read_table/10000": "1592800",
                            "read/read_table/100000": "16179000",
                        },
                        "head": {
                            "read/read_table/1000": "246230",
                            "read/read_table/10000": "1595300",
                            "read/read_table/100000": "16490000",
                        },
                    }
                    assert additions == expected_inline[side], (side, additions)
                else:
                    assert not additions, (suite, additions)
                print(
                    side,
                    suite,
                    f"legacy={len(before)} normalized={len(after)}",
                    additions,
                )
            assert matched == 25 and len(document["measurements"]) == 28
            print(
                f"{side}: all 25 legacy source decimals match; 28 normalized estimates"
            )


if __name__ == "__main__":
    main()
