# go-simulator-flora

![A made-up island growing its vegetation from bare ground, over 200 years](docs/img/succession.jpg)


A [go-simulator](https://github.com/akrck02/go-3d-simulator) plugin that grows the vegetation of a landscape, year by year:
where every species can live, from the climate and the relief in the world, and how plants spread,
compete for light, age and come back after a disturbance, after the LANDIS succession model
(Mladenoff and He 1999; Scheller et al. 2007). It knows nothing about any world: the species,
their traits and their habitats come from the scenario.

```bash
cd ../go-simulator
go run ./cmd/simulate -rules ../go-simulator-flora/examples/valley.yaml -output results/
```

`examples/valley.yaml` grows the vegetation of a synthetic mountain from bare ground: heather
first, then oak in the lowlands and pine on the slopes.

Documentation in [docs/](docs/README.md): [the model](docs/model.md),
[writing a scenario](docs/scenario.md) and the [rules reference](docs/rules.md).
