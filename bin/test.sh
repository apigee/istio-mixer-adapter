#!/usr/bin/env bash

SCRIPTPATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOTDIR="$(dirname "$SCRIPTPATH")"

DIRS=(
  "$ROOTDIR/apigee"
  "$ROOTDIR/mixer"
)

for DIR in "${DIRS[@]}"; do
  echo "$DIR"
  cd $DIR
  
  if go test -coverprofile=coverage.txt ./...; then
    if [ -f coverage.txt ]; then
      cat coverage.txt >> ../coverage.txt
    fi
  else
    exit 1
  fi
done
