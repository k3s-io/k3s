#!/usr/bin/env bash
set -euo pipefail

# Pin GitHub Actions to commit SHAs in the local repository.
#
# Requirements: gh, awk, jq
# Usage: ./pin-gha-to-sha.sh [--cache-file PATH] [--dry-run]

usage() {
  cat <<EOF
Usage: $0 [options]

Pin GitHub Actions uses: references to immutable commit SHAs in the local repository.
Only modifies YAML files in the .github/workflows/ directory.

Options:
  --cache-file PATH       Path to a JSON file that caches resolved SHAs across runs.
                          Default: /tmp/gha-pin-cache.json
  --dry-run               Show proposed changes only. Does not modify the files.
  -h, --help              Show this help message and exit.

Behavior:
  - Scans all YAML files in .github/workflows/ for 'uses:' references.
  - For each uses: reference not already pinned to a 40-char SHA:
      * If the ref is a tag: resolves the tag to its commit SHA.
      * If the ref is a branch: looks up the latest release tag via
        gh release list; if found, pins to that tag's SHA; otherwise
        pins to the branch HEAD SHA.
  - Modifies the file in-place, adding a trailing comment (# tag-or-branch) to each pinned line.
  - Inaccessible/private actions are logged to a temporary file.
  - Resolved SHAs are cached in the cache file for faster follow-up runs.

Hardcoded overrides:
  - aquasecurity/trivy-action  => 57a97c7e7821a5776cebc9bb87c984fa69cba8f1 # v0.35.0
  - aquasecurity/setup-trivy   => 3fb12ec12f41e471780db15c232d5dd185dcb514 # v0.2.6

Examples:
  $0
  $0 --dry-run
  $0 --cache-file /tmp/gha-pin-cache.json
EOF
  exit 0
}

# ─── Parse arguments ─────────────────────────────────────────────────
CACHE_FILE="/tmp/gha-pin-cache.json"
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      ;;
    --cache-file)
      [[ -z "${2:-}" ]] && { echo "Error: --cache-file requires a value" >&2; exit 1; }
      CACHE_FILE="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    -*)
      echo "Error: unknown option: $1" >&2
      echo "Run '$0 --help' for usage information." >&2
      exit 1
      ;;
    *)
      echo "Error: unexpected argument: $1" >&2
      echo "Run '$0 --help' for usage information." >&2
      exit 1
      ;;
  esac
done

for cmd in gh awk jq; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Error: $cmd is required but not found" >&2
    exit 1
  fi
done

if ! gh auth status >/dev/null 2>&1; then
  echo "Error: gh is not authenticated. Run: gh auth login" >&2
  exit 1
fi

if [[ ! -d ".github/workflows" ]]; then
  echo "Error: .github/workflows directory not found in the current path." >&2
  exit 1
fi

PRIVATE_LOG="/tmp/private-action-uses.log"
: > "$PRIVATE_LOG"

# ─── Hardcoded overrides ─────────────────────────────────────────────
declare -A HARDCODED_PINS=(
  ["aquasecurity/trivy-action"]="57a97c7e7821a5776cebc9bb87c984fa69cba8f1:v0.35.0"
  ["aquasecurity/setup-trivy"]="3fb12ec12f41e471780db15c232d5dd185dcb514:v0.2.6"
)

hardcoded_lookup() {
  local key="$1"
  echo "${HARDCODED_PINS[$key]:-}"
}

# ─── Cache file handling ─────────────────────────────────────────────
if [[ ! -f "$CACHE_FILE" ]]; then
  echo '{}' > "$CACHE_FILE"
fi

cache_get_sha() {
  local key="$1"
  jq -r --arg k "$key" '.[$k].sha // empty' "$CACHE_FILE"
}

cache_get_comment() {
  local key="$1"
  jq -r --arg k "$key" '.[$k].comment // empty' "$CACHE_FILE"
}

cache_set() {
  local key="$1" sha="$2" comment="$3"
  local tmp="${CACHE_FILE}.tmp.$$"
  jq --arg k "$key" --arg s "$sha" --arg c "$comment" \
    '.[$k] = {"sha": $s, "comment": $c}' "$CACHE_FILE" > "$tmp" \
    && mv "$tmp" "$CACHE_FILE"
}

# ─── Collect YAML files ───────────────────────────────────────────────
mapfile -t FILES < <(
  find .github/workflows -type f \( -name '*.yml' -o -name '*.yaml' \) 2>/dev/null | sort -u
)

if [[ ${#FILES[@]} -eq 0 ]]; then
  echo "No YAML files found in .github/workflows."
  exit 0
fi

echo "==> Cache file: $CACHE_FILE"
echo "==> Found ${#FILES[@]} file(s) to scan in .github/workflows/"

# ─── Extract every uses: value with file + line number ───────────────
# Outputs: FILE:LINENO:ORIGINAL_USES_VALUE
extract_all_uses() {
  local f
  for f in "${FILES[@]}"; do
    awk -v file="$f" '
    {
      line = $0
      sub(/\r$/, "", line)
      if (line ~ /^[[:space:]]*(-[[:space:]]+)?uses[[:space:]]*:/) {
        val = line
        sub(/^[[:space:]]*(-[[:space:]]+)?uses[[:space:]]*:[[:space:]]*/, "", val)
        gsub(/^["'\''"]|["'\''"]$/, "", val)
        sub(/[[:space:]]+#.*$/, "", val)
        sub(/[[:space:]]+$/, "", val)
        if (val != "") {
          printf "%s:%d:%s\n", file, NR, val
        }
      }
    }
    ' "$f"
  done
}

is_sha() { [[ "$1" =~ ^[0-9a-fA-F]{40}$ ]]; }

resolve_tag_sha() {
  local owner="$1" repo="$2" tag="$3"
  local json sha obj_type inner
  local rc=0

  json="$(gh api "repos/${owner}/${repo}/git/ref/tags/${tag}" 2>/dev/null)" || rc=$?
  if (( rc != 0 )); then return 1; fi

  sha="$(echo "$json" | jq -r '.object.sha // empty')"
  obj_type="$(echo "$json" | jq -r '.object.type // empty')"
  [[ -z "$sha" ]] && return 1

  if [[ "$obj_type" == "tag" ]]; then
    inner="$(gh api "repos/${owner}/${repo}/git/tags/${sha}" 2>/dev/null | jq -r '.object.sha // empty')" || true
    [[ -n "$inner" ]] && sha="$inner"
  fi

  echo "$sha"
}

resolve_branch_sha() {
  local owner="$1" repo="$2" branch="$3"
  local json sha
  local rc=0

  json="$(gh api "repos/${owner}/${repo}/git/ref/heads/${branch}" 2>/dev/null)" || rc=$?
  if (( rc != 0 )); then return 1; fi

  sha="$(echo "$json" | jq -r '.object.sha // empty')"
  [[ -z "$sha" ]] && return 1
  echo "$sha"
}

ref_is_tag() {
  local owner="$1" repo="$2" ref="$3"
  gh api "repos/${owner}/${repo}/git/ref/tags/${ref}" >/dev/null 2>&1
}

latest_release_tag() {
  local owner="$1" repo="$2"
  local tag
  tag="$(gh release list -R "${owner}/${repo}" --limit 1 --json tagName --jq '.[0].tagName' 2>/dev/null)" || true
  [[ -n "$tag" && "$tag" != "null" ]] && { echo "$tag"; return 0; }
  return 1
}

# ─── Main processing loop ───────────────────────────────────────────
total=0
pinned=0
skipped=0
failed=0
private=0
cache_hits=0
hardcoded_hits=0

echo "==> Processing uses: references ..."

while IFS=: read -r file lineno uses_val; do
  ((total++)) || true

  [[ "$uses_val" == ./* || "$uses_val" == ../* || "$uses_val" == docker://* ]] && continue
  [[ "$uses_val" != *@* ]] && continue

  left="${uses_val%@*}"
  ref="${uses_val##*@}"

  # shellcheck disable=SC2034
  IFS='/' read -r gha_owner gha_repo _ <<< "$left"
  [[ -z "${gha_owner:-}" || -z "${gha_repo:-}" ]] && continue

  gha_path=""
  if [[ "$left" != "${gha_owner}/${gha_repo}" ]]; then
    gha_path="${left#"${gha_owner}/${gha_repo}"}"
  fi

  if is_sha "$ref"; then
    ((skipped++)) || true
    continue
  fi

  echo "  Processing: ${gha_owner}/${gha_repo}${gha_path}@${ref} (${file}:${lineno})"

  sha=""
  comment=""

  # ── Check hardcoded overrides first ──
  hc_key="${gha_owner}/${gha_repo}"
  hc_val="$(hardcoded_lookup "$hc_key")"

  if [[ -n "$hc_val" ]]; then
    sha="${hc_val%%:*}"
    comment="${hc_val#*:}"
    echo "    Hardcoded override: @${sha} # ${comment}"
    ((hardcoded_hits++)) || true
  else
    # ── Check cache ──
    cache_key="${gha_owner}/${gha_repo}@${ref}"
    cached_sha="$(cache_get_sha "$cache_key")"
    cached_comment="$(cache_get_comment "$cache_key")"

    if [[ -n "$cached_sha" ]] && is_sha "$cached_sha"; then
      sha="$cached_sha"
      comment="$cached_comment"
      echo "    Cache hit: @${sha} # ${comment}"
      ((cache_hits++)) || true
    else
      if ref_is_tag "$gha_owner" "$gha_repo" "$ref"; then
        sha="$(resolve_tag_sha "$gha_owner" "$gha_repo" "$ref" || true)"
        comment="$ref"
      else
        latest="$(latest_release_tag "$gha_owner" "$gha_repo" || true)"
        if [[ -n "$latest" ]]; then
          sha="$(resolve_tag_sha "$gha_owner" "$gha_repo" "$latest" || true)"
          comment="$latest"
        fi

        if [[ -z "$sha" ]] || ! is_sha "$sha"; then
          sha="$(resolve_branch_sha "$gha_owner" "$gha_repo" "$ref" || true)"
          comment="$ref"
        fi
      fi

      if [[ -z "$sha" ]] || ! is_sha "$sha"; then
        echo "    WARN: could not resolve to SHA, logging as inaccessible"
        printf '%s\t%s\t%s\t%s\n' "$(date -u +%FT%TZ)" "$file" "$uses_val" "inaccessible/private" >> "$PRIVATE_LOG"
        ((private++)) || true
        continue
      fi

      cache_set "$cache_key" "$sha" "$comment"
    fi
  fi

  new_uses_base="${gha_owner}/${gha_repo}${gha_path}@${sha}"

  if [[ "$new_uses_base" == "$uses_val" ]]; then
    continue
  fi

  awk -v ln="$lineno" -v newbase="$new_uses_base" -v newcomment="$comment" '
    BEGIN { squote = sprintf("%c", 39) }
    NR == ln {
      if (match($0, /^[[:space:]]*(-[[:space:]]+)?uses[[:space:]]*:[[:space:]]*/)) {
        prefix = substr($0, 1, RLENGTH)
        rest = substr($0, RLENGTH + 1)
        quote = substr(rest, 1, 1)

        if (quote == "\"" || quote == squote) {
          value = quote newbase quote
        } else {
          value = newbase
        }

        $0 = prefix value " # " newcomment
      }
    }
    { print }
  ' "$file" > "${file}.tmp"

  if [[ "$DRY_RUN" == true ]]; then
    echo "    [Dry Run] Would pin to: @${sha} # ${comment}"
    rm "${file}.tmp"
  else
    mv "${file}.tmp" "$file"
    echo "    Pinned: @${sha} # ${comment}"
    ((pinned++)) || true
  fi

done < <(extract_all_uses)

echo
echo "==> Summary"
echo "    Total uses: references found : $total"
echo "    Already pinned (SHA)         : $skipped"
echo "    Pinned in this run           : $pinned"
echo "    Resolved from cache          : $cache_hits"
echo "    Hardcoded overrides applied  : $hardcoded_hits"
echo "    Private/inaccessible (logged): $private"
echo "    Failed to resolve            : $failed"

if (( private > 0 )); then
  echo "    Log file: $PRIVATE_LOG"
fi

if [[ "$DRY_RUN" == true ]]; then
  echo
  echo "==> Dry-run mode completed. No files were modified."
else
  echo
  echo "==> Done. Files have been updated in-place."
fi
