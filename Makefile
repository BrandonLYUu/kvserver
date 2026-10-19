VERB=-v

EXE := $(shell go env GOEXE)

all: kvsrv1 lock1

kvsrv1-build:
	go build -o main/kvsrv1d$(EXE) ./main

kvsrv1: kvsrv1-build
	go test $(VERB) .

lock1: kvsrv1-build
	go test $(VERB) ./lock

.PHONY: all kvsrv1 lock1 kvsrv1-build
