#!/usr/bin/env sh
mkdir ./builder/buildpkt/bin 2>/dev/null || true
( cd ./builder/buildpkt && go build -o ./bin ./... ) && exec ./builder/buildpkt/bin/build $@