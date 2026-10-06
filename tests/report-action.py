#!/usr/bin/env python3
"""Run the report action shell wrapper locally against the actual Go CLI."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def parse_outputs(text):
    # Preserve every byte of multiline values, including trailing newlines.
    remaining = text
    result = {}
    while remaining:
        declaration, remaining = remaining.split('\n', 1)
        name, delimiter = declaration.split('<<', 1)
        assert name not in result
        value, remaining = remaining.split('\n' + delimiter + '\n', 1)
        result[name] = value
    return result


def main():
    argparse.ArgumentParser(description=__doc__).parse_args()
    with tempfile.TemporaryDirectory(prefix='benchreport-action-tests-') as temporary:
        root = Path(temporary)
        binary = root / 'benchreport'
        validator = root / 'contractcheck'
        for destination, package in ((binary, './cmd/benchreport'), (validator, './cmd/contractcheck')):
            subprocess.run(['go', 'build', '-o', str(destination), package], cwd=ROOT, check=True)
        fake = root / 'fake'
        fake.mkdir()
        for tool in ('go', 'python', 'python3'):
            path = fake / tool
            path.write_text('#!/bin/sh\nprintf forbidden >> "$TEST_FORBIDDEN"\nexit 99\n')
            path.chmod(0o755)
        count = 0
        def run(label, parser='go', config=None, overrides=None, expected=None, custom=False):
            nonlocal count
            count += 1
            case = root / str(count)
            case.mkdir()
            fixture = 'go-pr15' if parser == 'go' else 'criterion-four-suites'
            output = case / 'github-output'
            output.touch()
            env = dict(os.environ, PATH=str(fake) + os.pathsep + os.environ['PATH'], REPORT_BINARY=str(binary),
                       REPORT_PARSER=parser, BASE_MANIFEST=str(ROOT / 'testdata/captured' / fixture / 'base-manifest.json'),
                       HEAD_MANIFEST=str(ROOT / 'testdata/captured' / fixture / 'head-manifest.json'),
                       RUNNER_TEMP=str(case), GITHUB_OUTPUT=str(output), TEST_FORBIDDEN=str(case / 'forbidden'))
            for key in ('REPORT_CONFIG', 'REPORT_OUTPUT_DIR', 'ALLOW_ENVIRONMENT_MISMATCH', 'COMMENT_HEADER', 'ARTIFACT_URL'):
                env.pop(key, None)
            if config:
                config_path = case / 'config.json'
                config_path.write_text(json.dumps(config))
                env['REPORT_CONFIG'] = str(config_path)
            if custom:
                env['REPORT_OUTPUT_DIR'] = str(case / 'bundle\ninjected-key=value\n')
            env.update(overrides or {})
            result = subprocess.run(['/bin/bash', str(ROOT / 'scripts/report.sh')], env=env, capture_output=True, text=True)
            assert not (case / 'forbidden').exists(), label
            if expected:
                assert result.returncode != 0 and expected in result.stderr, (label, result.returncode, result.stderr)
                assert output.read_bytes() == b'', label
            else:
                assert result.returncode == 0, (label, result.stderr)
                values = parse_outputs(output.read_text())
                assert not result.stdout, label
                document = case / 'outputs.json'
                document.write_text(json.dumps(values))
                def validate(schema, path):
                    subprocess.run([str(validator), str(ROOT / 'schemas' / schema), str(path)], check=True)
                validate('report-action-outputs.schema.json', document)
                for key, schema in [('comparison-path', 'comparison.schema.json'), ('reproduction-path', 'reproduction.schema.json'), ('json-path', 'presentation.schema.json')]:
                    if values[key]:
                        validate(schema, values[key])
                for key in ('report-path', 'json-path', 'comparison-path', 'reproduction-path', 'comment-path'):
                    if values[key]:
                        assert Path(values[key]).is_file(), (label, key)
                if custom:
                    physical_dir = Path(env['REPORT_OUTPUT_DIR']).resolve()
                    assert values['artifact-path'] == str(physical_dir), values
                    assert values['report-path'] == str(physical_dir / 'custom\nreport.md\n'), values
                    assert values['json-path'] == str(physical_dir / 'custom\npresentation.json\n'), values
                if config and config.get('outputs', {}).get('markdown', 'default') is None:
                    assert values['report-path'] == values['comment-path'] == '', values
                if config and config.get('comparison', {}).get('fail_on_regression'):
                    assert values['gate'] == 'failed', values
                else:
                    assert values['gate'] == 'disabled', values
            print('PASS', label)

        run('captured Go schema-backed report')
        run('captured Criterion schema-backed report', parser='criterion')
        run('JSON only', config={'schema_version': 1, 'outputs': {'markdown': None, 'json': 'report.json'}})
        run('failed enabled gate still generates outputs', config={'schema_version': 1, 'comparison': {'regression_percent': 0.01, 'fail_on_regression': True}})
        run('custom output names and directory retain trailing newlines', custom=True, config={'schema_version': 1, 'outputs': {'markdown': 'custom\nreport.md\n', 'json': 'custom\npresentation.json\n'}})
        for value in ('TRUE', '1', ''):
            run('reject boolean ' + repr(value), overrides={'ALLOW_ENVIRONMENT_MISMATCH': value}, expected='must be true or false')
        for value in ('', 'bad header', 'x' * 101, 'name\ninjected=value'):
            run('reject header ' + repr(value), overrides={'COMMENT_HEADER': value}, expected='Invalid comment-header')
        print(f'{count} report wrapper cases passed')


if __name__ == '__main__':
    main()
