# The UnGathered GitHub Action

`action.yml` in the repository root is a composite action. It downloads an UnGathered release for the runner, checks it against the release's SHA256SUMS, scans the collection, writes a summary to the job page, adds an annotation for each problem and uploads the JSON and HTML reports.

## Use it

```yaml
name: Assets
on: pull_request
jobs:
  ungathered:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: softdevstudio/ungathered@v0.2.0
        with:
          manifest: ungathered.json
          root: content
```

| Input | Default | What it does |
| --- | --- | --- |
| `manifest` | | The manifest, relative to the workspace. A relative roster file is read from the manifest's folder. |
| `root` | `.` | Folder to scan, relative to the workspace. |
| `args` | | More options as one string, such as `--max-content-bytes 4194304`. |
| `out` | a folder in the runner's temp directory | Where `ungathered.json` and `ungathered.html` go. It must be outside `root`. A missing folder is made only after the scan, once it is known to be outside `root`; existing report files there are never replaced. |
| `upload` | `true` | Upload the two reports as an artifact. |
| `artifact-name` | `ungathered-report` | Artifact name. Using the action twice in one job needs a different name for each. |
| `fail` | `true` | Fail the step when files are missing. `false` reports only; any value other than `true` or `false` is refused. |
| `version` | | `latest` or a release tag such as `v0.2.0`. Empty runs the release the action is pinned to (`@v0.2.0` runs 0.2.0); a branch or commit pin runs the latest. |
| `ungathered-path` | | Use this binary instead of downloading one, for testing the action. |

Outputs: `result` (`pass` or `fail`), `exit-code` (0 complete, 1 declared gaps, 2 invalid input or an operational error), `version` (the release that ran) and `report` (the folder holding the two reports).

A scan that cannot finish, such as a missing manifest, an unreadable roster, a symlink that could hide items or a report folder inside the scanned root, always fails the step, whatever `fail` says. The reports are uploaded when the scan wrote them, including after an operational error, so the evidence survives a failed step.

## What the job shows

Each problem becomes one error annotation, titled with its status (`UnGathered missing`, `UnGathered missing_item` and so on). When the scanned root is inside the workspace, the annotation names a file, so it appears beside that file in a pull request: the file itself for a size or content failure, and otherwise the item's own file when the collection selects files. GitHub shows only the first annotations of a step, so the job summary also gets a table of up to 200 problems with the totals and any roster counts. The full list is always in the uploaded reports.

Annotations and the summary are built from the report's fixed fields: statuses, collection and requirement names, paths and the scanner's own diagnostics. File contents and regex matches are never copied into them.

## Running the same scan locally

The action runs one command. To get the same results on a workstation:

```sh
ungathered --manifest ungathered.json --root content --json /tmp/report.json --html /tmp/report.html
```

Add `--format github` or `--format markdown` to see the annotation lines or the summary table.
