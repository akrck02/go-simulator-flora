package flora

import (
	"fmt"
	"math"

	"github.com/akrck02/go-simulator/world"
)

// Landscape is the grid the vegetation grows on, with the facts of every cell read from the
// world's fields: rows from the smallest z, columns from the smallest x, values at cell centres.
type Landscape struct {
	Columns, Rows      int
	Cell               float64 // world units (km) between cell centres
	MinimumX, MinimumZ float64 // the first cell's centre
	land               []bool
	numbers            map[string][]float32 // numeric facts by name (temperature, elevation…)
	biomes             []int16              // index into biomeNames, −1 for none
	biomeNames         []string
}

// numericFacts are the fields a habitat can ask about, by the name a habitat uses.
var numericFacts = []string{world.FieldTemperature, world.FieldPrecipitation, world.FieldElevation, world.FieldSlope}

// newLandscape lays a grid over the climate's extent and samples the world's fields on it.
func newLandscape(current *world.World, cell float64, biomeNames []string) (*Landscape, error) {
	base := current.Field(world.FieldTemperature)
	if base == nil {
		return nil, fmt.Errorf("no %s field: put the climate plugin before flora in the rules file", world.FieldTemperature)
	}
	if cell <= 0 {
		cell = (base.Extent.MaximumX - base.Extent.MinimumX) / float64(base.Columns)
	}
	landscape := &Landscape{
		Columns: int(math.Round((base.Extent.MaximumX - base.Extent.MinimumX) / cell)),
		Rows:    int(math.Round((base.Extent.MaximumZ - base.Extent.MinimumZ) / cell)),
		Cell:    cell, MinimumX: base.Extent.MinimumX + cell/2, MinimumZ: base.Extent.MinimumZ + cell/2,
		numbers: map[string][]float32{}, biomeNames: biomeNames,
	}
	for _, name := range numericFacts {
		if field := current.Field(name); field != nil {
			landscape.numbers[name] = landscape.sampled(field, sampleBilinear)
		}
	}
	landscape.readBiomes(current.Field(world.FieldBiome))
	return landscape, nil
}

// readBiomes takes the biome of every cell (the nearest value: categories do not blend) and marks
// land: cells with a biome or, without a biome field, above the sea.
func (landscape *Landscape) readBiomes(field *world.Field) {
	cells := landscape.Columns * landscape.Rows
	landscape.biomes = make([]int16, cells)
	landscape.land = make([]bool, cells)
	if field == nil {
		elevation := landscape.numbers[world.FieldElevation]
		for index := range landscape.land {
			landscape.biomes[index] = -1
			landscape.land[index] = elevation == nil || elevation[index] > 0
		}
		return
	}
	for index, value := range landscape.sampled(field, sampleNearest) {
		landscape.biomes[index] = int16(value)
		landscape.land[index] = value >= 0
	}
}

// Centre returns the position of a cell's centre.
func (landscape *Landscape) Centre(index int) (x, z float64) {
	column, row := index%landscape.Columns, index/landscape.Columns
	return landscape.MinimumX + float64(column)*landscape.Cell, landscape.MinimumZ + float64(row)*landscape.Cell
}

// CellOf returns the index of the cell a position falls in, or −1 outside the grid.
func (landscape *Landscape) CellOf(x, z float64) int {
	column := int(math.Floor((x-landscape.MinimumX)/landscape.Cell + 0.5))
	row := int(math.Floor((z-landscape.MinimumZ)/landscape.Cell + 0.5))
	if column < 0 || row < 0 || column >= landscape.Columns || row >= landscape.Rows {
		return -1
	}
	return row*landscape.Columns + column
}

// Biome returns the name of a cell's biome, or "" when it has none.
func (landscape *Landscape) Biome(index int) string {
	biome := int(landscape.biomes[index])
	if biome < 0 || biome >= len(landscape.biomeNames) {
		return ""
	}
	return landscape.biomeNames[biome]
}

// Extent is the area the grid covers, as a world field extent.
func (landscape *Landscape) Extent() world.Extent {
	half := landscape.Cell / 2
	return world.Extent{
		MinimumX: landscape.MinimumX - half, MaximumX: landscape.MinimumX - half + float64(landscape.Columns)*landscape.Cell,
		MinimumZ: landscape.MinimumZ - half, MaximumZ: landscape.MinimumZ - half + float64(landscape.Rows)*landscape.Cell,
	}
}

type sampling func(field *world.Field, column, row float64) float32

// sampled returns a field's value at the centre of every cell of the grid.
func (landscape *Landscape) sampled(field *world.Field, sample sampling) []float32 {
	values := make([]float32, landscape.Columns*landscape.Rows)
	cell := (field.Extent.MaximumX - field.Extent.MinimumX) / float64(field.Columns)
	for index := range values {
		x, z := landscape.Centre(index)
		values[index] = sample(field, (x-field.Extent.MinimumX)/cell-0.5, (z-field.Extent.MinimumZ)/cell-0.5)
	}
	return values
}

// sampleBilinear interpolates between the four nearest cell centres, clamped to the field's edges.
func sampleBilinear(field *world.Field, column, row float64) float32 {
	column = clamp(column, 0, float64(field.Columns-1))
	row = clamp(row, 0, float64(field.Rows-1))
	left, top := int(column), int(row)
	right, bottom := min(left+1, field.Columns-1), min(top+1, field.Rows-1)
	across, down := float32(column-float64(left)), float32(row-float64(top))
	upper := field.At(left, top)*(1-across) + field.At(right, top)*across
	lower := field.At(left, bottom)*(1-across) + field.At(right, bottom)*across
	return upper*(1-down) + lower*down
}

// sampleNearest takes the value of the cell the point falls in.
func sampleNearest(field *world.Field, column, row float64) float32 {
	nearestColumn := int(clamp(math.Round(column), 0, float64(field.Columns-1)))
	nearestRow := int(clamp(math.Round(row), 0, float64(field.Rows-1)))
	return field.At(nearestColumn, nearestRow)
}

func clamp(value, lowest, highest float64) float64 { return math.Max(lowest, math.Min(highest, value)) }
