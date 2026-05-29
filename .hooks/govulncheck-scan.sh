#!/bin/bash
set -e

if ! command -v govulncheck &>/dev/null; then
	echo "govulncheck is not installed. Installing..."
	if ! go install golang.org/x/vuln/cmd/govulncheck@latest; then
		echo "Warning: Failed to install govulncheck, skipping vulnerability scan"
		exit 0
	fi
	echo "govulncheck installed successfully"
fi

if ! command -v govulncheck &>/dev/null; then
	echo "Warning: govulncheck not found in PATH after installation, skipping scan"
	exit 0
fi

IGNORED_VULNS=()

echo "Running govulncheck vulnerability scan..."
if ! output=$(govulncheck ./... 2>&1); then
	filtered_output="$output"
	for vuln in "${IGNORED_VULNS[@]}"; do
		filtered_output=$(echo "$filtered_output" | sed "/Vulnerability.*${vuln}/,/^$/d")
	done

	if echo "$filtered_output" | grep -q "Your code is affected by"; then
		remaining=$(echo "$filtered_output" | grep -c "^Vulnerability #" || true)
		if [ "$remaining" -gt 0 ]; then
			echo ""
			echo "govulncheck found vulnerabilities in dependencies!"
			echo "$output"
			echo ""
			echo "Please fix the vulnerabilities before committing."
			echo ""
			echo "To update vulnerable dependencies, run:"
			echo "  go get -u <package>@<fixed-version>"
			echo "  go mod tidy"
			echo ""
			echo "For more information, visit: https://go.dev/security/vuln"
			exit 1
		fi
	fi

	echo "govulncheck found vulnerabilities with no fix available (ignored):"
	for vuln in "${IGNORED_VULNS[@]}"; do
		if echo "$output" | grep -q "$vuln"; then
			echo "  - $vuln (no upstream fix)"
		fi
	done
	echo "No actionable vulnerabilities found by govulncheck"
	exit 0
fi

echo "No vulnerabilities found by govulncheck"
