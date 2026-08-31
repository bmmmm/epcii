#!/usr/bin/env bash
# Create the labels the issue forms reference. Idempotent — reruns update
# colour and description in place. GitHub silently drops a label an issue
# form requests but that does not exist, so run this once after adding or
# changing the forms (and again at GitHub-mirror promotion time).
set -euo pipefail

repo="${1:-$(gh repo view --json nameWithOwner -q .nameWithOwner)}"

label() {
  gh label create "$1" --repo "$repo" --color "$2" --description "$3" --force >/dev/null
  printf '  %s\n' "$1"
}

printf 'Labels for %s:\n' "$repo"

# Every label referenced by a `labels:` field in .github/ISSUE_TEMPLATE/*.yml
# plus dependabot.yml. bug/enhancement/question exist as GitHub defaults but
# are listed anyway so a repo with pruned defaults still works.
label bug          "d73a4a" "Something is broken"
label enhancement  "a2eeef" "New behaviour proposal"
label question     "d876e3" "Usage or behaviour question"
label triage       "ededed" "Not looked at yet"
label task         "0e8a16" "Fully specified, agent-ready work item"
label dependencies "0366d6" "Dependency updates (dependabot)"

printf 'Done.\n'
