# Report format and status semantics

Report schema version is 1, independently of the executable's version. JSON records tool_version, canonical root, manifest_sha256, status, totals, collections and issues. There are no timestamps, so unchanged inputs with the same root and manifest bytes produce stable report ordering. The manifest digest covers its exact bytes, including whitespace.

Collections retain manifest order. Items are sorted by relative forward-slash path, and requirements retain declaration order. An item is complete only when every requirement has status present. Issues record collection-wide minimum failures and operational enumeration/symlink failures. A collection's min_items defaults to 1. Explicit 0 permits an empty selection by design.

| Check status | Classification |
| --- | --- |
| present | Requirement passed |
| missing | Declared gap |
| wrong_type | Declared gap, including a non-directory parent |
| too_small | Declared gap |
| content_mismatch | Declared gap |
| unreadable | Operational error |
| unsafe_symlink | Operational error |
| unsafe_path | Operational error |
| content_too_large | Operational error |

Issue status below_minimum contributes one gap per collection below its threshold. scan_error and unsafe_symlink issues contribute operational errors. Totals.gaps therefore includes both failed requirements and unmet collection minimums; it is not always the number of missing files. A single physical file can be checked by several rules or collections, each counted separately. An item may satisfy every companion while the collection as a whole has too few items.

Status error takes priority when any operational error exists. Otherwise a gap makes status incomplete; no gaps or operational errors makes it complete. Invalid manifests, invalid roots and failed report writes exit 2 with a diagnostic rather than a fabricated report. Text shows problem details and aggregate totals; JSON/HTML include passed checks too.

A regular-expression content check tests the complete file up to its configured read limit. Oversized files are errors, without partial matching. Empty contains means no content check. Use min_bytes:1 for nonempty files. Regexes test bytes and do not parse frontmatter, validate attribution rights or establish that an image URL is reachable. No file contents or regex capture values are embedded in the report.
