default: build

build:
    go build -o conch ./cmd/conch
    go build -o conchd ./cmd/conchd

install:
    go install ./cmd/conch
    go install ./cmd/conchd

test:
    go test ./...

link:
    ./tooling/link.sh

clean:
    rm -f conch conchd
