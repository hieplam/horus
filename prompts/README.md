# Saved prompts

A prompt that works is worth saving. Save it with its result, so that the next
person can trust it or fix it.

## Rule: one file per prompt, filed by model family

```
prompts/<family>/<name>.md
```

`<family>` is a short kebab-case name for the model family, for example
`flux3-klein`. `<name>` describes the subject, for example
`watercolor-lake-postcard`.

Prompts are not portable between model families. The families expect different
prompt styles:

| Family style | Prompt form |
|---|---|
| FLUX and other models with a large language-model text encoder | Full sentences that describe the scene, the style, and the colors |
| SDXL-based anime-style families | Comma-separated tag lists, with quality tags |

A prompt written for one family gives poor results in another. Save it under
the family that you tested it with.

## File format

Start with frontmatter. Then write the prompt text and the result.

```markdown
---
family: flux3-klein
workflow: postcard-watercolor
seed: 1234
size: 1024x1536
steps: 15
---

# Watercolor lake postcard

## Prompt

A soft watercolor landscape postcard of a mountain lake at sunrise, pale
blue sky on the left, a small wooden boat in the lower third, loose brush
strokes, paper texture visible.

## Result

- Status: verified
- Output: `lake-1234.png`
- Run time: 142 s
- Notes: the boat appears in every seed that I tried. Two seeds add a second boat.
```

| Frontmatter key | Meaning |
|---|---|
| `family` | Model family folder name. Must match the folder that holds the file. |
| `workflow` | Name of the folder under `workflows/` that ran the prompt. |
| `seed` | Seed value. The same seed with the same settings gives the same image. |
| `size` | Width and height in pixels, as `WIDTHxHEIGHT`. |
| `steps` | Number of sampler steps. |

## Apply a saved prompt

1. Copy the workflow to a scratch path. Never change the committed file.
2. Run `list_workflow_slots` on the copy.
3. Run `set_workflow_slot` for the prompt, seed, size, and steps. Use `stdout=False` to write the copy.
4. Run `validate_workflow`, then follow [running workflows](../docs/running-workflows.md).

## After a run

Update the Result section. Write the status, the output name, and the run
time. A prompt with no recorded result cannot be trusted later.
