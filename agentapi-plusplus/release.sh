#!/usr/bin/env bash
set -euo pipefail

GIT_BRANCH="${GIT_BRANCH:-main}"
echo "GIT_BRANCH=${GIT_BRANCH}"
LOCAL_HEAD=$(git rev-parse --short "${GIT_BRANCH}")
echo "LOCAL_HEAD=${LOCAL_HEAD}"
REMOTE_HEAD=$(git rev-parse --short origin/"${GIT_BRANCH}")
echo "REMOTE_HEAD=${REMOTE_HEAD}"
if [[ "${LOCAL_HEAD}" != "${REMOTE_HEAD}" ]]; then
    echo "Please ensure your local branch is up to date before continuing."
    exit 1
fi

VERSION=""
if ! VERSION=$(./version.sh); then
    echo "version.sh exited with a non-zero status code. Fix this before continuing."
    exit 1
elif [[ -z "${VERSION}" ]]; then
    echo "Version reported by version.sh was empty. Fix this before continuing."
    exit 1
fi

echo "VERSION=${VERSION}"
./check_unstaged.sh || exit 1

if ! grep -q "## v${VERSION}" CHANGELOG.md; then
   echo "Please update CHANGELOG.md with details for ${VERSION} before continuing."
   exit 1
fi

TAG_NAME="v${VERSION}"
echo "TAG_NAME=${TAG_NAME}"
git fetch --tags
git tag -a "${TAG_NAME}" -m "${TAG_NAME}"
git push origin tag "${TAG_NAME}"

echo "https://github.com/coder/agentapi/releases/new?tag=${TAG_NAME}"
