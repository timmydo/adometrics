GO?=go
GOFLAGS?=

GOSRC!=find . -name '*.go'
GOSRC+=go.mod go.sum

adometrics: $(GOSRC)
	$(GO) build $(GOFLAGS) \
		-o $@