#!/usr/bin/env bash

set -exo pipefail

./scripts/skynet/skynet_setup.sh

export FRUGAL_HOME=$GOPATH/src/github.com/Workiva/frugal
cd ${FRUGAL_HOME}

# TODO: Move all of this to Makefile
# Remove any leftover log files (necessary for skynet-cli)
rm -rf test/integration/log/*

# rm any existing generated code (necessary for skynet-cli)
rm -rf test/integration/go/gen/*
rm -rf test/integration/java/frugal-integration-test/gen-java/*
rm -rf test/integration/python/tornado/gen_py_tornado/*
rm -rf test/integration/python/ascynio/gen_py_asyncio/*
rm -rf test/integration/python/vanilla/gen_py/*
rm -rf test/integration/dart/gen-dart/*

frugal --gen go:package_prefix=github.com/Workiva/frugal/ -r --out='test/integration/go/gen' test/integration/frugalTest.frugal
frugal --gen java -r --out='test/integration/java/frugal-integration-test/gen-java' test/integration/frugalTest.frugal
frugal --gen py:tornado -r --out='test/integration/python/tornado/gen_py_tornado' test/integration/frugalTest.frugal
frugal --gen py:asyncio -r --out='test/integration/python/asyncio/gen_py_asyncio' test/integration/frugalTest.frugal
frugal --gen py -r -out='test/integration/python/tornado/gen-py' test/integration/frugalTest.frugal
frugal --gen dart:use_enums=true -r --out='test/integration/dart/gen-dart' test/integration/frugalTest.frugal

# integration tests use logrus, but frugal does not so it won't be in /vendor
go get github.com/Sirupsen/logrus

# Set everything up in parallel (code generation is fast enough to not require in parallel)
go run scripts/skynet/cross/cross_setup.go

# Run cross tests - want to report any failures, so don't allow command to exit
# without cleaning up
cd ${FRUGAL_HOME}/test/integration

if go run main.go --tests tests.json --outDir log; then
    /testing/scripts/skynet/test_cleanup.sh
else
    /testing/scripts/skynet/test_cleanup.sh
    exit 1
fi
