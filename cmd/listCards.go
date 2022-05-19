package main

import (
	"fmt"

	"github.com/yobert/alsa"
)

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
