#!/bin/sh

# Read changed repository paths from stdin and emit Mailclown validation scopes.

go_changed=false
deps_changed=false

while IFS= read -r path; do
  [ -z "$path" ] && continue

  case "$path" in
  go.mod | go.sum)
    go_changed=true
    deps_changed=true
    ;;
  *.go)
    go_changed=true
    ;;
  esac
done

[ "$go_changed" = true ] && printf '%s\n' go
[ "$deps_changed" = true ] && printf '%s\n' deps
exit 0
