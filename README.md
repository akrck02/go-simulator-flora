# go-simulator-flora

> **Note:** this project was written with an AI assistant and is pending thorough review. See the [AI disclosure](#ai-disclosure).

![A made-up island growing its vegetation from bare ground, over 200 years](docs/img/succession.jpg)

A [go-simulator](https://github.com/akrck02/go-simulator) plugin that grows the vegetation of a landscape, year by year:
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

## AI disclosure

I write most of my code by hand, so I want to be clear about this project: **it was written by an
AI assistant (Claude, by Anthropic) under my direction.** I decided what to build and how it should
behave; the code, the tests and the documentation were written by the AI. Every commit made this
way carries a `Co-Authored-By: Claude` line (all of them).

**Status: pending thorough review.** The code compiles and its tests pass, but I have not yet
reviewed it line by line. Treat it as a working draft, not as code I vouch for.
