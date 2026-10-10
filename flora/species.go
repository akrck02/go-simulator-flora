// Package flora holds the rules of the flora plugin. The landscape is a grid of cells; in every
// cell each species covers a share of the ground. Every year, after the LANDIS succession model
// (Mladenoff and He 1999; Scheller et al. 2007), mature plants spread their seeds as far as their
// dispersal reaches, seeds take where the habitat suits the species and there is room and light
// enough for its shade tolerance, plants fill the room left in their layer, and they die of age,
// of shade, of a habitat that does not suit them or of a disturbance (windthrow, fire) that opens
// a gap. Trees shade shrubs and shrubs shade herbs. The habitat of a species is an envelope over
// the facts of a cell (temperature, precipitation, elevation, slope, biome, distance to an anchor),
// with soft edges, as in bioclimatic envelope models (Busby 1991).
package flora

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Form is the layer a species grows in: trees shade shrubs, shrubs shade herbs.
type Form int

const (
	Herb Form = iota
	Shrub
	Tree
)

// layers is the number of forms.
const layers = 3

var formNames = map[string]Form{"herb": Herb, "shrub": Shrub, "tree": Tree}

func (form Form) String() string {
	for name, value := range formNames {
		if value == form {
			return name
		}
	}
	return "unknown"
}

// Species is what the scenario says about a plant. Every number is a trait of the species, in the
// units LANDIS uses: years, metres and a shade tolerance from 1 to 5.
type Species struct {
	Identifier     string         `json:"id"`
	Name           string         `json:"name"` // for people: "Laurel (Laurus novocanariensis)"
	FormName       string         `json:"form"` // tree, shrub or herb
	Height         float64        `json:"height"`
	Longevity      float64        `json:"longevity"`       // years a plant lives
	Maturity       float64        `json:"maturity"`        // years before it bears seeds
	ShadeTolerance float64        `json:"shade_tolerance"` // 1: needs full light … 5: grows in deep shade
	Dispersal      float64        `json:"dispersal"`       // metres: how far its seeds usually travel
	Growth         float64        `json:"growth"`          // share of the free room it fills in a year
	Habitat        map[string]any `json:"habitat"`         // where it can live (see habitat.go)
	Spreads        *bool          `json:"spreads"`         // false: only grows where an anchor plants it
	Form           Form           `json:"-"`
}

// SpreadsBySeed tells whether the species takes new ground from seed.
func (species *Species) SpreadsBySeed() bool { return species.Spreads == nil || *species.Spreads }

// check fills in the form and finds mistakes.
func (species *Species) check() error {
	if species.Identifier == "" {
		return fmt.Errorf("a species has no id")
	}
	if strings.ContainsAny(species.Identifier, " /") {
		return fmt.Errorf("species %q: an id has no spaces or slashes (it names fields and indicators)", species.Identifier)
	}
	form, known := formNames[species.FormName]
	if !known {
		return fmt.Errorf("species %q: form %q is unknown (tree, shrub or herb)", species.Identifier, species.FormName)
	}
	species.Form = form
	if species.Longevity <= 0 || species.Growth <= 0 || species.Growth > 1 {
		return fmt.Errorf("species %q: needs a positive longevity and a growth between 0 and 1", species.Identifier)
	}
	if species.ShadeTolerance < 1 || species.ShadeTolerance > 5 {
		return fmt.Errorf("species %q: shade_tolerance goes from 1 to 5", species.Identifier)
	}
	if species.Maturity < 0 || species.Dispersal < 0 {
		return fmt.Errorf("species %q: maturity and dispersal cannot be negative", species.Identifier)
	}
	return nil
}

// lightNeeded is the light a species needs to grow, from 0 (shade tolerance 5) to 0.8 (1).
func (species *Species) lightNeeded() float64 { return (5 - species.ShadeTolerance) / 4 * 0.8 }

// Settings are the parameters of the vegetation rule.
type Settings struct {
	Cell            float64            `json:"cell"`             // km between cell centres; 0: the temperature field's
	Seed            uint64             `json:"seed"`             // the same seed gives the same landscape
	Softness        map[string]float64 `json:"softness"`         // how far, in each fact's unit, a habitat fades past its edge
	Disturbance     Disturbance        `json:"disturbance"`      // gaps opened every year
	BackgroundSeeds float64            `json:"background_seeds"` // seeds reaching every cell from far away (birds, storms)
	PublishEvery    float64            `json:"publish_every"`    // years between publishing the fields
	SpeciesFields   bool               `json:"species_fields"`   // also publish the cover of every species
	MetresPerUnit   float64            `json:"metres_per_unit"`  // size of a world unit in metres (1000: the scenes are in km)
	Managed         []Managed          `json:"managed"`          // land people keep: only some forms grow there
	Species         []Species          `json:"species"`
}

// Managed is land people keep, such as fields and orchards: in the cells its condition matches
// (written like a habitat, without soft edges), only the species of the listed forms grow.
//
//	managed:
//	  - {where: {biome: [orchards]}, forms: [herb]}
type Managed struct {
	Where map[string]any `json:"where"`
	Forms []string       `json:"forms"`
}

// allows tells whether a form may grow on this managed land.
func (managed Managed) allows(form Form) bool {
	for _, name := range managed.Forms {
		if formNames[name] == form {
			return true
		}
	}
	return false
}

// Disturbance opens gaps: every year each cell has a chance of losing some of its trees and shrubs.
type Disturbance struct {
	Chance   float64 `json:"chance"`
	Severity float64 `json:"severity"` // share of the woody cover a gap takes
}

// settingsFrom reads the rule's parameters and fills in the defaults.
func settingsFrom(parameters map[string]any) (*Settings, error) {
	encoded, err := json.Marshal(parameters)
	if err != nil {
		return nil, err
	}
	settings := &Settings{}
	if err := json.Unmarshal(encoded, settings); err != nil {
		return nil, fmt.Errorf("vegetation: %w", err)
	}
	settings.fillDefaults()
	return settings, settings.check()
}

func (settings *Settings) fillDefaults() {
	if settings.Seed == 0 {
		settings.Seed = 1
	}
	if settings.PublishEvery <= 0 {
		settings.PublishEvery = 10
	}
	if settings.Disturbance.Severity == 0 {
		settings.Disturbance.Severity = 0.8
	}
}

func (settings *Settings) check() error {
	if len(settings.Species) == 0 {
		return fmt.Errorf("vegetation needs species")
	}
	seen := map[string]bool{}
	for index, managed := range settings.Managed {
		if len(managed.Where) == 0 {
			return fmt.Errorf("managed land %d needs a \"where\"", index+1)
		}
		for _, name := range managed.Forms {
			if _, known := formNames[name]; !known {
				return fmt.Errorf("managed land %d: form %q is unknown (tree, shrub or herb)", index+1, name)
			}
		}
	}
	for index := range settings.Species {
		species := &settings.Species[index]
		if err := species.check(); err != nil {
			return err
		}
		if seen[species.Identifier] {
			return fmt.Errorf("species %q appears twice", species.Identifier)
		}
		seen[species.Identifier] = true
	}
	return nil
}
