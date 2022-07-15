#!/usr/bin/env sh
( cd ./builder/buildpkt && go build -o ./bin ./... ) && exec ./builder/buildpkt/bin/build $@