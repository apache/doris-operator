#!/bin/bash
#

go test ./... -cover -coverprofile=cover.out
go tool cover -func=cover.out

