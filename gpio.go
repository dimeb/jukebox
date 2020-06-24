package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

// GPIO structure.
type GPIO struct {
	ChipMoneyType          string                               `yaml:"chip_money_type,omitempty"`
	ChipMoneyPin           string                               `yaml:"chip_money_pin,omitempty"`
	ChipMoneyPulseDuration int                                  `yaml:"chip_money_pulse_duration,omitempty"`
	ChipMoneyPulsePause    int                                  `yaml:"chip_money_pulse_pause,omitempty"`
	ChipMoneyContact       string                               `yaml:"chip_money_contact,omitempty"`
	ChipMoney              map[string]map[string]map[string]int `yaml:"chip_money,omitempty"`
	LightLEDNumber         int                                  `yaml:"light_led_number,omitempty"`
	gpioFile               string
}

var gpio *GPIO

// NewGPIO creates new GPIO object.
func NewGPIO() (*GPIO, error) {
	var (
		toSave bool
		err    error
		lf     []byte
		data   []byte
	)

	g := &GPIO{
		ChipMoneyType:          `chip`,
		ChipMoneyPin:           `11`,
		ChipMoneyPulseDuration: 20,
		ChipMoneyPulsePause:    100,
		ChipMoneyContact:       `no`,
		ChipMoney: map[string]map[string]map[string]int{
			`chip`: {
				`1`: {
					`pulses`: 3,
					`songs`:  1,
				},
			},
			`money`: {
				`5`: {
					`pulses`: 3,
					`songs`:  1,
				},
				`10`: {
					`pulses`: 5,
					`songs`:  2,
				},
			},
		},
		LightLEDNumber: 150,
		gpioFile:       `gpio.yaml`,
	}

	_, err = os.Stat(g.gpioFile)
	if os.IsNotExist(err) {
		toSave = true
	} else {
		lf, err = ioutil.ReadFile(g.gpioFile)
		if err == nil {
			err = yaml.Unmarshal(lf, &g)
		}
		if err != nil {
			toSave = true
			logger.queue <- fmt.Sprint(err)
		}
	}
	if toSave {
		data, err = yaml.Marshal(&g)
		if err == nil {
			err = ioutil.WriteFile(g.gpioFile, data, 0644)
		}
	}
	if err != nil {
		return nil, err
	}

	err = g.Init()

	return g, err
}
