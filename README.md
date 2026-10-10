# go-simulator-flora

A [go-simulator](../go-simulator) plugin that grows the vegetation of a landscape, year by year:
where every species can live, from the climate and the relief in the world, and how plants spread,
compete for light, age and come back after a disturbance. It knows nothing about any world: the
species, their traits and their habitats come from the scenario.

## The model

The landscape is a grid of cells (100 m by default, the climate's); in every cell each species
covers a share of the ground and has saplings waiting below. Every year, after the LANDIS
succession model (Mladenoff and He 1999; Scheller et al. 2007):

1. Mature plants (older than their `maturity`) spread seeds around them, as far as their
   `dispersal` reaches (a Gaussian kernel); some seeds reach every cell from far away
   (`background_seeds`).
2. Seeds become saplings where the habitat suits the species and there is light enough for its
   `shade_tolerance`: in the gaps of its layer, shaded by the plants around them when the layer
   is closed, and under the plants of its own layer, with the light that reaches the ground.
3. Saplings take the room plants leave in their layer. Trees shade shrubs and shrubs shade herbs.
4. Plants die of age (`longevity`), of a habitat that does not suit them and of shade too deep
   for them; now and then a disturbance (windthrow, fire) opens a gap (`disturbance`).

So pioneers that need full light take open ground first and cannot follow themselves under their
own shade, and shade-tolerant species waiting below take over as they age: succession. Frequent
gaps keep the pioneers in the landscape.

A species' habitat is an envelope over the facts of a cell (bioclimatic envelope models, Busby
1991), written like go-simulator's conditions, with soft edges (`softness`, in each fact's unit):

| Fact | From | Unit |
|---|---|---|
| `temperature`, `precipitation` | the climate plugin's fields | °C (yearly mean), mm a year |
| `elevation`, `slope` | the terrain plugin's fields | metres, rise over run |
| `biome` | the climate's `biome` field and its `biomes` event | a name |
| `distance_to_<anchor>` | an anchor of the rules file | metres |

## Rules

| Rule | What it does | Parameters |
|---|---|---|
| `vegetation` | Sets up the landscape at the start and advances it every step; its instance name names the landscape | see below |
| `vegetation_indicators` | Emits a `vegetation_indicators` event: hectares of every species, of forest, shrubland, grassland and bare ground, and every region's forest share and commonest species | `vegetation`, `years` |

```yaml
- rule: vegetation
  name: island
  with:
    cell: 0.1                                   # km (default: the temperature field's)
    seed: 11
    background_seeds: 0.002
    disturbance: {chance: 0.004, severity: 0.8} # a gap every 250 years in each cell, on average
    softness: {temperature: 1.5, precipitation: 150, slope: 0.1, distance: 40}
    publish_every: 50                           # years between publishing the fields
    managed:                                    # land people keep: only these forms grow there
      - {where: {biome: [fields]}, forms: [herb]}
    species:
      - id: beech
        name: Beech (Fagus sylvatica)
        form: tree                              # tree, shrub or herb
        height: 30                              # metres
        longevity: 300                          # years
        maturity: 50                            # years before it bears seeds
        shade_tolerance: 5                      # 1: needs full light … 5: grows in deep shade
        dispersal: 40                           # metres
        growth: 0.08                            # share of the free room it fills in a year
        habitat: {temperature: {min: 6, max: 11}, precipitation: {min: 1200}, slope: {max: 0.6}}
      - id: old_oak
        name: The old oak
        form: tree
        height: 20
        longevity: 2000
        maturity: 50
        shade_tolerance: 3
        dispersal: 0
        growth: 0.1
        spreads: false                          # only where an anchor plants it
        habitat: {}
```

Anchors of the rules file with role `plant` fix a plant: `properties: {species, crown}` (crown
radius in metres; smaller than a cell, it covers its share of it). Anchors with role `region`
(outlines) are measured by the indicators. The scenario can live in its own file and come in with
`with: {include: scenario.yaml}`.

## What it publishes

Following go-simulator's vocabulary: the event `vegetation` (`names`, `labels`, `forms` and
`heights` of the species) at the start, and every `publish_every` years the fields `vegetation`
(index of the species that most marks every cell: the commonest tree where tree cover reaches 0.3,
else the commonest shrub, else the commonest herb; −1 for bare ground), `tree_cover`,
`shrub_cover` and `herb_cover` (0 to 1) and, with `species_fields: true`, `cover_<id>` for every
species. Put the snapshots of the stage at multiples of `publish_every`, and choose the fields they
keep with `snapshot_fields`: a large landscape's fields take megabytes each.

The plugin needs the climate plugin before it in the rules file (and the terrain plugin when a
habitat asks for elevation or slope).

## Use

```bash
cd ../go-simulator
go run ./cmd/simulate -rules ../go-simulator-flora/examples/valley.yaml -output results/
```

## Known limits

- Plants do not yet react to other plugins' entities: a city growing over a forest does not clear
  it, and abandoned fields do not grow back.
- The viewer does not draw fields yet; the scene builder can sow the vegetation from the
  `vegetation` field as a layer, as it does with biomes.
- Seasons (flowering, leaves) belong to the climate's `seasons` rule.

## Tests

```bash
go test ./...
```

The tests grow synthetic landscapes and check what must hold anywhere: every species within its
habitat, pioneers followed by shade-tolerant trees, disturbances keeping pioneers, anchored plants
staying, managed land keeping forms out and the same seed giving the same landscape.
