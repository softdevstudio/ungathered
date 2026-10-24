#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
go build -o bin/ungathered ./cmd/ungathered
rm -rf demo-output
mkdir -p demo-output/pack/products/alpha demo-output/pack/products/beta demo-output/publishing/articles demo-output/publishing/images
printf 'Specification placeholder\n' > demo-output/pack/products/alpha/specification.pdf
printf 'Drawing placeholder\n' > demo-output/pack/products/alpha/drawing.pdf
printf 'part,result\nalpha,pass\n' > demo-output/pack/products/alpha/inspection.csv
printf 'Specification placeholder\n' > demo-output/pack/products/beta/specification.pdf
: > demo-output/pack/products/beta/drawing.pdf
printf 'A cafe article.\nPhoto credit: Studio\n' > demo-output/publishing/articles/cafe.md
printf 'A harbor article without a credit.\n' > demo-output/publishing/articles/harbor.md
printf 'Image placeholder\n' > demo-output/publishing/images/cafe.jpg
python3 - <<'PY'
from pathlib import Path
import hashlib,json
p=Path('demo-output')
files=list((p/'pack').rglob('*'))+list((p/'publishing').rglob('*'))
(p/'before.json').write_text(json.dumps({str(f):hashlib.sha256(f.read_bytes()).hexdigest() for f in files if f.is_file()},sort_keys=True))
PY
expect_gap() {
  if "$@"; then echo 'Expected declared gaps' >&2; exit 1; else code=$?; test "$code" -eq 1; fi
}
expect_gap ./bin/ungathered --manifest examples/manufacturing.json --root demo-output/pack --format json --output demo-output/manufacturing.json
expect_gap ./bin/ungathered --manifest examples/manufacturing.json --root demo-output/pack --format html --output demo-output/manufacturing.html
expect_gap ./bin/ungathered --manifest examples/publishing.json --root demo-output/publishing --format json --output demo-output/publishing.json
expect_gap ./bin/ungathered --manifest examples/publishing.json --root demo-output/publishing --format text > demo-output/publishing.txt
python3 - <<'PY'
from pathlib import Path
import hashlib,json
p=Path('demo-output')
before=json.loads((p/'before.json').read_text())
files=list((p/'pack').rglob('*'))+list((p/'publishing').rglob('*'))
assert before=={str(f):hashlib.sha256(f.read_bytes()).hexdigest() for f in files if f.is_file()}
a=json.loads((p/'manufacturing.json').read_text());b=json.loads((p/'publishing.json').read_text())
assert a['status']=='incomplete' and a['totals']=={'items':2,'complete_items':1,'checks':6,'present':4,'gaps':2,'errors':0},a
assert b['status']=='incomplete' and b['totals']=={'items':2,'complete_items':1,'checks':4,'present':2,'gaps':2,'errors':0},b
print('Manufacturing: 2 items, 6 checks, 2 gaps. Publishing: 2 items, 4 checks, 2 gaps. Input digests unchanged.')
PY
# Preparing a repaired fixture is done by this demo, never by the scanner.
printf 'Drawing placeholder\n' > demo-output/pack/products/beta/drawing.pdf
printf 'part,result\nbeta,pass\n' > demo-output/pack/products/beta/inspection.csv
./bin/ungathered --manifest examples/manufacturing.json --root demo-output/pack --format json --output demo-output/repaired.json
python3 - <<'PY'
import json
r=json.load(open('demo-output/repaired.json'))
assert r['status']=='complete' and r['totals']['complete_items']==2 and r['totals']['gaps']==0
print('Repaired manufacturing fixture: 2 complete items, 6 present checks, no gaps.')
PY
