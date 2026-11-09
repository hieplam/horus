---
family: flux3-klein
workflow: workflows/it-illustration
seed: 77
size: 1536x1024
steps: 15
---

# A laptop, a load balancer and three servers

## Caption (flat-vector style sentences + subject; see [flat-vector-tech-style](flat-vector-tech-style.md))

```text
A clean flat vector illustration for a modern tech explainer. It looks like a tidy technical diagram, with dark navy outlines of uniform medium weight and rounded corners, and solid flat pastel fills. On the left, a sky-blue laptop shows only abstract coloured bars. A thin navy arrow runs from the laptop to a lilac rounded box in the centre, and three short navy arrows fan out of that box to the right. On the right, three mint-green server boxes stand in a column, and each of the three arrows points at one server box. The colour palette is dark navy for outlines, white, sky blue, lilac, mint green, sun yellow and coral, each with a light pastel tint for fills. Each object has one very soft flat shadow below it. The background is plain very pale blue with a faint sparse dotted grid and nothing else. Shapes are simple and friendly, with generous empty space. The whole scene is centred and fully inside the frame, with wide margins on all sides. Every surface is blank: screens and panels show only abstract coloured bars and blocks. The picture contains no text of any kind: no letters, no digits, no symbols and no logos.
```

## Result

- File: `workflows/it-illustration/examples/load-balancer.png`
- Run: comfy MCP: `set_workflow_slot` on a copy of `workflow.json` (caption, seed 77, `filename_prefix`), `validate_workflow`, `run_workflow(wait=False)`, `job(action="wait")`, `fetch_outputs`. Prompt id `2e80323c-53f3-4c73-bf99-8287d0a7432b`.
- Time: 284.5 s on a GTX 1660 Ti (6 GB).
- Check: flat vector, on subject, no visible text.

## Notes

The subject does not name the middle box. A name in the subject can come back as text in the picture.
