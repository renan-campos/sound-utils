package main

import (
	"fmt"
	"os"

	"github.com/yobert/alsa"
)

func stderr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

func main() {
	if len(os.Args) < 2 {
		stderr("Card name expected")
		os.Exit(1)
	}

	cards, err := alsa.OpenCards()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer alsa.CloseCards(cards)

	card, err := findCard(cards, os.Args[1])
	if err != nil {
		stderr(err.Error())
		os.Exit(1)
	}
	fmt.Println(card, "found!")
}

type cardNotFound struct{ cardName string }

func (cnf *cardNotFound) Error() string {
	return fmt.Sprintf("Card %q not found", cnf.cardName)
}

func findCard(cards []*alsa.Card, name string) (*alsa.Card, error) {
	for _, card := range cards {
		if card.Title == name {
			return card, nil
		}
	}
	return nil, &cardNotFound{cardName: name}
}
