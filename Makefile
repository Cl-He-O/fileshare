.PHONY: fileshare

fileshare:
	CGO_ENABLED=0 go build -v -trimpath -ldflags "-s -w -buildid="
