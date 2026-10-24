# UnGathered

## Project facts

UnGathered finds missing companions in local collections. Domain: ungathered.com. Code: softdevstudio/ungathered. Website repository has not been supplied. Go, Apache 2.0.

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

Manufacturing demo: two items, six checks, four present and two gaps (empty drawing, missing inspection). Publishing demo: two items, four checks, two present and two gaps (missing image and credit). Input digests and file inventories remain unchanged during scans. A deliberately repaired fixture has two complete items and six present checks. These are generated placeholder documents, not PDF/image validation. Version 0.1.0 is ready for the initial remote push, Linux/macOS CI and automatic release. Website repo is not supplied.

## Next

Publish the verified first implementation to main, confirm CI and automated release, verify all five tag/latest archives and a downloaded binary, then save release evidence. Website waits for its own repo.
