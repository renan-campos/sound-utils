package alsa

import "github.com/yobert/alsa"

func FindCard(name string) (*alsa.Card, error) {
	cards, err := alsa.OpenCards()
	if err != nil {
		return nil, err
	}

	for _, card := range cards {
		if card.Title == name {
			return card, nil
		} else {
			CloseCard(card)
		}
	}
	return nil, &cardNotFound{cardName: name}
}

func CloseCard(card *alsa.Card) {
	alsa.CloseCards([]*alsa.Card{card})
}
