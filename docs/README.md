# 📚 go-simulator-flora documentation

| | Page | What is in it |
|---|---|---|
| 🏴‍☠️ | [The flora quest](quest.md) | Grow an island's plants over 200 years in 5 levels: grass, heather, pioneers, succession, storms, and an old oak. |
| 🚀 | [Getting started](getting-started.md) | The folders it needs, running the example, what it writes, the tests. |
| 🌳 | [The model](model.md) | Cells, habitats, layers and light, one year, how a forest spreads, succession, disturbance, anchors, managed land, outputs, determinism and speed, known limits, with pictures of each. |
| 📝 | [Writing a scenario](scenario.md) | How to write the species, with an annotated example and advice on traits. |
| 🔍 | [How it works](how-it-works.md) | How the plugin fits go-simulator, what it reads and writes, and how the pictures were made. |
| 📏 | [Rules reference](rules.md) | The `vegetation` and `vegetation_indicators` rules and every parameter, generated from the plugin's own description. |

New here? Start with the [project README](../README.md).

## 🚧 Not done yet

- Plants do not react yet to other plugins' entities: a growing city does not clear the forest,
  and abandoned fields do not grow back.
- Pictures of the 3D viewer itself: the viewer does not draw fields yet, so the pictures in these
  pages are 2D drawings of the fields the plugin published, not screenshots (see
  [how the pictures were made](how-it-works.md#how-the-pictures-were-made)).
- Seasons (flowering, leaves) belong to the climate's `seasons` rule, not to this plugin.
