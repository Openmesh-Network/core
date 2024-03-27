#!/bin/bash
# Build binary file and move it to test/bft
if [ -d openmesh-core ]; then
  rm -f openmesh-core
fi
cd ../.. && GOOS=linux GOARCH=amd64 go build -o openmesh-core
mv openmesh-core ./test/bft/openmesh-core

# Build docker image
cd test/bft && docker build -t openmesh-core:latest .
