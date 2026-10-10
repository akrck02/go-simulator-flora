package flora

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/akrck02/go-simulator/conditions"
	"github.com/akrck02/go-simulator/world"
)

// A habitat is written like a go-simulator condition, over the facts of a cell:
//
//	habitat:
//	  temperature: {min: 9, max: 18}         # °C, the yearly mean
//	  precipitation: {min: 900}              # mm a year
//	  elevation: {max: 1500}                 # metres
//	  slope: {max: 0.8}                      # rise over run
//	  biome: [laurel_forest, riverbank]      # one of the climate's biomes
//	  distance_to_old_oak: {max: 60}      # metres from an anchor of the rules file
//
// Inside every range the habitat suits the species fully. Past an edge it fades to nothing over
// the fact's softness (Settings.Softness, in the fact's unit; none: a hard edge). Values that are
// not ranges (a biome, a list of biomes) hold or not. A cell suits a species as much as the least
// suiting of its facts allows, times the others: the product of every fact's suitability.

// distancePrefix starts the facts that measure the distance to an anchor.
const distancePrefix = "distance_to_"

// anchorShape is the ground shape of an anchor, in world units.
type anchorShape struct {
	point   *[2]float64
	outline [][2]float32
	line    [][2]float32
}

// facts gives the facts of the cells of a landscape.
type facts struct {
	landscape *Landscape
	distances map[string][]float32 // metres to every anchor a habitat names, by anchor id
}

// newFacts measures the distances the species' habitats ask for.
func newFacts(landscape *Landscape, species []Species, anchors map[string]anchorShape, metresPerUnit float64) (*facts, error) {
	cellFacts := &facts{landscape: landscape, distances: map[string][]float32{}}
	for _, plant := range species {
		for name := range plant.Habitat {
			if err := cellFacts.prepare(plant.Identifier, name, anchors, metresPerUnit); err != nil {
				return nil, err
			}
		}
		if err := conditions.Check(plant.Habitat, nil); err != nil {
			return nil, fmt.Errorf("species %q: %w", plant.Identifier, err)
		}
	}
	return cellFacts, nil
}

// prepareManaged makes sure the facts the managed land asks about can be read.
func (cellFacts *facts) prepareManaged(managed []Managed, anchors map[string]anchorShape, metresPerUnit float64) error {
	for index, land := range managed {
		for name := range land.Where {
			if err := cellFacts.prepare(fmt.Sprintf("managed land %d", index+1), name, anchors, metresPerUnit); err != nil {
				return err
			}
		}
	}
	return nil
}

// prepare makes sure a fact a habitat names can be read, measuring distances when it is one.
func (cellFacts *facts) prepare(species, name string, anchors map[string]anchorShape, metresPerUnit float64) error {
	anchor, isDistance := strings.CutPrefix(name, distancePrefix)
	if !isDistance {
		return cellFacts.checkKnown(species, name)
	}
	if _, measured := cellFacts.distances[anchor]; measured {
		return nil
	}
	shape, found := anchors[anchor]
	if !found {
		return fmt.Errorf("species %q: no anchor %q in the rules file for %q", species, anchor, name)
	}
	cellFacts.distances[anchor] = cellFacts.measureDistances(shape, metresPerUnit)
	return nil
}

func (cellFacts *facts) checkKnown(species, name string) error {
	if name == world.FieldBiome {
		return nil
	}
	if !slices.Contains(numericFacts, name) {
		return fmt.Errorf("species %q: unknown habitat fact %q (known: %v, %s and %s<anchor>)", species, name, numericFacts, world.FieldBiome, distancePrefix)
	}
	if _, present := cellFacts.landscape.numbers[name]; !present {
		return fmt.Errorf("species %q: its habitat needs the %s field, which no plugin publishes", species, name)
	}
	return nil
}

func (cellFacts *facts) measureDistances(shape anchorShape, metresPerUnit float64) []float32 {
	distances := make([]float32, cellFacts.landscape.Columns*cellFacts.landscape.Rows)
	for index := range distances {
		x, z := cellFacts.landscape.Centre(index)
		distances[index] = float32(shape.distance(x, z) * metresPerUnit)
	}
	return distances
}

// of returns the facts of one cell.
func (cellFacts *facts) of(index int) conditions.Facts {
	return func(name string) (any, bool) {
		if anchor, isDistance := strings.CutPrefix(name, distancePrefix); isDistance {
			return float64(cellFacts.distances[anchor][index]), true
		}
		if name == world.FieldBiome {
			biome := cellFacts.landscape.Biome(index)
			return biome, biome != ""
		}
		values, present := cellFacts.landscape.numbers[name]
		if !present {
			return nil, false
		}
		return float64(values[index]), true
	}
}

// suitability tells how well a cell suits a habitat, from 0 to 1.
func suitability(habitat map[string]any, cellFacts conditions.Facts, softness map[string]float64) float64 {
	result := 1.0
	for name, wanted := range habitat {
		value, exists := cellFacts(name)
		if !exists {
			return 0
		}
		result *= factSuitability(wanted, value, softness[softnessName(name)])
		if result == 0 {
			return 0
		}
	}
	return result
}

// softnessName is the name a fact's softness is given under: distances share "distance".
func softnessName(name string) string {
	if strings.HasPrefix(name, distancePrefix) {
		return "distance"
	}
	return name
}

func factSuitability(wanted, value any, softness float64) float64 {
	bounds, isRange := wanted.(map[string]any)
	if !isRange {
		return booleanSuitability(conditions.Holds(map[string]any{"fact": wanted}, conditions.OfMap(map[string]any{"fact": value})))
	}
	number, isNumber := conditions.Number(value)
	if !isNumber {
		return 0
	}
	return rangeSuitability(bounds, number, softness)
}

// rangeSuitability is 1 inside the range and fades linearly to 0 over the softness past an edge.
func rangeSuitability(bounds map[string]any, value, softness float64) float64 {
	outside := 0.0
	if minimum, present := conditions.Number(bounds["min"]); present && value < minimum {
		outside = minimum - value
	}
	if maximum, present := conditions.Number(bounds["max"]); present && value > maximum {
		outside = value - maximum
	}
	if outside == 0 {
		return 1
	}
	if softness <= 0 {
		return 0
	}
	return math.Max(0, 1-outside/softness)
}

func booleanSuitability(holds bool) float64 {
	if holds {
		return 1
	}
	return 0
}

// distance from a point to the shape, in world units: 0 inside an outline.
func (shape anchorShape) distance(x, z float64) float64 {
	switch {
	case shape.point != nil:
		return math.Hypot(x-shape.point[0], z-shape.point[1])
	case len(shape.outline) >= 3:
		if inside(shape.outline, x, z) {
			return 0
		}
		return distanceToSegments(closed(shape.outline), x, z)
	}
	return distanceToSegments(shape.line, x, z)
}

func closed(outline [][2]float32) [][2]float32 { return append(slices.Clone(outline), outline[0]) }

func distanceToSegments(points [][2]float32, x, z float64) float64 {
	nearest := math.Inf(1)
	for index := 1; index < len(points); index++ {
		nearest = math.Min(nearest, distanceToSegment(points[index-1], points[index], x, z))
	}
	return nearest
}

func distanceToSegment(start, end [2]float32, x, z float64) float64 {
	startX, startZ, endX, endZ := float64(start[0]), float64(start[1]), float64(end[0]), float64(end[1])
	alongX, alongZ := endX-startX, endZ-startZ
	length := alongX*alongX + alongZ*alongZ
	share := 0.0
	if length > 0 {
		share = clamp(((x-startX)*alongX+(z-startZ)*alongZ)/length, 0, 1)
	}
	return math.Hypot(x-(startX+share*alongX), z-(startZ+share*alongZ))
}

// inside tells whether a point is inside a polygon (even–odd rule).
func inside(outline [][2]float32, x, z float64) bool {
	result := false
	for index, previous := 0, len(outline)-1; index < len(outline); previous, index = index, index+1 {
		firstX, firstZ := float64(outline[index][0]), float64(outline[index][1])
		secondX, secondZ := float64(outline[previous][0]), float64(outline[previous][1])
		if (firstZ > z) != (secondZ > z) && x < (secondX-firstX)*(z-firstZ)/(secondZ-firstZ)+firstX {
			result = !result
		}
	}
	return result
}
