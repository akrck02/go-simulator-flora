package flora

import (
	"github.com/akrck02/go-simulator/plugin"
	"github.com/akrck02/go-simulator/world"
)

// Thresholds that decide what a cell is, for the vegetation field and the indicators.
const (
	forestFrom    = 0.3 // tree cover from which a cell is forest
	shrublandFrom = 0.3 // shrub cover from which a cell without forest is shrubland
	grasslandFrom = 0.2 // herb cover from which a cell without trees or shrubs is grassland
	bare          = -1
)

// publishNames tells the other plugins and the scene what every value of the vegetation field is.
func (vegetation *Vegetation) publishNames(context *plugin.Context) {
	names, labels, forms := make([]any, 0, len(vegetation.species)), make([]any, 0, len(vegetation.species)), make([]any, 0, len(vegetation.species))
	heights := make([]any, 0, len(vegetation.species))
	for _, plant := range vegetation.species {
		names = append(names, plant.Identifier)
		labels = append(labels, plant.Name)
		forms = append(forms, plant.Form.String())
		heights = append(heights, plant.Height)
	}
	context.Emit(world.EventVegetation, map[string]any{"names": names, "labels": labels, "forms": forms, "heights": heights})
}

// publishFields puts the vegetation in the world: what grows in every cell and how much of the
// ground every layer covers, and, when asked, the cover of every species.
func (vegetation *Vegetation) publishFields(context *plugin.Context) {
	cells := vegetation.landscape.Columns * vegetation.landscape.Rows
	dominant := make([]float32, cells)
	for index := range dominant {
		dominant[index] = bare
	}
	layerFields := [layers][]float32{make([]float32, cells), make([]float32, cells), make([]float32, cells)}
	for position, cell := range vegetation.landCells {
		layerCover := vegetation.layerCover(position)
		for layer := range layerFields {
			layerFields[layer][cell] = float32(layerCover[layer])
		}
		dominant[cell] = float32(vegetation.dominant(position, layerCover))
	}
	context.SetField(vegetation.field(world.FieldVegetation, dominant))
	context.SetField(vegetation.field(world.FieldTreeCover, layerFields[Tree]))
	context.SetField(vegetation.field(world.FieldShrubCover, layerFields[Shrub]))
	context.SetField(vegetation.field(world.FieldHerbCover, layerFields[Herb]))
	if vegetation.settings.SpeciesFields {
		vegetation.publishSpeciesFields(context)
	}
}

func (vegetation *Vegetation) publishSpeciesFields(context *plugin.Context) {
	cells := vegetation.landscape.Columns * vegetation.landscape.Rows
	for speciesIndex, plant := range vegetation.species {
		values := make([]float32, cells)
		for position, cell := range vegetation.landCells {
			values[cell] = vegetation.cover[speciesIndex][position]
		}
		context.SetField(vegetation.field("cover_"+plant.Identifier, values))
	}
}

func (vegetation *Vegetation) field(name string, values []float32) *world.FieldPatch {
	extent := vegetation.landscape.Extent()
	return &world.FieldPatch{Name: name, Columns: vegetation.landscape.Columns, Rows: vegetation.landscape.Rows, Extent: &extent, Values: values}
}

// dominant is the species that most marks a cell: the commonest tree in a forest, else the
// commonest shrub in a shrubland, else the commonest herb in a grassland; bare otherwise.
func (vegetation *Vegetation) dominant(position int, layerCover [layers]float64) int {
	switch {
	case layerCover[Tree] >= forestFrom:
		return vegetation.commonest(position, Tree)
	case layerCover[Shrub] >= shrublandFrom:
		return vegetation.commonest(position, Shrub)
	case layerCover[Herb] >= grasslandFrom:
		return vegetation.commonest(position, Herb)
	}
	return bare
}

func (vegetation *Vegetation) commonest(position int, form Form) int {
	best, bestCover := bare, float32(0)
	for speciesIndex, plant := range vegetation.species {
		if plant.Form == form && vegetation.cover[speciesIndex][position] > bestCover {
			best, bestCover = speciesIndex, vegetation.cover[speciesIndex][position]
		}
	}
	return best
}

// Indicators measures the landscape: the hectares every species covers, the hectares of forest,
// shrubland, grassland and bare ground and, for every region anchor, its share of forest and the
// species that covers most of it.
func (vegetation *Vegetation) Indicators() map[string]any {
	hectaresPerCell := vegetation.landscape.Cell * vegetation.landscape.Cell * 100 // km² → ha
	measures := map[string]any{}
	kinds := map[string]float64{"forest_hectares": 0, "shrubland_hectares": 0, "grassland_hectares": 0, "bare_hectares": 0}
	for position := range vegetation.landCells {
		kinds[kindOf(vegetation.layerCover(position))] += hectaresPerCell
	}
	for name, hectares := range kinds {
		measures[name] = round(hectares)
	}
	for speciesIndex, plant := range vegetation.species {
		total := 0.0
		for _, cover := range vegetation.cover[speciesIndex] {
			total += float64(cover)
		}
		measures["hectares_"+plant.Identifier] = round(total * hectaresPerCell)
	}
	for _, area := range vegetation.regions {
		vegetation.measureRegion(area, measures)
	}
	return measures
}

func (vegetation *Vegetation) measureRegion(area region, measures map[string]any) {
	if len(area.cells) == 0 {
		return
	}
	forest := 0
	totals := make([]float64, len(vegetation.species))
	for _, position := range area.cells {
		if vegetation.layerCover(position)[Tree] >= forestFrom {
			forest++
		}
		for speciesIndex := range vegetation.species {
			totals[speciesIndex] += float64(vegetation.cover[speciesIndex][position])
		}
	}
	measures["forest_share_"+area.identifier] = round(float64(forest) / float64(len(area.cells)))
	best := 0
	for speciesIndex, total := range totals {
		if total > totals[best] {
			best = speciesIndex
		}
	}
	measures["commonest_"+area.identifier] = vegetation.species[best].Identifier
}

func kindOf(layerCover [layers]float64) string {
	switch {
	case layerCover[Tree] >= forestFrom:
		return "forest_hectares"
	case layerCover[Shrub] >= shrublandFrom:
		return "shrubland_hectares"
	case layerCover[Herb] >= grasslandFrom:
		return "grassland_hectares"
	}
	return "bare_hectares"
}

// round keeps two decimals, so indicators stay readable.
func round(value float64) float64 { return float64(int64(value*100+0.5)) / 100 }
