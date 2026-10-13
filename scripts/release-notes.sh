#!/bin/sh
set -eu

usage() {
  echo "usage: $0 <version>" >&2
  echo "example: $0 v1.0.0" >&2
  exit 2
}

[ "$#" -eq 1 ] || usage

tag="$1"
version=${tag#v}

case "$tag" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *)
    echo "error: version must look like v1.0.0 or v1.0.0-rc.1" >&2
    exit 2
    ;;
esac

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
changelog="$repo_root/CHANGELOG.md"

[ -f "$changelog" ] || {
  echo "error: CHANGELOG.md not found" >&2
  exit 1
}

section=$(awk -v target="$version" '
  BEGIN {
    prefix = "## [" target "]"
    found = 0
  }
  index($0, prefix) == 1 {
    found = 1
    next
  }
  found && /^## \[/ {
    exit
  }
  found && /^\[[^]]+\]:/ {
    exit
  }
  found {
    print
  }
  END {
    if (!found) {
      exit 3
    }
  }
' "$changelog") || {
  echo "error: CHANGELOG.md has no section for [$version]" >&2
  echo "add '## [$version] - YYYY-MM-DD' before creating tag $tag" >&2
  exit 1
}

if [ -z "$(printf '%s' "$section" | tr -d '[:space:]')" ]; then
  echo "error: CHANGELOG.md section [$version] is empty" >&2
  exit 1
fi

previous=""
if command -v git >/dev/null 2>&1 && git -C "$repo_root" rev-parse --git-dir >/dev/null 2>&1; then
  previous=$(git -C "$repo_root" tag --list 'v*' --sort=-v:refname \
    | awk -v current="$tag" '$0 != current { print; exit }')
fi

description="Routurn is an agentless project workflow CLI for local or SSH-based task execution, updates, and results."
model="## Execution model
Routurn runs locally. A project may execute on the local machine without SSH, or on Unix-like remote machines over standard SSH (including ProxyJump routes). Remote machines need no Routurn installation."

cat <<EOF_NOTES
$description

## What's Changed
$section

## Installation

\`\`\`sh
curl -fsSL https://raw.githubusercontent.com/selimserbes/routurn/${tag}/install.sh | ROUTURN_VERSION=${tag} sh
\`\`\`

## Release binaries

Prebuilt archives are published for Linux, macOS, and Windows on amd64 and arm64. Verify downloaded archives with \`checksums.txt\`. On Windows, download the matching ZIP asset, extract \`routurn.exe\`, and place it on \`PATH\`.

$model
EOF_NOTES

if [ -n "$previous" ]; then
  printf '\n**Full Changelog:** https://github.com/selimserbes/routurn/compare/%s...%s\n' "$previous" "$tag"
else
  printf '\n**Full Changelog:** https://github.com/selimserbes/routurn/commits/%s\n' "$tag"
fi
