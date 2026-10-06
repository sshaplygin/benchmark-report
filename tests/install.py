#!/usr/bin/env python3
"""Exercise the release installer using local downloads; Python is test-only."""
import argparse
import copy
import hashlib
import io
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--assets', type=Path, required=True)
    args = parser.parse_args()
    system = {'Darwin': 'darwin', 'Linux': 'linux'}[platform.system()]
    arch = {'arm64': 'arm64', 'aarch64': 'arm64', 'x86_64': 'amd64', 'amd64': 'amd64'}[platform.machine()]
    version = (ROOT / 'report/VERSION').read_text().strip()
    asset = f'benchreport_{version}_{system}_{arch}.tar.gz'
    original = (args.assets / asset).read_bytes()
    with tarfile.open(fileobj=io.BytesIO(original), mode='r:gz') as archive:
        members = [(entry, archive.extractfile(entry).read()) for entry in archive]

    def mutate(name=None, data=None, extra=None):
        out = io.BytesIO()
        with tarfile.open(fileobj=out, mode='w:gz') as archive:
            for info, contents in members:
                info = copy.copy(info)
                if info.name == name:
                    contents = data
                info.size = len(contents)
                archive.addfile(info, io.BytesIO(contents))
            if extra:
                info, contents = extra
                archive.addfile(info, io.BytesIO(contents) if contents else None)
        return out.getvalue()

    with tempfile.TemporaryDirectory(prefix='benchreport-install-tests-') as temporary:
        root = Path(temporary)
        (root / 'scripts').mkdir()
        (root / 'report').mkdir()
        shutil.copyfile(ROOT / 'scripts/install.sh', root / 'scripts/install.sh')
        (root / 'report/VERSION').write_text(version + '\n')
        fake = root / 'fake'
        fake.mkdir()
        def executable(name, text):
            path = fake / name
            path.write_text('#!/bin/bash\nset -eu\n' + text)
            path.chmod(0o755)
        executable('curl', '''printf '%s\\n' "$*" >> "$TEST_CURL_LOG"
url= output=
while [[ $# -gt 0 ]]; do
 case "$1" in --output) output=$2; shift 2;; https://*) url=$1; shift;; *) shift;; esac
done
[[ "$url" == "$TEST_BASE_URL/$TEST_ASSET" || "$url" == "$TEST_BASE_URL/$TEST_ASSET.sha256" ]]
case "$url" in *.sha256) cp "$TEST_CHECKSUM" "$output";; *) cp "$TEST_ARCHIVE" "$output";; esac
''')
        executable('uname', 'if [[ "$1" == -s ]]; then printf "%s\\n" "$TEST_OS"; else printf "%s\\n" "$TEST_ARCH"; fi\n')
        for tool in ('go', 'python', 'python3'):
            executable(tool, 'printf "%s\\n" "Unexpected runtime toolchain invocation" >> "$TEST_FORBIDDEN"; exit 99\n')
        sentinel = root / 'outside'
        sentinel.write_text('preserved')
        count = 0
        def run(label, payload=original, checksum=None, expected=None, os_name=platform.system(), machine=platform.machine()):
            nonlocal count
            count += 1
            case = root / f'case-{count}'
            case.mkdir()
            archive = case / 'archive'
            archive.write_bytes(payload)
            digest = hashlib.sha256(payload).hexdigest()
            checksum_file = case / 'checksum'
            checksum_file.write_text(checksum if checksum is not None else f'{digest}  {asset}\n')
            output = case / 'output'
            output.touch()
            env = dict(os.environ, PATH=str(fake) + os.pathsep + os.environ['PATH'],
                       RUNNER_TEMP=str(case), GITHUB_OUTPUT=str(output), TEST_ARCHIVE=str(archive),
                       TEST_CHECKSUM=str(checksum_file), TEST_CURL_LOG=str(case / 'curl-log'),
                       TEST_FORBIDDEN=str(case / 'forbidden'), TEST_OS=os_name, TEST_ARCH=machine,
                       TEST_ASSET=asset, TEST_BASE_URL=f'https://github.com/sshaplygin/benchmark-report/releases/download/v{version}')
            result = subprocess.run(['/bin/bash', str(root / 'scripts/install.sh')], env=env, text=True, capture_output=True)
            assert not (case / 'forbidden').exists(), label
            assert sentinel.read_text() == 'preserved', label
            if expected is None:
                assert result.returncode == 0, (label, result.stderr)
                lines = output.read_text().splitlines()
                assert len(lines) == 3 and lines[0] == 'bin-dir<<' + lines[2], (label, lines)
                installed = Path(lines[1])
                assert subprocess.check_output([str(installed / 'benchreport'), '--version'], text=True).strip() == f'benchreport {version}'
                assert (installed / 'benchstat').is_file()
            else:
                assert result.returncode != 0 and expected in result.stderr, (label, result.returncode, result.stderr)
                assert not output.read_text(), label
                assert not list(case.glob('benchreport-install.*')), label
            if (case / 'curl-log').exists():
                for line in (case / 'curl-log').read_text().splitlines():
                    assert '--proto =https --proto-redir =https' in line, line
            print('PASS', label)

        run('native executable without Go or Python runtime')
        run('corrupt archive', original + b'corrupt', checksum=f'{hashlib.sha256(original).hexdigest()}  {asset}\n', expected='Archive checksum mismatch')
        marker = root / 'binary-executed'
        instrumented = mutate('benchreport', f'#!/bin/sh\ntouch "{marker}"\nprintf "benchreport {version}\\n"\n'.encode())
        run('checksum rejection before execution', instrumented, checksum=f'{hashlib.sha256(original).hexdigest()}  {asset}\n', expected='Archive checksum mismatch')
        assert not marker.exists()
        digest = hashlib.sha256(original).hexdigest()
        for label, checksum in [('missing checksum', ''), ('duplicate checksum', f'{digest}  {asset}\n' * 2), ('wrong asset checksum', f'{digest}  other.tar.gz\n')]:
            run(label, checksum=checksum, expected='Missing, duplicate, or invalid archive checksum')
        manifest = json.loads(next(contents for info, contents in members if info.name == 'manifest.json'))
        for field, value in [('version', '9.9.9'), ('os', 'unsupported'), ('arch', 'unsupported'), ('benchstat', 'wrong-pin')]:
            altered = dict(manifest, **{field: value})
            run('manifest ' + field, mutate('manifest.json', json.dumps(altered).encode()), expected='Archive platform or version mismatch')
        run('archive VERSION mismatch', mutate('VERSION', b'9.9.9\n'), expected='Archive version mapping mismatch')
        run('binary version mismatch', mutate('benchreport', b'#!/bin/sh\nprintf "benchreport 9.9.9\\n"\n'), expected='Installed binary version mismatch')
        for name in ('../outside', '/outside', 'LICENSES/../../outside'):
            info = tarfile.TarInfo(name)
            info.size = 7
            run('unsafe path ' + name, mutate(extra=(info, b'changed')), expected='archive')
        for kind in (tarfile.SYMTYPE, tarfile.LNKTYPE):
            # A known filename is necessary to exercise the link-type guard itself.
            altered = []
            for info, contents in members:
                if info.name != 'benchstat':
                    altered.append((info, contents))
            saved = members[:]
            members[:] = altered
            info = tarfile.TarInfo('benchstat')
            info.type = kind
            info.linkname = str(sentinel)
            run('symlink' if kind == tarfile.SYMTYPE else 'hardlink', mutate(extra=(info, b'')), expected='Archive contains links or nonregular entries')
            members[:] = saved
        info = tarfile.TarInfo('VERSION')
        info.size = len(version) + 1
        run('duplicate archive member', mutate(extra=(info, (version + '\n').encode())), expected='Duplicate archive members')
        run('unsupported OS', expected='Unsupported operating system', os_name='Plan9')
        run('unsupported architecture', expected='Unsupported architecture', machine='sparc')
        print(f'{count} installer cases passed')


if __name__ == '__main__':
    main()
