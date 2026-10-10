package flora

import (
	"math"

	"github.com/akrck02/go-simulator/conditions"
)

// shadeOfNeighbours is the share of the light the plants around a gap take from it when their
// layer is closed: tall trees shade a small gap, shrubs less, grasses not at all.
var shadeOfNeighbours = [layers]float64{Herb: 0, Shrub: 0.3, Tree: 0.6}

const (
	shadeOfFullCover   = 0.9   // share of the light a closed layer takes from the layers below it
	deathFromStress    = 0.2   // yearly share that dies where the habitat does not suit the species at all
	deathFromShade     = 0.1   // yearly share that dies in shade too deep for the species
	saplingDeath       = 0.1   // yearly share of the saplings that die waiting for room
	presentAbove       = 0.001 // cover from which a species counts as growing in a cell (its age runs): a few plants
	defaultMetresPerKm = 1000
)

// Vegetation is the state of the landscape: in every land cell, the share of the ground each
// species covers, its saplings waiting for room (the advance regeneration of LANDIS) and for how
// many years it has grown there.
//
// Seeds become saplings where the light that reaches the ground suits the species' shade
// tolerance, even under a closed canopy; saplings take the room plants leave when they die. So a
// pioneer that needs full light takes open ground first but cannot follow itself under its own
// shade, and the shade-tolerant species waiting below take over as it ages: succession.
type Vegetation struct {
	settings     *Settings
	landscape    *Landscape
	species      []Species
	landCells    []int       // grid index of every land cell
	suitability  [][]float32 // by species, by land cell
	cover        [][]float32 // by species, by land cell
	saplings     [][]float32 // by species, by land cell: share of the ground with young plants waiting
	yearsPresent [][]float32 // by species, by land cell
	plantings    []planting
	regions      []region
	years        int // years simulated so far
}

// planting is an anchor that fixes a plant: its species covers the cells of its crown or, when
// the crown is smaller than a cell, the share of the cell under it.
type planting struct {
	species int
	cells   []int   // land cell positions
	share   float32 // cover in each of those cells
}

// region is an anchor that names an area for the indicators.
type region struct {
	identifier string
	cells      []int // land cell positions
}

// newVegetation lays bare ground over the land and works out how well every cell suits every species.
func newVegetation(settings *Settings, landscape *Landscape, cellFacts *facts) *Vegetation {
	vegetation := &Vegetation{settings: settings, landscape: landscape, species: settings.Species}
	for index, land := range landscape.land {
		if land {
			vegetation.landCells = append(vegetation.landCells, index)
		}
	}
	count := len(vegetation.landCells)
	for range vegetation.species {
		vegetation.suitability = append(vegetation.suitability, make([]float32, count))
		vegetation.cover = append(vegetation.cover, make([]float32, count))
		vegetation.saplings = append(vegetation.saplings, make([]float32, count))
		vegetation.yearsPresent = append(vegetation.yearsPresent, make([]float32, count))
	}
	for speciesIndex, plant := range vegetation.species {
		suits := vegetation.suitability[speciesIndex]
		inParallel(count, func(position int) {
			cell := vegetation.landCells[position]
			if !managedAllows(settings.Managed, cellFacts.of(cell), plant.Form) {
				return
			}
			suits[position] = float32(suitability(plant.Habitat, cellFacts.of(cell), settings.Softness))
		})
	}
	return vegetation
}

// managedAllows tells whether a form may grow in a cell: everywhere but on managed land that
// keeps it out.
func managedAllows(managed []Managed, cellFacts conditions.Facts, form Form) bool {
	for _, land := range managed {
		if conditions.Holds(land.Where, cellFacts) && !land.allows(form) {
			return false
		}
	}
	return true
}

// Advance simulates some years.
func (vegetation *Vegetation) Advance(years int) {
	for range years {
		vegetation.advanceYear()
	}
}

func (vegetation *Vegetation) advanceYear() {
	seeds := vegetation.seedRain()
	inParallel(len(vegetation.landCells), func(position int) {
		vegetation.growCell(position, seeds)
		vegetation.disturbCell(position)
		vegetation.ageCell(position)
	})
	vegetation.plant()
	vegetation.years++
}

// seedRain returns, for every species and land cell, the seeds that land there from the mature
// plants around it.
func (vegetation *Vegetation) seedRain() [][]float32 {
	landscape := vegetation.landscape
	cellMetres := landscape.Cell * vegetation.metresPerUnit()
	rain := make([][]float32, len(vegetation.species))
	for speciesIndex, plant := range vegetation.species {
		if !plant.SpreadsBySeed() {
			continue
		}
		sources := make([]float32, landscape.Columns*landscape.Rows)
		for position, cell := range vegetation.landCells {
			if vegetation.yearsPresent[speciesIndex][position] >= float32(plant.Maturity) {
				sources[cell] = vegetation.cover[speciesIndex][position]
			}
		}
		landed := spread(sources, landscape.Columns, landscape.Rows, plant.Dispersal/cellMetres)
		rain[speciesIndex] = make([]float32, len(vegetation.landCells))
		for position, cell := range vegetation.landCells {
			rain[speciesIndex][position] = landed[cell]
		}
	}
	return rain
}

// growCell runs a year of establishment, recruitment and death in one cell, from the state it had
// at the start of the year.
func (vegetation *Vegetation) growCell(position int, seeds [][]float32) {
	layerCover := vegetation.layerCover(position)
	grown := make([]float64, len(vegetation.species))
	for speciesIndex := range vegetation.species {
		grown[speciesIndex] = vegetation.grownCover(speciesIndex, position, layerCover, seeds)
	}
	vegetation.storeWithinRoom(position, grown)
}

// grownCover updates a species' saplings and returns its cover after a year: saplings take what
// room the layer has, and plants die of age, of an unsuiting habitat and of deep shade.
func (vegetation *Vegetation) grownCover(speciesIndex, position int, layerCover [layers]float64, seeds [][]float32) float64 {
	plant := &vegetation.species[speciesIndex]
	current := float64(vegetation.cover[speciesIndex][position])
	suits := float64(vegetation.suitability[speciesIndex][position])
	lightAbove := lightFactor(lightBelow(layerCover, plant.Form), plant.lightNeeded())
	saplings := vegetation.grownSaplings(speciesIndex, position, layerCover, seeds)
	room := math.Max(0, 1-layerCover[plant.Form])
	recruited := math.Min(saplings, plant.Growth*saplings*room*suits*lightAbove)
	vegetation.saplings[speciesIndex][position] = float32(saplings - recruited)
	death := current * (1/plant.Longevity + deathFromStress*(1-suits) + deathFromShade*(1-lightAbove))
	return clamp(current+recruited-death, 0, 1)
}

// grownSaplings returns a species' saplings after this year's seeds took and some died. Seeds
// take in the gaps of their layer and under its plants, each with its light: a cell of 100 m is
// not one canopy.
func (vegetation *Vegetation) grownSaplings(speciesIndex, position int, layerCover [layers]float64, seeds [][]float32) float64 {
	plant := &vegetation.species[speciesIndex]
	saplings := float64(vegetation.saplings[speciesIndex][position])
	suits := float64(vegetation.suitability[speciesIndex][position])
	if plant.SpreadsBySeed() {
		arriving := float64(seeds[speciesIndex][position]) + vegetation.settings.BackgroundSeeds
		room := math.Max(0, 1-layerCover[plant.Form])
		inGaps := room * lightFactor(lightInGaps(layerCover, plant.Form), plant.lightNeeded())
		underPlants := (1 - room) * lightFactor(lightOnTheGround(layerCover, plant.Form), plant.lightNeeded())
		saplings += plant.Growth * arriving * suits * (inGaps + underPlants) * (1 - saplings)
	}
	return saplings * (1 - saplingDeath - deathFromStress*(1-suits))
}

// storeWithinRoom keeps the cover of every layer within the ground there is, sharing it out in
// proportion when the species of a layer grew past it.
func (vegetation *Vegetation) storeWithinRoom(position int, grown []float64) {
	var total [layers]float64
	for speciesIndex, plant := range vegetation.species {
		total[plant.Form] += grown[speciesIndex]
	}
	for speciesIndex, plant := range vegetation.species {
		value := grown[speciesIndex]
		if total[plant.Form] > 1 {
			value /= total[plant.Form]
		}
		vegetation.cover[speciesIndex][position] = float32(value)
	}
}

// disturbCell opens a gap in some cells every year: the trees and shrubs lose most of their cover
// and start again.
func (vegetation *Vegetation) disturbCell(position int) {
	disturbance := vegetation.settings.Disturbance
	if disturbance.Chance <= 0 || chance(vegetation.settings.Seed, vegetation.years, position) >= disturbance.Chance {
		return
	}
	for speciesIndex, plant := range vegetation.species {
		if plant.Form == Herb {
			continue
		}
		vegetation.cover[speciesIndex][position] *= float32(1 - disturbance.Severity)
		vegetation.yearsPresent[speciesIndex][position] = 0
	}
}

// ageCell counts the years every species has grown in a cell; a species that left starts again.
func (vegetation *Vegetation) ageCell(position int) {
	for speciesIndex := range vegetation.species {
		if vegetation.cover[speciesIndex][position] < presentAbove {
			vegetation.yearsPresent[speciesIndex][position] = 0
			continue
		}
		vegetation.yearsPresent[speciesIndex][position]++
	}
}

// plant keeps the anchored plants: their species covers their crown, mature, and the other
// species of their layer have no room there.
func (vegetation *Vegetation) plant() {
	for _, fixed := range vegetation.plantings {
		form := vegetation.species[fixed.species].Form
		for _, position := range fixed.cells {
			vegetation.makeRoom(position, form, fixed.share)
			vegetation.cover[fixed.species][position] = fixed.share
			maturity := float32(vegetation.species[fixed.species].Maturity)
			vegetation.yearsPresent[fixed.species][position] = max(vegetation.yearsPresent[fixed.species][position], maturity)
		}
	}
}

// makeRoom shrinks the other species of a layer in a cell so a share of it is free.
func (vegetation *Vegetation) makeRoom(position int, form Form, share float32) {
	for speciesIndex, plant := range vegetation.species {
		if plant.Form == form {
			vegetation.cover[speciesIndex][position] *= 1 - share
		}
	}
}

// layerCover is the cover of every layer in a cell.
func (vegetation *Vegetation) layerCover(position int) [layers]float64 {
	var total [layers]float64
	for speciesIndex, plant := range vegetation.species {
		total[plant.Form] += float64(vegetation.cover[speciesIndex][position])
	}
	return total
}

// lightBelow is the share of the light that reaches a layer through the layers above it.
func lightBelow(layerCover [layers]float64, form Form) float64 {
	above := 0.0
	for layer := max(form+1, 0); layer < layers; layer++ {
		above += layerCover[layer]
	}
	return 1 - shadeOfFullCover*math.Min(1, above)
}

// lightInGaps is the share of the light that reaches the gaps of a layer: what the taller layers
// leave, less the shade of the plants around the gap. Gaps in a closed layer are small and
// shaded; gaps in an open one, or the ground a disturbance cleared, get nearly all of it.
func lightInGaps(layerCover [layers]float64, form Form) float64 {
	return lightBelow(layerCover, form) * (1 - shadeOfNeighbours[form]*math.Min(1, layerCover[form]))
}

// lightOnTheGround is the share of the light that reaches seedlings of a layer: they grow under
// their own layer as well as under the taller ones.
func lightOnTheGround(layerCover [layers]float64, form Form) float64 {
	return lightBelow(layerCover, form-1)
}

// lightFactor is how much of its growth a species keeps with some light: none below what it
// needs, all of it in full light.
func lightFactor(light, needed float64) float64 {
	return clamp((light-needed)/(1-needed), 0, 1)
}

func (vegetation *Vegetation) metresPerUnit() float64 {
	if vegetation.settings.MetresPerUnit > 0 {
		return vegetation.settings.MetresPerUnit
	}
	return defaultMetresPerKm
}

// chance is a number from 0 to 1, the same for the same seed, year and cell (splitmix64), so the
// cells can be worked on in any order.
func chance(seed uint64, year, position int) float64 {
	value := seed ^ uint64(year)*0x9e3779b97f4a7c15 ^ uint64(position)*0xbf58476d1ce4e5b9
	value += 0x9e3779b97f4a7c15
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	value ^= value >> 31
	return float64(value>>11) / float64(uint64(1)<<53)
}
