package main

import (
	"fmt"
	"os"

	"github.com/yobert/alsa"
)

func stderr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", a...)
}

func main() {
	cards, err := alsa.OpenCards()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer alsa.CloseCards(cards)

	for _, card := range cards {
		fmt.Println(card)
	}
}
