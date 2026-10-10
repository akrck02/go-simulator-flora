package flora

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/akrck02/go-simulator/configuration"
	"github.com/akrck02/go-simulator/host"
	"github.com/akrck02/go-simulator/plugin"
	"github.com/akrck02/go-simulator/protocol"
	"github.com/akrck02/go-simulator/world"
)

// slope is a synthetic landscape 6 km wide and 1 km deep in cells of 100 m: the temperature
// falls from 16 °C in the west (x = 0) to 0 °C in the east, it rains 1200 mm everywhere and the
// last column is sea.
const slopeColumns, slopeRows = 60, 10

func slopeWorld(t *testing.T) *world.World {
	t.Helper()
	temperature, precipitation, biome := make([]float32, slopeColumns*slopeRows), make([]float32, slopeColumns*slopeRows), make([]float32, slopeColumns*slopeRows)
	for index := range temperature {
		column := index % slopeColumns
		temperature[index] = 16 - 16*float32(column)/float32(slopeColumns-1)
		precipitation[index] = 1200
		if column == slopeColumns-1 {
			biome[index] = -1
		}
	}
	extent := &world.Extent{MinimumX: 0, MaximumX: 6, MinimumZ: 0, MaximumZ: 1}
	current := world.New()
	for name, values := range map[string][]float32{world.FieldTemperature: temperature, world.FieldPrecipitation: precipitation, world.FieldBiome: biome} {
		patch := &world.FieldPatch{Name: name, Columns: slopeColumns, Rows: slopeRows, Extent: extent, Values: values}
		if err := current.ApplyChange(world.SetFieldValues(patch)); err != nil {
			t.Fatal(err)
		}
	}
	return current
}

// announcer stands for the climate: it names the biome of the synthetic world.
func announcer() *plugin.Plugin {
	return &plugin.Plugin{Name: "climate", Version: "1", Rules: []plugin.Definition{{
		Description: protocol.RuleDescription{Name: "name_biomes"},
		New:         func(plugin.Parameters) (plugin.Rule, error) { return &biomeNamer{}, nil },
	}}}
}

type biomeNamer struct{}

func (rule *biomeNamer) Start(context *plugin.Context) error {
	context.Emit(world.EventBiomes, map[string]any{"names": []any{"land"}})
	return nil
}

func (rule *biomeNamer) Step(*plugin.Context) error { return nil }

func runLandscape(t *testing.T, years int, scenario string) (*host.Engine, *Vegetation) {
	t.Helper()
	rules := fmt.Sprintf(`
plugins: [{name: climate, builtin: true}, {name: flora, builtin: true}]
%s
stages:
  - name: growth
    clock: {unit: year, from: 0, to: %d, step: 1}
    rules:
      - rule: name_biomes
      - rule: vegetation
        name: slope
        with:
          background_seeds: 0.002
          seed: 4
%s
`, anchorsPart(scenario), years, speciesPart(scenario))
	settings, err := configuration.Parse([]byte(rules))
	if err != nil {
		t.Fatal(err)
	}
	registry := landscapes{}
	flora := Plugin()
	flora.Rules = []plugin.Definition{vegetationRule(registry), indicatorsRule(registry)}
	engine, err := host.Build(settings, slopeWorld(t), map[string]host.Builtin{"climate": announcer().ServeConnection, "flora": flora.ServeConnection})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := engine.Start(); err != nil {
		t.Fatal(err)
	}
	for !engine.Stage("growth").Finished() {
		if _, err := engine.StepStage("growth"); err != nil {
			t.Fatal(err)
		}
	}
	return engine, registry["slope"]
}

// A scenario is the species (indented under "with") and, after a "---", the anchors.
func speciesPart(scenario string) string {
	species, _, _ := strings.Cut(scenario, "---")
	return species
}

func anchorsPart(scenario string) string {
	_, anchors, _ := strings.Cut(scenario, "---")
	return anchors
}

const bandsScenario = `          species:
            - {id: pine, name: Pine, form: tree, height: 25, longevity: 200, maturity: 10, shade_tolerance: 2, dispersal: 150, growth: 0.2, habitat: {temperature: {max: 7}}}
            - {id: oak, name: Oak, form: tree, height: 25, longevity: 300, maturity: 15, shade_tolerance: 3, dispersal: 80, growth: 0.15, habitat: {temperature: {min: 8}}}
            - {id: grass, name: Grass, form: herb, height: 0.5, longevity: 20, maturity: 1, shade_tolerance: 1, dispersal: 300, growth: 0.6, habitat: {}}`

// cover returns the mean cover of a species over the land cells of some columns.
func cover(vegetation *Vegetation, identifier string, fromColumn, toColumn int) float64 {
	speciesIndex := vegetation.speciesIndex(identifier)
	total, count := 0.0, 0
	for position, cell := range vegetation.landCells {
		if column := cell % slopeColumns; column >= fromColumn && column < toColumn {
			total += float64(vegetation.cover[speciesIndex][position])
			count++
		}
	}
	return total / float64(count)
}

func TestEverySpeciesGrowsWhereItsHabitatIs(t *testing.T) {
	_, vegetation := runLandscape(t, 200, bandsScenario)
	if pine := cover(vegetation, "pine", 40, 59); pine < 0.45 {
		t.Errorf("pine covers %.2f of the cold east, want an open pine forest (it needs light to regenerate)", pine)
	}
	if oak := cover(vegetation, "oak", 0, 20); oak < 0.6 {
		t.Errorf("oak covers %.2f of the warm west, want most of it", oak)
	}
	if pine := cover(vegetation, "pine", 0, 20); pine > 0.01 {
		t.Errorf("pine covers %.2f of the warm west, outside its habitat", pine)
	}
	if grass := cover(vegetation, "grass", 0, 59); grass > 0.1 {
		t.Errorf("grass covers %.2f under closed forests, want little: it needs full light", grass)
	}
	if len(vegetation.landCells) != slopeRows*(slopeColumns-1) {
		t.Errorf("%d land cells, want every cell but the sea column", len(vegetation.landCells))
	}
}

const successionScenario = `          disturbance: {chance: 0}
          species:
            - {id: birch, name: Birch, form: tree, height: 20, longevity: 60, maturity: 5, shade_tolerance: 1, dispersal: 300, growth: 0.4, habitat: {}}
            - {id: beech, name: Beech, form: tree, height: 30, longevity: 300, maturity: 30, shade_tolerance: 5, dispersal: 60, growth: 0.1, habitat: {}}`

func TestShadeTolerantTreesFollowPioneers(t *testing.T) {
	_, early := runLandscape(t, 40, successionScenario)
	birch, beech := cover(early, "birch", 0, 59), cover(early, "beech", 0, 59)
	if birch <= beech {
		t.Errorf("after 40 years birch covers %.2f and beech %.2f: the pioneer should take open ground first", birch, beech)
	}
	_, late := runLandscape(t, 300, successionScenario)
	birch, beech = cover(late, "birch", 0, 59), cover(late, "beech", 0, 59)
	if beech <= birch {
		t.Errorf("after 300 years birch covers %.2f and beech %.2f: the shade-tolerant tree should follow", birch, beech)
	}
}

const anchoredScenario = bandsScenario + `
            - {id: great_willow, name: Great willow, form: tree, height: 60, longevity: 2000, maturity: 50, shade_tolerance: 5, dispersal: 0, growth: 0.1, spreads: false, habitat: {}}
            - {id: lily, name: Lily, form: herb, height: 0.4, longevity: 10, maturity: 1, shade_tolerance: 5, dispersal: 30, growth: 0.5, habitat: {distance_to_willow: {max: 150}}}
---
anchors:
  - {id: willow, role: plant, at: [1.05, 0.55], properties: {species: great_willow, crown: 120}}
  - {id: warm_end, role: region, outline: [[0, 0], [1, 0], [1, 1], [0, 1]]}`

func TestAnchoredPlantsStayAndHabitatsCanBeNearThem(t *testing.T) {
	engine, vegetation := runLandscape(t, 200, anchoredScenario)
	willow := vegetation.speciesIndex("great_willow")
	position := vegetation.positionsByCell()[vegetation.landscape.CellOf(1.05, 0.55)]
	if vegetation.cover[willow][position] != 1 {
		t.Errorf("the anchored willow covers %.2f of its cell, want all of it", vegetation.cover[willow][position])
	}
	if far := cover(vegetation, "great_willow", 30, 59); far != 0 {
		t.Errorf("the willow spread to %.2f of the east, but it only grows where an anchor plants it", far)
	}
	if lilies := cover(vegetation, "lily", 30, 59); lilies != 0 {
		t.Errorf("lilies cover %.2f far from the willow, outside their habitat", lilies)
	}
	if lilies := vegetation.Indicators()["hectares_lily"].(float64); lilies <= 0 {
		t.Error("lilies should grow around the willow")
	}
	measures := vegetation.Indicators()
	if share, _ := measures["forest_share_warm_end"].(float64); share < 0.5 {
		t.Errorf("the warm end is %.2f forest, want most of it (indicators %v)", share, measures)
	}
	if engine.World.Field(world.FieldVegetation) == nil || engine.World.Field(world.FieldTreeCover) == nil {
		t.Error("the vegetation fields should be in the world")
	}
}

func TestTheSameSeedGivesTheSameLandscape(t *testing.T) {
	_, first := runLandscape(t, 30, bandsScenario)
	_, second := runLandscape(t, 30, bandsScenario)
	for speciesIndex := range first.cover {
		for position := range first.cover[speciesIndex] {
			if first.cover[speciesIndex][position] != second.cover[speciesIndex][position] {
				t.Fatalf("species %d differs at land cell %d", speciesIndex, position)
			}
		}
	}
}

func TestHabitatEdgesAreSoft(t *testing.T) {
	bounds := map[string]any{"min": 8, "max": 12}
	cases := []struct{ value, softness, want float64 }{{10, 2, 1}, {13, 2, 0.5}, {14, 2, 0}, {7, 0, 0}, {6.5, 2, 0.25}}
	for _, testCase := range cases {
		if got := rangeSuitability(bounds, testCase.value, testCase.softness); math.Abs(got-testCase.want) > 1e-9 {
			t.Errorf("suitability of %g with softness %g = %g, want %g", testCase.value, testCase.softness, got, testCase.want)
		}
	}
}

func TestSpeciesNeedAFormAndTraits(t *testing.T) {
	for _, species := range []Species{
		{Identifier: "tree", FormName: "bush", Longevity: 10, Growth: 0.1, ShadeTolerance: 3},
		{Identifier: "tree", FormName: "tree", Longevity: 0, Growth: 0.1, ShadeTolerance: 3},
		{Identifier: "tree", FormName: "tree", Longevity: 10, Growth: 0.1, ShadeTolerance: 7},
		{Identifier: "a tree", FormName: "tree", Longevity: 10, Growth: 0.1, ShadeTolerance: 3},
	} {
		if species.check() == nil {
			t.Errorf("%+v should be refused", species)
		}
	}
}

func TestDisturbancesKeepPioneersInTheLandscape(t *testing.T) {
	disturbed := strings.Replace(successionScenario, "disturbance: {chance: 0}", "disturbance: {chance: 0.005}", 1)
	_, calm := runLandscape(t, 300, successionScenario)
	_, stormy := runLandscape(t, 300, disturbed)
	calmBirch, stormyBirch := cover(calm, "birch", 0, 59), cover(stormy, "birch", 0, 59)
	if stormyBirch <= 2*calmBirch || stormyBirch < 0.05 {
		t.Errorf("birch covers %.2f with gaps and %.2f without: gaps should keep the pioneer", stormyBirch, calmBirch)
	}
	if beech := cover(stormy, "beech", 0, 59); beech < 0.2 {
		t.Errorf("beech covers %.2f with gaps: it should still hold most of the forest", beech)
	}
}

func TestManagedLandKeepsOutSomeForms(t *testing.T) {
	scenario := strings.Replace(bandsScenario, "          species:", "          managed: [{where: {temperature: {min: 14}}, forms: [herb]}]\n          species:", 1)
	_, vegetation := runLandscape(t, 200, scenario)
	if oak := cover(vegetation, "oak", 0, 4); oak != 0 {
		t.Errorf("oak covers %.2f of the managed warm end, where only herbs may grow", oak)
	}
	if grass := cover(vegetation, "grass", 0, 4); grass < 0.5 {
		t.Errorf("grass covers %.2f of the managed warm end, want the herbs to keep it", grass)
	}
	if oak := cover(vegetation, "oak", 6, 20); oak < 0.5 {
		t.Errorf("oak covers %.2f just past the managed land, want forest there", oak)
	}
}

func TestASmallCrownCoversItsShareOfTheCell(t *testing.T) {
	scenario := strings.Replace(anchoredScenario, "crown: 120", "crown: 30", 1)
	_, vegetation := runLandscape(t, 20, scenario)
	willow := vegetation.speciesIndex("great_willow")
	position := vegetation.positionsByCell()[vegetation.landscape.CellOf(1.05, 0.55)]
	want := float32(math.Pi * 30 * 30 / (100 * 100))
	if got := vegetation.cover[willow][position]; math.Abs(float64(got-want)) > 1e-6 {
		t.Errorf("a crown of 30 m covers %.3f of its 100 m cell, want %.3f", got, want)
	}
}
