#!/usr/bin/env python3
"""Verify locally built release assets before uploading them for review."""
import argparse
import hashlib
import json
from pathlib import Path
import tarfile

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--assets', type=Path, required=True)
    args = parser.parse_args()
    version = (ROOT / 'report/VERSION').read_text().strip()
    license_bytes = (ROOT / 'LICENSE').read_bytes()
    archives = sorted(args.assets.glob('*.tar.gz'))
    if not archives:
        raise ValueError('no release archives')
    for archive_path in archives:
        expected_name = f'benchreport_{version}_'
        if not archive_path.name.startswith(expected_name):
            raise ValueError(f'unexpected release asset: {archive_path.name}')
        checksum = archive_path.with_name(archive_path.name + '.sha256').read_text().split()
        if len(checksum) != 2 or checksum[1] != archive_path.name:
            raise ValueError('invalid release checksum sidecar')
        if checksum[0] != hashlib.sha256(archive_path.read_bytes()).hexdigest():
            raise ValueError('release archive checksum mismatch')
        with tarfile.open(archive_path, 'r:gz') as archive:
            members = archive.getmembers()
            names = [member.name for member in members]
            if len(names) != len(set(names)) or any(not member.isfile() for member in members):
                raise ValueError('duplicate or nonregular release members')
            manifest = json.loads(archive.extractfile('manifest.json').read())
            if manifest['candidate'] is not False or manifest['version'] != version:
                raise ValueError('candidate or mismatched release version')
            if archive_path.name != f"benchreport_{version}_{manifest['os']}_{manifest['arch']}.tar.gz":
                raise ValueError('release platform does not match asset name')
            if archive.extractfile('LICENSE').read() != license_bytes:
                raise ValueError('release LICENSE differs from repository LICENSE')
            if archive.extractfile('VERSION').read().decode().strip() != version:
                raise ValueError('archive VERSION mismatch')
            listed = [entry['path'] for entry in manifest['files']]
            if len(listed) != len(set(listed)) or set(listed) != set(names) - {'manifest.json'}:
                raise ValueError('incomplete release inventory')
            if 'PROJECT-LICENSE-STATUS.txt' in names:
                raise ValueError('candidate license notice in release')
            for entry in manifest['files']:
                data = archive.extractfile(entry['path']).read()
                if hashlib.sha256(data).hexdigest() != entry['sha256']:
                    raise ValueError(f"member checksum mismatch: {entry['path']}")
            notice = archive.extractfile('LICENSES/NOTICE.txt').read().decode('utf-8')
            source_notice = (
                f'Benchmark Report {version} source code (MPL-2.0):\n'
                f'https://github.com/sshaplygin/benchmark-report/tree/v{version}\n'
            )
            if not notice.startswith(source_notice):
                raise ValueError('missing versioned project source URL or MPL-2.0 notice')
            for name in (
                'benchreport', 'benchstat', 'LICENSES/NOTICE.txt', 'LICENSES/Go_LICENSE'
            ):
                if name not in names:
                    raise ValueError(f'missing required release member: {name}')
        print(f'PASS licensed release inventory and checksums: {archive_path.name}')


if __name__ == '__main__':
    main()
