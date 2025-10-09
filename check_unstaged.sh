#!/usr/bin/env bash
set -euo pipefail

FILES=()
IFS=$'\n' read -r -d '' -a FILES < <(git ls-files --other --modified --exclude-standard && printf '\0')
if [[ ${#FILES[@]} -gt 0 ]]; then

	echo
	echo "The following files contain unstaged changes:"
	echo
	for file in "${FILES[@]}"; do
		echo "  - $file"
	done

	echo
	echo "These are the changes:"
	echo
	for file in "${FILES[@]}"; do
		git --no-pager diff -- "$file" 1>&2
	done

	echo
	echo "ERROR: Unstaged changes, see above for details."
	exit 1
fi
