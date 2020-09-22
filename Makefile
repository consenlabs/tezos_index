APP=tezos_index
build: clean
	go build -o ${APP} ./cmd/main.go

run:
	go run -race ./cmd/main.go
clean:
	go clean