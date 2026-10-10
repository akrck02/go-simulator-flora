// Command go-simulator-flora is a go-simulator plugin that grows the vegetation of a landscape:
// where every species can live, from the climate and the relief in the world, and how the plants
// spread, compete for light, age and come back after a disturbance, year by year. The host starts
// it; it talks over standard input and output.
package main

import (
	"log"

	"github.com/akrck02/go-simulator-flora/flora"
)

func main() {
	log.SetFlags(0)
	if err := flora.Plugin().Serve(); err != nil {
		log.Fatal(err)
	}
}
