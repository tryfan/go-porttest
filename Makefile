all: porttest sender receiver

porttest:
	go build -o bin/porttest ./cmd/porttest

sender:
	go build -o bin/sender ./cmd/sender

receiver:
	go build -o bin/receiver ./cmd/receiver

clean:
	rm -f bin/*