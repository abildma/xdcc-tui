
bin/xdcc-tui: ./**/*.go
	go build -o bin/xdcc-tui cmd/main.go

.PHONY: run
run: bin/xdcc-tui
	./bin/xdcc-tui

.PHONY: install
install: bin/xdcc-tui
	sudo cp bin/xdcc-tui /usr/local/bin/
	sudo chmod +x /usr/local/bin/xdcc-tui