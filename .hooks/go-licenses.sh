#!/bin/bash

check_licenses() {
	action=$1

	if ! command -v go-licenses &>/dev/null; then
		go install github.com/google/go-licenses@latest
	fi

	if [[ $action == "check_forbidden" ]]; then
		echo "Checking for forbidden licenses..."
		output=$(go-licenses check ./... 2>/dev/null)
		if [[ "${output}" == *"ERROR: forbidden license found"* ]]; then
			echo "Forbidden licenses found. Please remove them."
			exit 1
		else
			echo "No forbidden licenses found."
		fi
	elif [[ $action == "output_csv" ]]; then
		echo "Outputting licenses to csv..."
		go-licenses csv ./... 2>/dev/null
	fi
}

if [[ $# -lt 1 ]]; then
	echo "Incorrect number of arguments."
	echo "Usage: $0 <licenses action>"
	echo "Example: $0 check_forbidden"
	exit 1
fi

check_licenses "${1}"
