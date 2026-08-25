.PHONY: test
test:
	go test ./...
	GOFLAGS="-tags=goldmark_v1_attribute" go test .
