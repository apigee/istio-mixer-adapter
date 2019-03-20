#!/usr/bin/env bash

if [[ `command -v protoc` == "" ]]; then
  mkdir /tmp/protoc
  wget -O /tmp/protoc/protoc.zip https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip
  unzip /tmp/protoc/protoc.zip -d /tmp/protoc
  sudo mv -f /tmp/protoc/bin/protoc /usr/bin/
  sudo mv -f /tmp/protoc/include/google /usr/local/include/
  rm -rf /tmp/protoc
fi
