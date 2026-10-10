package flora

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/akrck02/go-simulator/conditions"
	"github.com/akrck02/go-simulator/plugin"
	"github.com/akrck02/go-simulator/protocol"
	"github.com/akrck02/go-simulator/world"
)

// Version of the plugin.
const Version = "0.1.0"

// Anchor roles the plugin reads.
const (
	rolePlant  = "plant"  // a fixed plant: properties "species" and "crown" (metres of radius)
	roleRegion = "region" // an outline the indicators measure
)

// landscapes are the vegetations the plugin grows, by the instance name of their vegetation rule.
type landscapes map[string]*Vegetation

// Plugin returns the flora plugin.
func Plugin() *plugin.Plugin {
	registry := landscapes{}
	return &plugin.Plugin{
		Name:    "flora",
		Version: Version,
		Summary: "Vegetation that spreads, competes for light, ages and comes back after disturbances, year by year, " +
			"where the climate and the relief suit each species (after LANDIS: Mladenoff and He 1999; Scheller et al. 2007).",
		Rules: []plugin.Definition{vegetationRule(registry), indicatorsRule(registry)},
	}
}

func vegetationRule(registry landscapes) plugin.Definition {
	return plugin.Definition{
		Description: protocol.RuleDescription{
			Name: "vegetation",
			Summary: "Sets up the landscape from the world's climate and relief, and advances its vegetation every step. " +
				"Its instance name names the landscape.",
			Parameters: []protocol.ParameterDescription{
				{Name: "species", Type: "list", Summary: "[{id, name, form: tree|shrub|herb, height, longevity, maturity, shade_tolerance, dispersal, growth, habitat, spreads}]"},
				{Name: "cell", Type: "number", Summary: "km between cell centres (default: the temperature field's)"},
				{Name: "softness", Type: "map", Summary: "{temperature: °C, precipitation: mm, elevation: m, slope, distance: m}: how far a habitat fades past its edges"},
				{Name: "disturbance", Type: "map", Summary: "{chance: per cell and year, severity: share of trees and shrubs lost}"},
				{Name: "background_seeds", Type: "number", Default: 0, Summary: "seeds reaching every cell from far away"},
				{Name: "publish_every", Type: "number", Default: 10, Summary: "years between publishing the fields"},
				{Name: "species_fields", Type: "boolean", Default: false, Summary: "also publish cover_<id> for every species"},
				{Name: "seed", Type: "number", Default: 1},
			},
			Reads: world.Interest{
				Fields:     []string{world.FieldTemperature, world.FieldPrecipitation, world.FieldElevation, world.FieldSlope, world.FieldBiome},
				Events:     []string{world.EventBiomes},
				Properties: []string{world.PropertyAnchor},
			},
			Writes: []string{world.FieldVegetation, world.FieldTreeCover, world.FieldShrubCover, world.FieldHerbCover},
		},
		New: func(parameters plugin.Parameters) (plugin.Rule, error) {
			settings, err := settingsFrom(parameters)
			if err != nil {
				return nil, err
			}
			return &vegetationRuleInstance{registry: registry, settings: settings}, nil
		},
	}
}

type vegetationRuleInstance struct {
	registry      landscapes
	settings      *Settings
	vegetation    *Vegetation
	lastPublished float64
}

// Start lays out the landscape, plants the anchored plants and publishes the bare landscape.
func (rule *vegetationRuleInstance) Start(context *plugin.Context) error {
	vegetation, err := setUp(rule.settings, context)
	if err != nil {
		return err
	}
	rule.vegetation, rule.lastPublished = vegetation, context.Time
	rule.registry[context.Instance] = vegetation
	vegetation.publishNames(context)
	vegetation.publishFields(context)
	return nil
}

// Step simulates the years the step covers and publishes the fields when it is time.
func (rule *vegetationRuleInstance) Step(context *plugin.Context) error {
	if rule.vegetation == nil {
		return fmt.Errorf("the vegetation was not set up: it needs to start with the simulation")
	}
	rule.vegetation.Advance(max(1, int(math.Round(context.Step))))
	if context.Time-rule.lastPublished >= rule.settings.PublishEvery-1e-9 {
		rule.vegetation.publishFields(context)
		rule.lastPublished = context.Time
	}
	return nil
}

func setUp(settings *Settings, context *plugin.Context) (*Vegetation, error) {
	landscape, err := newLandscape(context.World, settings.Cell, biomeNames(context.Events))
	if err != nil {
		return nil, err
	}
	anchors := anchorsOf(context.World)
	metresPerUnit := settings.MetresPerUnit
	if metresPerUnit <= 0 {
		metresPerUnit = defaultMetresPerKm
	}
	cellFacts, err := newFacts(landscape, settings.Species, shapesOf(anchors), metresPerUnit)
	if err != nil {
		return nil, err
	}
	if err := cellFacts.prepareManaged(settings.Managed, shapesOf(anchors), metresPerUnit); err != nil {
		return nil, err
	}
	vegetation := newVegetation(settings, landscape, cellFacts)
	if err := vegetation.placeAnchors(anchors, metresPerUnit); err != nil {
		return nil, err
	}
	vegetation.plant()
	return vegetation, nil
}

// biomeNames are the names the climate gave its biomes, from its last biomes event.
func biomeNames(events []world.Event) []string {
	var names []string
	for _, event := range events {
		if event.Name != world.EventBiomes {
			continue
		}
		names = names[:0]
		list, _ := event.Data["names"].([]any)
		for _, name := range list {
			text, _ := name.(string)
			names = append(names, text)
		}
	}
	return names
}

// anchorsOf returns the anchors of the world, by id.
func anchorsOf(current *world.World) map[string]*world.Entity {
	anchors := map[string]*world.Entity{}
	for _, entity := range current.Entities() {
		if entity.Properties[world.PropertyAnchor] == true {
			anchors[string(entity.Identifier)] = entity
		}
	}
	return anchors
}

func shapesOf(anchors map[string]*world.Entity) map[string]anchorShape {
	shapes := map[string]anchorShape{}
	for identifier, entity := range anchors {
		shapes[identifier] = shapeOf(entity)
	}
	return shapes
}

func shapeOf(entity *world.Entity) anchorShape {
	if entity.Shape != nil && len(entity.Shape.Outline) > 0 {
		return anchorShape{outline: entity.Shape.Outline}
	}
	if entity.Shape != nil && len(entity.Shape.Line) > 0 {
		return anchorShape{line: entity.Shape.Line}
	}
	point := [2]float64{}
	if entity.Transform != nil {
		position := entity.Transform.Position()
		point = [2]float64{float64(position[0]), float64(position[2])}
	}
	return anchorShape{point: &point}
}

// placeAnchors turns plant anchors into plantings and region anchors into regions.
func (vegetation *Vegetation) placeAnchors(anchors map[string]*world.Entity, metresPerUnit float64) error {
	positions := vegetation.positionsByCell()
	identifiers := slices.Sorted(maps.Keys(anchors))
	for _, identifier := range identifiers {
		entity := anchors[identifier]
		switch entity.Properties[world.PropertyRole] {
		case rolePlant:
			fixed, err := vegetation.plantingOf(identifier, entity, positions, metresPerUnit)
			if err != nil {
				return err
			}
			vegetation.plantings = append(vegetation.plantings, fixed)
		case roleRegion:
			vegetation.regions = append(vegetation.regions, region{identifier: identifier, cells: vegetation.cellsInside(shapeOf(entity), 0, positions)})
		}
	}
	return nil
}

func (vegetation *Vegetation) plantingOf(identifier string, entity *world.Entity, positions map[int]int, metresPerUnit float64) (planting, error) {
	speciesIdentifier, _ := entity.Properties["species"].(string)
	speciesIndex := vegetation.speciesIndex(speciesIdentifier)
	if speciesIndex < 0 {
		return planting{}, fmt.Errorf("anchor %q plants species %q, which the vegetation does not have", identifier, speciesIdentifier)
	}
	crown, _ := conditions.Number(entity.Properties["crown"])
	cells := vegetation.cellsInside(shapeOf(entity), crown/metresPerUnit, positions)
	if len(cells) == 0 {
		return planting{}, fmt.Errorf("anchor %q is not on land", identifier)
	}
	share := float32(1)
	if len(cells) == 1 {
		cellMetres := vegetation.landscape.Cell * metresPerUnit
		share = float32(math.Min(1, math.Pi*crown*crown/(cellMetres*cellMetres)))
	}
	return planting{species: speciesIndex, cells: cells, share: share}, nil
}

// cellsInside returns the land cells (as positions) within some distance of a shape, or, when no
// cell centre is, the cell under it.
func (vegetation *Vegetation) cellsInside(shape anchorShape, reach float64, positions map[int]int) []int {
	var cells []int
	for position, cell := range vegetation.landCells {
		x, z := vegetation.landscape.Centre(cell)
		if shape.distance(x, z) <= reach {
			cells = append(cells, position)
		}
	}
	if len(cells) > 0 || shape.point == nil {
		return cells
	}
	if position, onLand := positions[vegetation.landscape.CellOf(shape.point[0], shape.point[1])]; onLand {
		cells = append(cells, position)
	}
	return cells
}

// positionsByCell maps the grid index of every land cell to its position among the land cells.
func (vegetation *Vegetation) positionsByCell() map[int]int {
	positions := make(map[int]int, len(vegetation.landCells))
	for position, cell := range vegetation.landCells {
		positions[cell] = position
	}
	return positions
}

func (vegetation *Vegetation) speciesIndex(identifier string) int {
	for index, plant := range vegetation.species {
		if plant.Identifier == identifier {
			return index
		}
	}
	return -1
}

// ---------------------------------------------------------------- indicators

type indicatorsRuleInstance struct {
	registry  landscapes
	landscape string
	years     []float64
}

func indicatorsRule(registry landscapes) plugin.Definition {
	return plugin.Definition{
		Description: protocol.RuleDescription{
			Name: "vegetation_indicators",
			Summary: "Emits a \"vegetation_indicators\" event in the listed years (every step when none): hectares of every species, " +
				"of forest, shrubland, grassland and bare ground, and every region's forest share and commonest species.",
			Parameters: []protocol.ParameterDescription{
				{Name: "vegetation", Type: "text", Summary: "instance name of the vegetation rule"},
				{Name: "years", Type: "list"},
			},
		},
		New: func(parameters plugin.Parameters) (plugin.Rule, error) {
			rule := &indicatorsRuleInstance{registry: registry, landscape: parameters.Text("vegetation", "")}
			list, _ := parameters["years"].([]any)
			for _, year := range list {
				if number, isNumber := year.(float64); isNumber {
					rule.years = append(rule.years, number)
				}
			}
			return rule, nil
		},
	}
}

func (rule *indicatorsRuleInstance) Step(context *plugin.Context) error {
	vegetation, found := rule.registry[rule.landscape]
	if !found {
		return fmt.Errorf("no vegetation %q: a \"vegetation\" rule named so must come first", rule.landscape)
	}
	if !rule.isDue(context.Time) {
		return nil
	}
	measures := vegetation.Indicators()
	measures["year"] = context.Time
	measures["vegetation"] = rule.landscape
	context.Emit("vegetation_indicators", measures)
	return nil
}

func (rule *indicatorsRuleInstance) isDue(time float64) bool {
	if len(rule.years) == 0 {
		return true
	}
	for _, year := range rule.years {
		if math.Abs(year-time) < 1e-9 {
			return true
		}
	}
	return false
}
