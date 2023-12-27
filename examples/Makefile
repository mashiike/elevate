
bootstrap: *.go ../*.go ../go.mod ../go.sum
	GOOS=linux GOARCH=arm64 go build -o bootstrap *.go

.PHONY: clean
clean:
	rm -f bootstrap

.PHONY: deploy
deploy: bootstrap
	lambroll deploy
