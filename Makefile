all: bin/findCard bin/listCards bin/listDevices

bin/findCard: cmd/findCard.go
	go build -o bin/findCard cmd/findCard.go
bin/listCards: cmd/listCards.go
	go build -o bin/listCards cmd/listCards.go
bin/listDevices: cmd/listDevices.go
	go build -o bin/listDevices cmd/listDevices.go

clean:
	rm bin/*
