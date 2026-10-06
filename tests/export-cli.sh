#!/usr/bin/env bash
set -euo pipefail
repo_dir=$(cd "$(dirname "$0")/.." && pwd)
test_dir=$(mktemp -d "${TMPDIR:-/tmp}/benchreport-export.XXXXXX")
trap 'rm -rf "$test_dir"' EXIT
cd "$repo_dir"
go build -o "$test_dir/benchreport" ./cmd/benchreport
cp testdata/handwritten/go/valid.txt "$test_dir/raw.txt"
node - "$test_dir" <<'JS'
const fs = require('fs'), path = require('path'), root = process.argv[2];
fs.writeFileSync(path.join(root,'manifest.json'), JSON.stringify({schema_version:1,revision:'a'.repeat(40),environment:{toolchain:'go1.25.0',os:'linux',arch:'amd64',runner:'fixture'},expected_suites:['go'],suites:[{id:'go',parser:{name:'go',version:'1'},command:'go test -bench .',files:['raw.txt']}]}));
fs.writeFileSync(path.join(root,'all.json'), JSON.stringify({schema_version:1,history:{enabled:true,metrics:['time','bytes','allocations','throughput']},report:{include:['a^']}}));
for (const [name,history] of Object.entries({time:{enabled:true,metrics:['time']},throughput:{enabled:true,metrics:['throughput']},disabled:{enabled:false},empty:{enabled:true,include:['a^']},collision:{enabled:true,smaller_file:'run.json'}})) fs.writeFileSync(path.join(root,name+'.json'),JSON.stringify({schema_version:1,history}));
JS
"$test_dir/benchreport" normalize --parser go --manifest "$test_dir/manifest.json" --out "$test_dir/run.json"
"$test_dir/benchreport" export --input "$test_dir/run.json" --config "$test_dir/all.json" --output-dir "$test_dir/out" > "$test_dir/all-paths.json"
node - "$test_dir" <<'JS'
const fs = require('fs'),path=require('path'),root=process.argv[2];const result=JSON.parse(fs.readFileSync(path.join(root,'all-paths.json')));
if(result.schema_version!==1||Object.keys(result.files).sort().join(',')!=='bigger,smaller')throw Error('missing direction paths');
const run=JSON.parse(fs.readFileSync(path.join(root,'run.json'))), lookup=new Map(run.measurements.map(m=>[m.key,m]));
for(const [direction,p] of Object.entries(result.files)){if(!path.isAbsolute(p))throw Error('relative path');const rows=JSON.parse(fs.readFileSync(p));if(rows.length!==(direction==='smaller'?9:1))throw Error('bad counts');for(const entry of rows){const m=lookup.get(entry.name);if(!m||typeof entry.value!=='number'||JSON.stringify(entry.value)!==m.estimate||entry.unit!==m.definition.unit)throw Error('changed measurement');if(JSON.parse(entry.extra).estimator!=='median')throw Error('metadata');}}
fs.writeFileSync(path.join(root,'out','unmanaged.txt'),'keep');
JS
"$test_dir/benchreport" export --input "$test_dir/run.json" --config "$test_dir/time.json" --output-dir "$test_dir/out" > "$test_dir/time-paths.json"
node - "$test_dir" <<'JS'
const fs=require('fs'),p=require('path'),r=process.argv[2],result=JSON.parse(fs.readFileSync(p.join(r,'time-paths.json')));if(Object.keys(result.files).join(',')!=='smaller'||fs.existsSync(p.join(r,'out','benchmark-bigger.json'))||fs.readFileSync(p.join(r,'out','unmanaged.txt'),'utf8')!=='keep')throw Error('stale cleanup');
JS
"$test_dir/benchreport" export --input "$test_dir/run.json" --config "$test_dir/throughput.json" --output-dir "$test_dir/out" > "$test_dir/bigger-paths.json"
test ! -e "$test_dir/out/benchmark-smaller.json"
for config in disabled empty collision; do
  if "$test_dir/benchreport" export --input "$test_dir/run.json" --config "$test_dir/$config.json" --output-dir "$test_dir" > "$test_dir/rejected-stdout" 2> "$test_dir/rejected-stderr"; then
    printf 'Export unexpectedly accepted %s.\n' "$config" >&2
    exit 1
  else
    test "$?" -eq 1
  fi
  test ! -s "$test_dir/rejected-stdout"
  test -s "$test_dir/rejected-stderr"
done
# Export needs no raw files or benchstat, including a configuration that requests statistics for comparison.
rm "$test_dir/raw.txt"
"$test_dir/benchreport" export --input "$test_dir/run.json" --config examples/benchmark-report.json --output-dir "$test_dir/no-raw" > "$test_dir/no-raw-paths.json"
printf 'History export numeric, independent selection, cleanup, and failure checks passed.\n'
