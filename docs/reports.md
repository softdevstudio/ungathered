# Report format and status semantics

Report schema version is 1, independently of the executable's version. JSON records tool_version, canonical root, manifest_sha256, status, totals, collections and issues. UnGathered 0.2 added fields without changing the version: each collection now records its kind, its exclude patterns when it has any, and a roster block when it has a roster. Readers written for 0.1 reports keep working. There are no timestamps, so unchanged inputs with the same root and manifest bytes produce stable report ordering. The manifest digest covers its exact bytes, including whitespace.

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

Issue status below_minimum contributes one gap per collection below its threshold. missing_item and unlisted_item come from a roster and contribute one gap each. scan_error and unsafe_symlink issues contribute operational errors.

| Issue status | Classification |
| --- | --- |
| below_minimum | Declared gap: the collection selected fewer items than min_items |
| missing_item | Declared gap: a roster name no selected item matches; path is the roster name |
| unlisted_item | Declared gap, only with unlisted set to gap: a selected item the roster does not name; path is the item |
| scan_error | Operational error |
| unsafe_symlink | Operational error |

The roster block records file (as written in the manifest), sha256 of the roster's exact bytes, match, entries, found, missing and unlisted. found counts roster names matched by at least one item. Two items can share a name or stem in different folders; both then satisfy the same roster name. Like the manifest digest, the roster digest lets a reader tell which list a report was checked against. Totals.gaps therefore includes both failed requirements and unmet collection minimums; it is not always the number of missing files. A single physical file can be checked by several rules or collections, each counted separately. An item may satisfy every companion while the collection as a whole has too few items.

Status error takes priority when any operational error exists. Otherwise a gap makes status incomplete; no gaps or operational errors makes it complete. Invalid manifests, invalid roots and failed report writes exit 2 with a diagnostic rather than a fabricated report. Text shows problem details, roster counts and aggregate totals; JSON/HTML include passed checks too. The github format prints a totals line and one ::error workflow command per problem; markdown prints the totals, roster counts and a problem table capped at 200 rows. Both are views of the same report and change neither the status nor the exit code.

A regular-expression content check tests the complete file up to its configured read limit. Oversized files are errors, without partial matching. Empty contains means no content check. Use min_bytes:1 for nonempty files. Regexes test bytes and do not parse frontmatter, validate attribution rights or establish that an image URL is reachable. No file contents or regex capture values are embedded in the report.
