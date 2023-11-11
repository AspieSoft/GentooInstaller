#!/bin/bash

go get -u
go mod tidy
# CGO_ENABLED="0" go build
go build

sha256sum gentoo-installer > gentoo-installer.sum
