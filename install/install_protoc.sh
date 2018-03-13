#!/bin/bash
mkdir protoc
cd protoc
wget -O protoc.zip https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip
unzip protoc.zip
sudo mv -f bin/* /usr/bin/
sudo mv -f include/google /usr/local/include/
cd ..
rm -rf protoc
