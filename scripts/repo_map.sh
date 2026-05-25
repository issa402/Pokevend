!#/usr/bin/env
set -Eeuo pipefail

root="${1:-.}"

cd "$root"

echo "== repo root =="

pwd
echo 
echo "== git state =="

if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
	git status --short
else
	echo "not a git repository"
fi 

echo 
echo "== high signal files =="

find . -maxdepth 4 -type f \
	\( -name 'package.json' \
	-o -name 'package-lock.json' \
	-o -name 'yarn.lock' \ 
	-o -name 'go.mod' \
	-o -name 'requirements.txt' \
	-o -name 'pyproject.toml' \
	-o -name 'Dockerfile' \
	-o -name 'Dockerfile.*' \
	-o -name 'docker-compose*.yaml' \
	-o -name 'docker-compose*.yml' \
	-o -name 'env.example' \
	-o -name 'AGENTS.md' \
	-o -name 'README.md' \
	| sort
