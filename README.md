# UnGathered

Find what should be in a local collection and is missing. UnGathered selects items from a folder, expands the companion paths declared in a small JSON manifest, and reports missing files, empty documents or absent credit text. It runs as one Go binary with no service or third party module dependencies. Scanning does not write to the collection, repair files or make network requests.

## Quick start

After unpacking a release archive, its executable and example manifests are ready to use:

```sh
./ungathered --manifest examples/manufacturing.json --root /path/to/packs
```

The commands below build a source checkout with Go 1.25 or newer:

```sh
go build -o bin/ungathered ./cmd/ungathered
./bin/ungathered --version
./bin/ungathered --manifest examples/manufacturing.json --root /path/to/packs
```

The manufacturing example selects directories under products. Each selected directory needs a nonempty specification.pdf, drawing.pdf and inspection.csv. A directory named products/alpha is an anchor; the scanner asks which declared companions should be beside it. It checks presence, regular-file type and minimum size, without parsing PDF or CSV contents.

```json
{
  "version": 1,
  "collections": [
    {
      "name": "Products",
      "items": "products/*",
      "kind": "directory",
      "require": [
        {"name": "Specification", "path": "{item}/specification.pdf", "min_bytes": 1},
        {"name": "Drawing", "path": "{item}/drawing.pdf", "min_bytes": 1},
        {"name": "Inspection", "path": "{item}/inspection.csv", "min_bytes": 1}
      ]
    }
  ]
}
```

A publishing collection can select articles/*.md and require images/{stem}.jpg plus a photo-credit line inside the article. The complete manifest is in [examples/publishing.json](examples/publishing.json). Its regex looks for a line beginning Photo credit: followed by a non-whitespace character. This is a literal content check, without interpreting Markdown or validating photo ownership. Image selection is an explicit naming convention, rather than an inferred relationship or a fetched URL.

## Reports and exit codes

```sh
./bin/ungathered --manifest collection.json --root pack --format json
./bin/ungathered --manifest collection.json --root pack --format html --output /outside/pack/report.html
```

The default text format lists problems and totals. JSON and self-contained HTML include every selected item and requirement, including passed checks. Output is written to stdout unless --output names a new file. That file's parent must already exist, the file must not exist, and its path must be outside the collection root, including when the output parent contains a symlink. A broken pipe or failed report write is an operational failure. No report file is silently overwritten.

| Exit | Report status | Meaning |
| --- | --- | --- |
| 0 | complete | All selected items satisfy their declared requirements and minimum item counts |
| 1 | incomplete | Declared gaps exist, including missing, wrong-type, undersized or content-mismatched companions |
| 2 | error, or a diagnostic on stderr | Operational errors or invalid input prevent a complete conclusion |

A complete report proves only the rules you wrote about the items selected. An existing collection item can itself be absent from the anchor set; the scanner cannot infer an expected product roster that was never declared. A misspelled glob normally produces a gap because each collection needs at least one item by default. Set min_items explicitly to 0 only when an empty collection is intentional. That setting also means a typo selecting nothing can pass, so review optional collection patterns carefully.

Operational problems such as an unreadable directory, a symlink that could hide anchors, or an oversized content check produce status error and exit 2, even when other checks pass. A missing companion produces a declared gap. No scan failure is converted into a complete result.

## Manifest reference

Manifest version is 1. Collection names must be nonempty and unique. Each collection has name, items, kind and require; min_items is optional and defaults to 1. kind is file or directory. At least one collection and one requirement per collection are mandatory. Schema field names use exact lowercase ASCII spelling. Unknown fields and duplicate JSON object keys are rejected. This also rejects case variants and Unicode lookalikes that could otherwise overwrite a schema field. Limits are 100 collections, 100 requirements per collection, a 4 MiB manifest and 64 levels of JSON nesting.

The items value is a canonical forward-slash glob relative to --root. A star matches within one path segment, so products/* selects immediate children and articles/*/*.md selects a further level. Character classes and question marks use Go path.Match syntax. Recursive **, absolute paths, drive names, backslashes, parent traversal, leading ./, repeated separators and trailing slashes are rejected. The root itself is not an anchor. File anchors select regular files; directory anchors select directories. Other matching types are excluded from that kind's item count, with matching symlinks reported as errors.

Every requirement has a unique name within its collection and a relative path. Its kind defaults to file; directory is also supported. File requirements may have min_bytes (default 0, meaning existence/type only) and contains (a nonempty Go regular expression). Directory requirements cannot use file size or content checks. The scanner checks these conditions in order and reports the first problem for that requirement. A minimum size is a size check, not a file-format or document-quality validator.

| Path variable | For an anchor articles/cafe.md |
| --- | --- |
| {item} | articles/cafe.md |
| {name} | cafe.md |
| {stem} | cafe |
| {dir} | articles |

Paths are relative to the scan root, not the manifest file or the anchor's directory. To require a sibling, use {dir}/{stem}.jpg. For a directory anchor, {item}/drawing.pdf addresses a file inside it. Only the final filename extension is removed by {stem}. Required paths use literal filenames after variable expansion; they are not globbed. Unknown variables and paths that escape the root are rejected.

Regex reads are bounded by --max-content-bytes, which defaults to 1048576 bytes per file. Larger files produce an operational error for a content rule; they are never matched against a silently truncated prefix. The flag accepts 1 to 1073741824. Presence-only and size-only rules do not read file contents, so they establish metadata conditions rather than content readability. Full status names and scope limits are in [docs/reports.md](docs/reports.md).

## Reproducible demonstrations

```sh
sh scripts/demo.sh
sh scripts/record-results.sh
```

Run these scripts from a source checkout. The demo prepares generated manufacturing packs and article assets. One product has an empty drawing and a missing inspection record. One article lacks an image and credit line. JSON assertions check the exact totals, and source digests are compared before and after scanning. The demo then repairs its own generated fixture and checks a complete result; the scanner never performs that repair. The PDF/image filenames contain plain placeholders and are not presented as valid documents or photographs.

The result recorder runs vet, race tests and those demos, replacing test/results/latest. CI runs on Linux and macOS. Release archives cover Linux amd64/arm64, macOS amd64/arm64 and Windows amd64; native release smoke tests cover Linux amd64, macOS arm64 and Windows amd64. The other two targets are cross-built.

## Boundaries

The collection must remain stable during a scan. Enumeration, metadata checks and content reads do not form an atomic filesystem snapshot. Symlinks are not followed; direct and possible ancestor symlinks affecting anchor selection, or any symlink in a required path, are errors. An unrelated symlink that cannot affect a collection's anchor pattern is ignored. This protects ordinary static scans; it is not a sandbox against a hostile process replacing paths concurrently. Scan a stable copy when other processes may be modifying the collection.

The scanner walks the root to enumerate possible anchors, including hidden directories. Pick a suitably narrow root. Required-file bytes are read only for content rules and are not copied into reports. Reports do contain paths, requirement names and operational diagnostics, so inspect a report before sharing it. Collection-to-collection dependencies, remote storage APIs, document parsers and automatic repairs are outside the first release.

Read [CLAUDE.md](CLAUDE.md) for the shared Claude/Codex plan and handoff. Apache 2.0; see LICENSE and NOTICE. The website is a separate final stage.
