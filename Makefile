.PHONY: server agent dashboard test up down fmt

server:
	cd server && go build ./... && go build -o server ./cmd/server

agent:
	cd agent && go build ./...
	cd agent && GOOS=windows GOARCH=amd64 go build -o agent.exe ./cmd/agent

dashboard:
	cd dashboard && npm install && npm run build

test:
	cd server && go test ./...
	cd agent && go test ./...

up:
	cd deploy && docker compose up -d

down:
	cd deploy && docker compose down

fmt:
	cd server && go fmt ./...
	cd agent && go fmt ./...
