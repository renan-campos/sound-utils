all: bin/findCard bin/listCards bin/listDevices \
	   bin/beepCard bin/beepDevice bin/wavData \
		 bin/myWavData \
		 bin/playWav bin/recordWav

bin/findCard: cmd/findCard.go
	go build -o bin/findCard cmd/findCard.go
bin/listCards: cmd/listCards.go
	go build -o bin/listCards cmd/listCards.go
bin/listDevices: cmd/listDevices.go
	go build -o bin/listDevices cmd/listDevices.go

bin/beepCard: cmd/beepCard.go
	go build -o bin/beepCard cmd/beepCard.go

bin/beepDevice: cmd/beepDevice.go
	go build -o bin/beepDevice cmd/beepDevice.go

bin/wavData: cmd/wavData.go
	go build -o bin/wavData cmd/wavData.go

bin/myWavData: cmd/myWavData.go
	go build -o bin/myWavData cmd/myWavData.go

bin/playWav: cmd/playWav.go
	go build -o bin/playWav cmd/playWav.go

bin/recordWav: cmd/recordWav.go
	go build -o bin/recordWav cmd/recordWav.go

clean:
	rm bin/*
