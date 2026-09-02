#!/usr/bin/env bash
set -euo pipefail

REPO="${1:-$(gh repo view --json nameWithOwner -q .nameWithOwner)}"

if ! command -v gh >/dev/null 2>&1; then
  echo "ERROR: GitHub CLI (gh) is required." >&2
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "ERROR: gh is not authenticated. Run: gh auth login" >&2
  exit 1
fi

echo "Applying branch protection to $REPO ..."

# develop: implementation must arrive through PR and pass the current CI jobs.
# required_approving_review_count=0 is intentional: this repository currently uses
# one GitHub identity for PM/TL/Developer automation, so requiring one approval
# would make self-authored PRs impossible to merge. Team Lead acceptance remains
# a workflow/process gate until a separate reviewer identity is introduced.
gh api \
  --method PUT \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "repos/$REPO/branches/develop/protection" \
  --input - <<'JSON'
{
  "required_status_checks": {
    "strict": true,
    "contexts": ["Frontend", "Backend", "Local Infrastructure"]
  },
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "dismiss_stale_reviews": false,
    "require_code_owner_reviews": false,
    "required_approving_review_count": 0,
    "require_last_push_approval": false
  },
  "restrictions": null,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "block_creations": false,
  "required_conversation_resolution": true,
  "lock_branch": false,
  "allow_fork_syncing": false
}
JSON

# main: stable/release branch. Require PR and forbid destructive history changes.
# CI is not required here yet because main currently does not contain the CI
# workflow. Once CI exists on main and runs for PRs targeting main, add the same
# required checks (or a dedicated release check set).
gh api \
  --method PUT \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "repos/$REPO/branches/main/protection" \
  --input - <<'JSON'
{
  "required_status_checks": null,
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "dismiss_stale_reviews": false,
    "require_code_owner_reviews": false,
    "required_approving_review_count": 0,
    "require_last_push_approval": false
  },
  "restrictions": null,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "block_creations": false,
  "required_conversation_resolution": true,
  "lock_branch": false,
  "allow_fork_syncing": false
}
JSON

echo
echo "Verification:"
gh api "repos/$REPO/branches/develop/protection" --jq '{develop: {enforce_admins: .enforce_admins.enabled, required_checks: [.required_status_checks.contexts[].context], required_pr: (.required_pull_request_reviews != null), conversation_resolution: .required_conversation_resolution.enabled, force_pushes: .allow_force_pushes.enabled, deletions: .allow_deletions.enabled}}'
gh api "repos/$REPO/branches/main/protection" --jq '{main: {enforce_admins: .enforce_admins.enabled, required_pr: (.required_pull_request_reviews != null), conversation_resolution: .required_conversation_resolution.enabled, force_pushes: .allow_force_pushes.enabled, deletions: .allow_deletions.enabled}}'

echo
echo "Done. All writers, including repository admins, must now respect the protected-branch rules."
