.PHONY: test frontend build bindings

test:
	go test -race ./...
	npm --prefix frontend test

frontend:
	npm --prefix frontend ci
	npm --prefix frontend run build

build:
	go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -nopackage -m

bindings:
	go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -nopackage -m
