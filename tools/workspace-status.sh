#!/bin/sh -e

if git diff-index --quiet origin/master --; then
  echo "BUILD_SCM_REVISION $(git rev-parse --short HEAD)"
  echo "BUILD_SCM_TIMESTAMP $(TZ=UTC date --date "@$(git show -s --format=%ct HEAD)" +%Y%m%dT%H%M%SZ)"
fi
