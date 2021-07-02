package main

import (
	"fmt"
	"time"

	gbgpio "gobot.io/x/gobot/drivers/gpio"
	"gobot.io/x/gobot/platforms/raspi"
)

// Init initialize gpio.
func (g *GPIO) Init() error {
	adaptor := raspi.NewAdaptor()
	adaptor.Connect()

	go g.led(adaptor)

	return nil
}

func (g *GPIO) led(adaptor *raspi.Adaptor) {
	l := gbgpio.NewLedDriver(adaptor, "7")
	err := l.Start()
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	}
	defer l.Halt()

	for {
		l.Toggle()
		time.Sleep(1000 * time.Millisecond)
	}
}

func (g *GPIO) chipMoneyBox(adaptor *raspi.Adaptor) {
	/*
		var (
			state int
		)

		b := gbgpio.NewDirectPinDriver(adaptor, g.ChipMoneyPin)
		err := b.Start()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
		}
		defer b.Halt()

		if g.ChipMoneyContact == `nc` {
			state = 0
		} else {
			state = 1
		}

		for {
			p, err := b.DigitalRead()
			if err != nil {
				logger.queue <- fmt.Sprint(err)
				return
			}
			time.Sleep(time.Millisecond * time.Duration(g.ChipMoneyPulseDuration / 2))
		}
		// userInterface.chipOrMoneyInserted(m[`amount`], m[`songs`])
	*/
}
