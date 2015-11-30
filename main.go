package main

import (
	"net/http"
	"encoding/json"
	"strings"
	"errors"
	"time"
	"log"
)

func main() {
	mw := multiWeatherProvider{
		openWeatherMap{apiKey: "your-key-here"},
		weatherUnderground{apiKey: "your-key-here"},
	}
	http.HandleFunc("/weather/", func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		city := strings.SplitN(r.URL.Path, "/", 3)[2]

		temp, err := mw.temperature(city)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"city": city,
			"temp": temp,
			"took": time.Since(begin).String(),
		})
	})
	http.ListenAndServe(":8080", nil)
}

// Create interface (like protocol) that requires a temperature function
type weatherProvider interface {
	temperature(city string) (float64, error) // in Kelvin
}

// Create func to query multiple weather providers and return the average temp.
type multiWeatherProvider []weatherProvider

func (w multiWeatherProvider) temperature(city string) (float64, error) {
	// Make a channel for temperatures and one for errors.
	// Each provider will push a value into only one.
	// Similar to a SignalProducer.buffer
	temps := make(chan float64, len(w))
	errs := make(chan error, len(w))

	for _, provider := range w {
		go func(p weatherProvider) {
			k, err := p.temperature(city)
			if err != nil {
				errs <- err
				return
			}

			temps <- k
		}(provider)
	}

	sum := 0.0

	for i := 0; i < len(w); i++ {
		select {
		case temp := <-temps:
			sum += temp
		case err := <- errs:
			return 0, err
		}
	}

	return sum / float64(len(w)), nil
}

// 
type multiWeatherProviders []weatherProvider

// Create OpenWeatherMap-backed weatherProvider implementation
// NOTE: The tutorial currently doesn't have an API key here, but it needs one.
type openWeatherMap struct {
	apiKey string
}

func (w openWeatherMap) temperature(city string) (float64, error) {
	if w.apiKey == "your-key-here" {
		return 0, errors.New("No API key. Open main.go and set an API key.")
	}
	resp, err := http.Get("http://api.openweathermap.org/data/2.5/weather?APPID=" + w.apiKey + "&q=" + city)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	var d struct {
		Main struct {
			Kelvin float64 `json:"temp"`
		} `json:"main"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}

	log.Printf("openWeatherMap: %s: %.2f", city, d.Main.Kelvin)
	return d.Main.Kelvin, nil
}

// Create Weather Underground-backed weatherProvider implementation
type weatherUnderground struct {
	apiKey string
}

func (w weatherUnderground) temperature(city string) (float64, error) {
	if w.apiKey == "your-key-here" {
		return 0, errors.New("No API key. Open main.go and set an API key.")
	}
	resp, err := http.Get("http://api.wunderground.com/api/" + w.apiKey + "/conditions/q/" + city + ".json")
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	var d struct {
		Observation struct {
			Celsius float64 `json:"temp_c"`
		} `json:"current_observation"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}

	kelvin := d.Observation.Celsius + 273.15
	log.Printf("weatherUnderground: %s: %.2f", city, kelvin)
	return kelvin, nil
}
