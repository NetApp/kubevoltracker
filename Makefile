# Copyright 2016 NetApp, Inc. All Rights Reserved.
GOOS=linux
GOARCH=amd64
GOGC=""

GO_PATH_VOLUME=kubevoltracker_go_path
GO=docker run --rm \
	-e GOOS=$(GOOS) \
	-e GOARCH=$(GOARCH) \
	-e GOGC=$(GOGC) \
	-v $(GO_PATH_VOLUME):/go \
	-v "$(PWD)":/go/src/github.com/netapp/kubevoltracker \
	-w /go/src/github.com/netapp/kubevoltracker \
	golang:1.6 go

DR=docker run --rm -it \
	-e GOOS=$(GOOS) \
	-e GOARCH=$(GOARCH) \
	-e GOGC=$(GOGC) \
	-v $(GO_PATH_VOLUME):/go \
	-v "$(PWD)":/go/src/github.com/netapp/netappdvp \
	-w /go/src/github.com/netapp/netappdvp \
	golang:1.6 

.PHONY=clean default fmt get install test

default: build

glide:
	curl https://glide.sh/get | sh
	glide install

run:
	@$(DR)

clean:
	rm -f $(PWD)/bin/kubevoltracker
	docker volume rm $(GO_PATH_VOLUME) || true

fmt:
	@$(GO) fmt

build:
	@mkdir -p $(PWD)/bin
	@$(GO) build -x -o /go/src/github.com/netapp/kubevoltracker/bin/kubevoltracker
	cp $(PWD)/bin/kubevoltracker $(PWD)/docker_image/

install: build
	@$(GO) install

test:
	@$(GO) test github.com/netapp/kubevoltracker/...
