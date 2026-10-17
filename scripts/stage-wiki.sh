#!/bin/sh
# Replace only the owned packaged source bundle; runtime state is elsewhere.
set -eu
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
source_root=${JSON2PPTX_WIKI_REPO:-"$script_dir/.."}
source_root=$(CDPATH='' cd -- "$source_root" && pwd -P)
if [ "$#" -ne 1 ]; then echo 'Usage: stage-wiki.sh DESTINATION' >&2; exit 1; fi
destination=$1
case "$destination" in ''|/|.|..) echo 'Choose a dedicated wiki source destination.' >&2; exit 1;; esac
parent=$(dirname -- "$destination")
mkdir -p "$parent"
parent=$(CDPATH='' cd -- "$parent" && pwd -P)
destination="$parent/$(basename -- "$destination")"
case "$source_root/" in "$destination/"*) echo 'Cannot replace the source checkout or its ancestor.' >&2; exit 1;; esac
stage=$(mktemp -d "$parent/.wiki-package.XXXXXX")
backup=$(mktemp -d "$parent/.wiki-backup.XXXXXX")
cleanup() {
    if [ -d "$backup/previous" ] && [ ! -e "$destination" ]; then mv "$backup/previous" "$destination"; fi
    rm -rf "$stage" "$backup"
}
trap cleanup EXIT HUP INT TERM
mkdir -p "$stage/docs" "$stage/scripts" "$stage/deploy"
cp "$source_root/wiki.toml" "$source_root/Dockerfile.wiki" "$stage/"
cp -R "$source_root/schemas" "$stage/schemas"
cp -R "$source_root/docs/wiki" "$stage/docs/wiki"
cp "$source_root/scripts/wiki.sh" "$source_root/scripts/stage-wiki.sh" "$stage/scripts/"
cp "$source_root/deploy/README.md" "$source_root/deploy/mcp.wiki.example.json" "$source_root/deploy/docker-compose.wiki.yml" "$stage/deploy/"
# Public documentation source must remain readable after a global/sudo install,
# independent of mktemp's0700 default or the installer's restrictive umask.
# This normalization applies only to the packaged bundle, never runtime state.
find "$stage" -type d -exec chmod 0755 {} +
find "$stage" -type f -exec chmod 0644 {} +
chmod 0755 "$stage/scripts/wiki.sh" "$stage/scripts/stage-wiki.sh"
if [ -e "$destination" ]; then mv "$destination" "$backup/previous"; fi
mv "$stage" "$destination"
