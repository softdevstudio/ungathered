# UnGathered

## Where the next session starts

Read "In progress" below and continue from the first step not marked done. The work queue for all SoftDevStudio.com projects is softdev-plan.md in photozeitgeist/jollyroger.biz (item 37 is this project).

## In progress: UnGathered 0.2 (softdev-plan item 37, Phase A, approved by YK 2026-11-01)

What and why: three features that close the biggest gaps the 0.1 README admits. Nested asset trees need one collection per depth today; a whole item that was never created can't be seen; content and docs repos have no ready CI gate.

Design decisions:
- `**` is allowed only as a whole path segment in `items` and in a new `exclude` list. As in .gitignore it matches zero or more segments, except at the end of a pattern, where it matches one or more (`drafts/**` is everything inside drafts, not drafts itself). `a**b`, `**.md` and `**/**` stay invalid.
- `exclude` (optional, up to 100 globs) removes anchors after matching. Excludes never remove companions; they only remove anchors. A symlink is skipped silently only when it can't be a counted anchor itself and an exclude ending in `**` covers everything below it; otherwise it stays an error, as in 0.1.
- `roster` (optional per collection): `{"file": ..., "match": "name|stem|item", "column": ..., "unlisted": "ignore|gap"}`. The file path is relative to the manifest file (absolute allowed), never to the scanned root. `.csv` files need `column` and read that header's column; other files are one entry per line, blank lines and `#` comments ignored. Entries are trimmed, exact and case-sensitive. A roster entry no selected item matches is a `missing_item` gap; with `unlisted: gap` an item absent from the roster is an `unlisted_item` gap. Duplicate or empty entries, a `/` in a name or stem entry, or an unreadable roster are invalid input (exit 2). Limits: 16 MiB and 100000 entries per roster. Reports gain a `roster` block per collection (file, sha256, match, entries, found, missing, unlisted), plus `kind` and `exclude` per collection. The library takes the roster folder as `Options.RosterDir`; the CLI passes the manifest's folder.
- Report schema stays version 1: the new fields are additive.
- Output formats `github` (annotations from fixed fields only) and `markdown` (step summary) for the Action. `--json FILE` and `--html FILE` write extra reports (new files outside the root, like `--output`) and `--summary FILE` appends Markdown to an existing file outside the root, so one Action run does annotations, summary and both reports. All targets are checked before any is written; a failure removes the files that run created.
- Action: composite `action.yml` per the PXTA ACTION.md pattern, tested by `.github/workflows/action-test.yml` on Linux, macOS and Windows with a binary built from the commit, plus a download job that runs after each release on the published binaries, pinned and latest.
- Upkeep: `go.mod` and README minimum Go 1.26; workflows use the current stable Go; a Windows CI job; CLAUDE.md restructured with this section first.

Steps (done / remaining):
1. [x] Upkeep: go.mod 1.26, workflows on stable Go, Windows CI job.
2. [x] `**` globs and `exclude`, with tests (nesting, zero-depth, excludes, symlinks under excluded and non-excluded trees, invalid patterns).
3. [x] Roster files, with tests (text and CSV, each match key, missing and unlisted items, duplicates, bad rows, size limits, BOM, CRLF).
4. [x] `--format github|markdown` and `--summary`, with tests.
5. [ ] `action.yml`, ACTION.md, action-test workflow.
6. [ ] README, docs/reports.md, examples (a nested-assets example with a roster), demo script assertions.
7. [ ] Independent review agent, fixes, fresh test/results.
8. [ ] Version 0.2.0 on main, release workflow, SHA256SUMS, run the downloaded binary, action-test download job.
9. [ ] ungathered.com post and pages, softdevstudio.com note, softdev-plan.md status.

Done so far (2026-11-01): steps 1 to 4, with tests in glob_test.go, roster_test.go, render_test.go and main_test.go; all pass locally with -race. A file name with a colon still makes its companion paths unsafe_path (0.1 behaviour, kept for Windows drive safety).

Local note: this cloud session has Go 1.24 only (the Go download hosts are blocked), so local tests run on a scratch go.mod; GitHub CI on stable Go is the real check.

## Project facts

UnGathered finds missing companions in local collections. Domain: ungathered.com. Code: softdevstudio/ungathered. Site: photozeitgeist/ungathered.com. Go, Apache 2.0.

## Low maintenance workflow

Work on main. This file is the only plan and shared Claude/Codex handoff; AGENTS.md points here. README is the manual. docs/ contains user reference only. No ROADMAP, RESUME, STATUS or session files. One Go binary with standard library only. Website comes last, after the verified code release, using the existing Hugo Launch template unchanged when a site repository is supplied. No domain sale/value discussion or content from other projects.

## Screening record

Problem: a folder listing shows present files and leaves missing specifications, images or credits for someone to notice manually. Technology: declare anchors and required companions, enumerate local items, expand relative companion paths and report gaps without modifying the collection. Differentiation target: reusable explicit collection rules, predictable reports and honest scan-error handling across publishing and document packs. This is a product goal, not a claim of novelty. Possible revenue: tailored manifests, integration and support for private document workflows; demand and name clearance have not been established.

## Code and completion standard

Tests for each operation and failures that threaten completeness claims. CI on Linux and macOS with vet and race tests. scripts/record-results.sh replaces test/results/latest; one evidence directory per release. Run an independent review before completion, fix findings and record the result here. Raise the version on main; automated release follows green CI, builds Linux/macOS/Windows archives, smoke-tests native platforms, publishes a full release titled UnGathered vX.Y.Z and checks every asset against SHA256SUMS at the tag and latest. Run a downloaded binary. If publication leaves a failed tag/draft, fix the cause and ship the next patch; report any leftovers. README is the manual; LICENSE, NOTICE and THIRD_PARTY_LICENSES.md stay current.

## Site standard

Separate repo, supplied later. Existing Launch theme unchanged; content and configuration only. Pages: about, contact (info@ungathered.com), quick start, download, reference. ASCII standalone titles, one H1 per page, no em dashes. Posts have double-quoted title, date only, draft false, tags and unique SVG featured_image. Download links must name actual latest-release archives. Add the studio listing and note at website launch, following its CONTENT-PLAN.md.

## Development plan

Version 1 JSON manifest: named collections select relative glob anchors of kind file or directory and declare named required relative paths using item/name/stem/dir variables. Required files may have a minimum size and a regular-expression content check; directories can be required too. Enumerate without following symlinks, reject escapes, expose unreadable paths and unmet minimum item counts. Distinguish a missing companion from an operational scan error. Provide text, JSON and self-contained HTML reports, deterministic ordering, read-only scanning and output files reserved outside the input tree. Demonstrate manufacturing packs and publishing assets. Test, review, release, verify downloads; website last.

## Where we are

2026-10-24: Implemented manifest validation, local anchor enumeration, companion path variables, file/directory requirements, minimum byte sizes, regex content checks, bounded reads, deterministic reports and safe external output reservation. Text, JSON and HTML reports distinguish declared gaps from operational errors. Local vet/race tests and generated manufacturing/publishing demos pass. Cross-builds for Windows amd64 and macOS arm64 pass.

Independent review found noncanonical globs could silently miss anchors when empty selection was permitted. Fixed by rejecting dot segments, repeated separators and trailing slash. Review also found duplicate-key bypasses through case and Unicode field folding; schema keys now require lowercase ASCII, with regression tests. Ancestor symlinks that could hide anchors now error, and scans copy rule slices before validation for concurrent safety. Release archives include examples/docs after a packaging review caught their omission. Final targeted independent review and race tests pass with no remaining findings.

Manufacturing demo: two items, six checks, four present and two gaps (empty drawing, missing inspection). Publishing demo: two items, four checks, two present and two gaps (missing image and credit). Input digests and file inventories remain unchanged during scans. A deliberately repaired fixture has two complete items and six present checks. These are generated placeholder documents, not PDF/image validation. UnGathered 0.1.0 is published as a full release. Linux/macOS CI and native Linux amd64, macOS arm64 and Windows amd64 archive smoke checks passed. All five archives at the tag and latest independently matched SHA256SUMS; both manifests matched. Every archive includes examples, reference and license files. A downloaded Linux binary reproduced both gap reports with unchanged input inventories and digests. There is no leftover draft. Verification is in test/results/ungathered-0.1.0-release.

The user already created photozeitgeist/ungathered.com, discovered during the final phase. Its Launch template was kept byte-identical while replacing sample content with five documentation/contact pages, a recorded scan explorer and six illustrated posts, including manufacturing and publishing cases. Hugo 0.147.9 and 635 internal link/asset checks pass. An independent content and DOM review found no mismatch; all three datasets exactly match recorded reports except the disclosed omission of incidental absolute root labels. SVGs were rasterized and inspected. Site commit: cba2d9546acdcbbadfd987ccf861f5be13c82c19.

UnGathered is listed once as the thirtieth SoftDevStudio.com project, with its launch note, name/tag/feed entries and coverage updated. Studio commit: 6f75c820dc35ad49181c7b191a3ce0b62e56e5a3. The note and project entry were verified in the public browser. 2026-10-24: After the user connected the domain, verified https://ungathered.com/ in the public browser over HTTPS. All seven homepage SVG images loaded. The live recorded explorer returned manufacturing 6 checks/4 present/2 gaps, publishing 4 checks/2 present/2 gaps, and repaired manufacturing 6 checks/6 present/0 gaps, all with zero operational errors and expected row counts. Quick Start and Reference rendered; RSS contained 12 items. Download links name the five published archives and SHA256SUMS, and the version command preserves both hyphens.

## Next

The released code, live website and studio listing are complete and verified. Further engine features await the user.
