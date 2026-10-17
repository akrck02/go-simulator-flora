# 📏 Rules of the flora plugin

Version 0.1.0, protocol 2. Vegetation that spreads, competes for light, ages and comes back after disturbances, year by year, where the climate and the relief suit each species (after LANDIS: Mladenoff and He 1999; Scheller et al. 2007).

This reference is generated from what the plugin itself tells the host (the `describe` message), so it always
matches the code. Parameters go under `with:` in a rules file; see go-simulator's
[rules file reference](https://github.com/akrck02/go-simulator/blob/main/docs/4-guides/1-rules-files.md).

| Rule | What it does |
|---|---|
| [`vegetation`](#vegetation) | Sets up the landscape from the world's climate and relief, and advances its vegetation every step. Its instance name names the landscape. |
| [`vegetation_indicators`](#vegetation_indicators) | Emits a "vegetation_indicators" event in the listed years (every step when none): hectares of every species, of forest, shrubland, grassland and bare ground, and every region's forest share and commonest species. |

## `vegetation`

Sets up the landscape from the world's climate and relief, and advances its vegetation every step. Its instance name names the landscape.

- **Reads:** entities with property `anchor`; fields `temperature`, `precipitation`, `elevation`, `slope`, `biome`; events `biomes`
- **Writes:** `vegetation`, `tree_cover`, `shrub_cover`, `herb_cover`

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `species` | list |  | [{id, name, form: tree\|shrub\|herb, height, longevity, maturity, shade_tolerance, dispersal, growth, habitat, spreads}] |
| `cell` | number |  | km between cell centres (default: the temperature field's) |
| `softness` | map |  | {temperature: °C, precipitation: mm, elevation: m, slope, distance: m}: how far a habitat fades past its edges |
| `disturbance` | map |  | {chance: per cell and year, severity: share of trees and shrubs lost} |
| `background_seeds` | number | `0` | seeds reaching every cell from far away |
| `publish_every` | number | `10` | years between publishing the fields |
| `species_fields` | boolean | `false` | also publish cover_<id> for every species |
| `managed` | list |  | [{where: condition over the cell's facts, forms: [herb…]}]: land people keep, where only those forms grow |
| `metres_per_unit` | number | `1000` | size of a world unit in metres (the scenes are in km) |
| `seed` | number | `1` | the same seed gives the same landscape |

## `vegetation_indicators`

Emits a "vegetation_indicators" event in the listed years (every step when none): hectares of every species, of forest, shrubland, grassland and bare ground, and every region's forest share and commonest species.

- **Reads:** nothing from other plugins
- **Writes:** nothing (it only emits events)

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `vegetation` | text |  | instance name of the vegetation rule |
| `years` | list |  |  |
