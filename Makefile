
SRC=$(wildcard *.go)

flipper: $(SRC)
	go build $(SRC)

clean:
	rm -f flipper
