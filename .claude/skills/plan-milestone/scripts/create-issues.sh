#!/usr/bin/env bash
# Create one GitHub issue per sub-scope in a milestone plan, assign the
# milestone, and add each issue to the milestone's project board.
#
# Usage:
#   create-issues.sh <plan-file> <milestone-num> <project-num> [repo] [owner]
#
# Example:
#   create-issues.sh docs/plans/m1-doctor.md 1 3
#
# Expectations about the plan file:
#   - Has a "## Sub-scopes" H2 section
#   - Each sub-scope begins "### N.M — Title [STATUS]" with a bracketed status
#   - Sub-scopes are separated by "---"; the section ends at the next H2
#
# Each issue body is prefixed with a link back to the sub-scope's own section in
# the plan, then the verbatim sub-scope content.

set -euo pipefail

plan_file="${1:?usage: $0 <plan-file> <milestone-num> <project-num> [repo] [owner]}"
ms_num="${2:?usage: $0 <plan-file> <milestone-num> <project-num> [repo] [owner]}"
project_num="${3:?usage: $0 <plan-file> <milestone-num> <project-num> [repo] [owner]}"
repo="${4:-rollingstart-dev/rollingstart}"
owner="${5:-rollingstart-dev}"

[[ -f "$plan_file" ]] || { echo "Plan file not found: $plan_file" >&2; exit 1; }

plan_url="https://github.com/${repo}/blob/main/${plan_file}"
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

# Resolve the "M{N}: Title" milestone. Roadmap milestones are created up front,
# so this only looks one up — it never creates.
milestone=$(gh api "repos/${repo}/milestones?state=all&per_page=100" \
  --jq "[.[].title | select(startswith(\"M${ms_num}:\"))][0] // empty")
[[ -n "$milestone" ]] || {
  echo "No 'M${ms_num}: …' milestone found in ${repo}" >&2
  exit 1
}

# Split the Sub-scopes section into one file per sub-scope, recording the
# GitHub anchor slug so each issue can link to its own section of the plan.
awk -v dir="$tmpdir" '
  /^## Sub-scopes/     { in_section=1; next }
  /^## / && in_section { in_section=0 }
  !in_section          { next }
  /^### [0-9]+\.[0-9]+ / {
    if (out) close(out)

    full = $0
    sub(/^### /, "", full)

    # GitHub anchor slug: lowercase, drop non-[a-z0-9 -], spaces to hyphens.
    slug = tolower(full)
    gsub(/[^a-z0-9 -]/, "", slug)
    gsub(/ /, "-", slug)

    # Title is the heading minus its trailing "[STATUS]" tag.
    header = full
    sub(/ \[[^]]*\][[:space:]]*$/, "", header)
    split(header, a, " — ")
    num = a[1]; title = a[2]

    out = dir "/" num ".body.md"
    printf "%s\t%s\t%s\n", num, title, slug >> dir "/index.tsv"
    next
  }
  /^---[[:space:]]*$/ { if (out) { close(out); out="" }; next }
  out { print >> out }
' "$plan_file"

[[ -s "$tmpdir/index.tsv" ]] || { echo "No sub-scopes found under '## Sub-scopes' in $plan_file" >&2; exit 1; }

while IFS=$'\t' read -r num title slug; do
  body_file="$tmpdir/$num.body.md"
  issue_file="$tmpdir/$num.issue.md"

  {
    printf 'Sub-scope of **[M%s plan](%s#%s)**.\n\n---\n' \
      "$ms_num" "$plan_url" "$slug"
    # Trim trailing blank lines for a clean issue body.
    awk 'NF {for(i=0;i<held;i++) print ""; held=0; print; next} {held++}' "$body_file"
  } > "$issue_file"

  issue_url=$(gh issue create \
    --repo "$repo" \
    --title "${num} — ${title}" \
    --body-file "$issue_file" \
    --milestone "$milestone")
  echo "Created ${num}: ${issue_url}"

  gh project item-add "$project_num" --owner "$owner" --url "$issue_url" >/dev/null
done < "$tmpdir/index.tsv"
