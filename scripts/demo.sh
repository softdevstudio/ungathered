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
# Nested magazine issues with an editorial plan as the roster.
mkdir -p demo-output/magazine/issues/spring/images demo-output/magazine/issues/spring/features demo-output/magazine/issues/spring/drafts demo-output/magazine/issues/summer/images
printf 'Cafe feature.\nPhoto credit: Studio\n' > demo-output/magazine/issues/spring/cafe.md
printf 'Image placeholder\n' > demo-output/magazine/issues/spring/images/cafe.jpg
printf 'Harbor feature.\nPhoto credit: Studio\n' > demo-output/magazine/issues/spring/features/harbor.md
printf 'Work in progress.\n' > demo-output/magazine/issues/spring/drafts/wip.md
printf 'Market story without a credit.\n' > demo-output/magazine/issues/summer/market.md
printf 'Image placeholder\n' > demo-output/magazine/issues/summer/images/market.jpg
printf 'Bonus story.\nPhoto credit: Studio\n' > demo-output/magazine/issues/summer/bonus.md
printf 'Image placeholder\n' > demo-output/magazine/issues/summer/images/bonus.jpg
printf 'Notes for editors.\n' > demo-output/magazine/issues/summer/README.md
expect_gap ./bin/ungathered --manifest examples/magazine.json --root demo-output/magazine --format json --output demo-output/magazine.json --html demo-output/magazine.html
expect_gap ./bin/ungathered --manifest examples/magazine.json --root demo-output/magazine --format markdown > demo-output/magazine.md
python3 - <<'PY2'
import json
r=json.load(open('demo-output/magazine.json'))
assert r['status']=='incomplete' and r['totals']=={'items':4,'complete_items':2,'checks':8,'present':6,'gaps':4,'errors':0},r['totals']
items=[i['path'] for i in r['collections'][0]['items']]
assert items==['issues/spring/cafe.md','issues/spring/features/harbor.md','issues/summer/bonus.md','issues/summer/market.md'],items
issues=[(i['status'],i['path']) for i in r['issues']]
assert issues==[('missing_item','ferry'),('unlisted_item','issues/summer/bonus.md')],issues
ro=r['collections'][0]['roster']
assert (ro['entries'],ro['found'],ro['missing'],ro['unlisted'])==(4,3,1,1),ro
print('Magazine: 4 items at two depths, drafts and README excluded, 8 checks, 4 gaps (missing image, missing credit, planned ferry never written, bonus not in the plan).')
PY2
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
