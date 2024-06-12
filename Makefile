install:
	go install github.com/go-swagger/go-swagger/cmd/swagger@latest

swagger:
	GO111MODULE=off swagger generate spec -o ./swagger.yaml --scan-models

run:
	go run main.go