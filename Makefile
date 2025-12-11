OUT_DIR = bin
BIN = $(OUT_DIR)/local-registry
MAIN = main.go

build:
	mkdir -p $(OUT_DIR)
	go build -o $(BIN) $(MAIN)

clean:
	rm -v $(BIN)
