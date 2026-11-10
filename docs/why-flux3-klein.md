# Why FLUX 3 Image Klein is the first horus model

horus uses FLUX 3 Image Klein (int8 build) because it is the best open model that we could run on a 6 GB graphics card. It makes flat-vector illustrations of technical topics with a style that stays the same from one picture to the next. Larger candidates did not fit in 6 GB of VRAM and 16 GB of RAM, and a small older model failed the style check.

This page explains the choice. It also records the hardware traps, the tooling problems, the prompt findings and the limits of our measurements. The workflow itself is in [`workflows/it-illustration`](../workflows/it-illustration/README.md). The files and hashes are in [`models.md`](models.md).

## Terms

- **VRAM:** the memory on the graphics card. A model that is larger than VRAM must stream from system RAM, and this is slow.
- **Distilled model:** a model that was trained to need few sampling steps (here 4 to 16) and CFG 1.
- **CFG (classifier-free guidance):** a setting that pushes the picture toward the prompt. A distilled model needs CFG 1.
- **int8:** a model weight format that uses 8 bits for each number. It makes the file about half the size of bf16.
- **bf16 and fp32:** two number formats for computing. bf16 uses 16 bits and is fast on new cards. fp32 uses 32 bits and is slower.
- **Attention kernel:** the GPU program that computes the attention step of the model. The int8 build ships its own kernel.
- **CUDA driver:** the NVIDIA software that lets programs use the GPU. Each driver supports a maximum CUDA version.
- **Prompt upsampler:** a language model that turns a short request into a long, detailed caption. The FLUX 3 authors ship one. horus does not install it. The agent that drives horus writes the caption instead.
- **Style score:** a number from 0 to 100 that a script computes from the pixels of a picture. It checks style only (see [section 5](#5-measurement-and-its-limits)).
- **Tiny GPU class:** in this page, a GTX 1660 Ti class card with 6 GB of VRAM and 16 GB of system RAM.

## 1. Which model, and why

We wanted flat-vector pictures of technical topics, on a plain background, in a fixed palette, with no readable text. We ranked open-weights image models on the public Artificial Analysis open-weights leaderboard. Then we checked which of the top models can run on the tiny GPU class.

| Model | Family | Size | Fits 6 GB VRAM and 16 GB RAM? | Result | Why chosen or rejected |
|---|---|---|---|---|---|
| Qwen-Image-2.1 | Qwen-Image | 7B image model, plus an 8B text encoder (9.35 GB in int8) | No | Rejected | The text encoder alone is too large for 16 GB of RAM next to the image model. The license was reported as non-commercial (not verified). |
| **FLUX 3 Image Klein** | FLUX 3 | 5.6B, int8 set of about 11 GB | Yes, verified | **Chosen** | Ranked second on the leaderboard. It runs with ComfyUI core nodes only. The publisher ships an int8 build for ComfyUI. |
| Fibo 2.0 | Fibo | not checked | not checked | Not tested | It ranked third. We did not check its size or license. |
| Ming-Image-0.1-Design | Ming | 6B, made for layouts with much text | not checked | Not tested | Our style forbids text in the picture, so its main strength does not help. |
| Z-Image Turbo | Z-Image | 6B, GGUF build needs a custom node | Probably, with heavy quantization | Not tested | It ranked far below the chosen model. It needs an extra custom node. |
| A 12.9B image model from another vendor | not named here | 12.9B | No | Rejected | The weights are too large for the tiny GPU class, even with streaming. |
| An SD 1.5 checkpoint with a style LoRA and an IP-Adapter | Stable Diffusion 1.5 | 2.1 GB | Yes | Failed the style check (score 13.4) | It fits easily, but the pictures did not match the flat-vector style. The IP-Adapter also leaked the content of the reference picture into the result. |

FLUX 3 Image Klein also won on four practical points:

1. ComfyUI supports it with core nodes only. The workflow needs no custom node pack.
2. The publisher ships a ready int8 build for ComfyUI. It is the three-file set that [`models.md`](models.md#flux-3-image-klein-int8-set) lists.
3. The license allows commercial use of the pictures for small owners (below a yearly revenue limit, see the license text on the repository page).
4. The model accepts reference pictures. We did not test this.

## 2. Hardware traps on a 6 GB card

These findings come from a GTX 1660 Ti class card (Turing architecture) with 6 GB of VRAM and 16 GB of system RAM. The driver supported CUDA 12.x only.

1. **The card computes in fp32.** FLUX 3 supports only bf16 and fp32. Turing cards have no native bf16. As a result, ComfyUI falls back to fp32, which is slow.
2. **The int8 attention kernel fails on an older driver.** The int8 checkpoint asks for the kernel `comfy_kitchen_int8`. The kernel is built for the CUDA 13 runtime. A driver that supports only CUDA 12.x cannot start it. Then the first sampler step fails with `CUDA driver version is insufficient for CUDA runtime version`. The availability check of the kernel tests the GPU model, not the driver, so it does not warn you.
3. **The fix is a core node.** The node `ModelAttentionBackend`, set to `pytorch attention`, makes ComfyUI use the standard PyTorch attention. ComfyUI reads this setting before the method that the checkpoint stores. The int8 linear layers still run. The workflow README explains the node in [Why the attention node exists](../workflows/it-illustration/README.md#why-the-attention-node-exists).
4. **Speed is low.** One sampler step takes about 14 s at 1536x1024. A 15-step picture takes about 220 to 285 s on this class. An 8-step picture takes about 130 s.
5. **Memory is tight.** ComfyUI streams the model through the 6 GB of VRAM. When other large programs run, free system RAM can drop below 1 GB. No run failed, but model loading then uses the page file. Close other large programs before a run.
6. **A newer driver is not tested.** A driver with CUDA 13 support can let the int8 kernel load. This card class has no tensor cores, so the kernel can still fail.

## 3. Tooling findings

| Finding | Effect | What we did |
|---|---|---|
| comfy-cli prints Unicode characters to a console with a legacy code page | `UnicodeEncodeError: 'charmap' codec` | Set `PYTHONUTF8=1` for the comfy MCP server. |
| comfy-cli gives each node a long random ID | Slot addresses look like `2752976026169904.text` | The README lists the addresses. The runner finds nodes by class. |
| `comfy workflow apply` builds a graph from a recipe and checks model names against the live server | You can build a graph without hand edits, but only after the models exist | `recipe.json` in the workflow folder. |
| The official template wraps the graph in a subgraph and includes the 9B upsampler | The server rejects a graph whose model file is missing | A flat 10-node graph without the upsampler. |
| The gated Hugging Face repository hides SHA-256 values without a token | No official hash before login | Read the hashes from the API with a token, then pin them in `models.json`. |
| Hugging Face answers 401 (bad token) or 403 (license not accepted) | The two answers need two different user actions | `horus workflow install` names the right action (`HF_TOKEN_INVALID` or `LICENSE_NOT_ACCEPTED` in the MCP server). |
| `Invoke-WebRequest` in Windows PowerShell 5.1 downloads multi-GB files very slowly | The progress bar slows the transfer | The old download script used `curl.exe`. `horus download-model` uses the HTTP client of Go and has no progress bar. |
| A PowerShell pipe adds a UTF-8 BOM, and curl reads the BOM as part of an option name | `config file option 'header' is unknown` | The old download script wrote the curl config to a temporary file without a BOM. `horus` calls no curl and no PowerShell, so this trap is gone. |
| curl progress output on stderr becomes an error when `ErrorActionPreference` is `Stop` | The download stopped at once | Use `--no-progress-meter`. |
| Git Bash rewrites paths that start with a drive letter inside comma lists | Arguments broke | Set `MSYS_NO_PATHCONV=1` for such calls. |
| The start script does not save the ComfyUI console | No log file to read | `horus start-comfyui` writes `horus-comfyui.out.log` and `horus-comfyui.err.log` in the ComfyUI folder. `GET /internal/logs/raw` returns the in-memory log. |

## 4. Prompt findings

horus follows the caption rules in [How to write the caption](../workflows/it-illustration/README.md#how-to-write-the-caption). These findings led to those rules.

1. **The agent replaces the upsampler.** The upsampler instructions of the authors give a caption form. It names the medium and style first. Then it lists the objects with their positions in the frame. It writes relations as things that you can see. It states negations as what is present.
2. **Naming a fill colour for each object matters most.** In our tests, a subject with a named fill colour for each object scored higher on the style check than the same subject without it. The picture also matched the reference style better.
3. **Weak points.**
   - Exact counts above two are wrong. Example: four cylinders for a request of three.
   - Some relations are missed. In our trials, about 3 of 7 pictures missed one relation. In the read-replicas example, the arrows reach only one of the three small cylinders.
   - A word such as "friendly" does not work without a face.
   - The outline colour sometimes drifts to a nearby navy shade.
4. **Strong points.** Every named main object appeared in all trial pictures. The palette, the background and the line style matched. No readable text appeared. Small barcode-like marks appeared on devices twice.
5. **Concept names come back as text.** A phrase such as "these are the read replicas" produces the words "read replicas" in the picture. Describe only what you can see.
6. **Hex colour codes come back as text.** Use colour names.
7. **Step count.** 8 steps are about 1.8 times faster than 15 steps. They also add pseudo-text and confused shapes. The default stays at 15 steps.

## 5. Measurement and its limits

The style score comes from a script that reads the pixels. It is the geometric mean of five properties:

- the border colour is near the background colour;
- the share of pixels near the 13-colour palette;
- the share of flat-fill pixels;
- the subject coverage (8 to 70 percent of the canvas);
- the share of navy outline pixels.

A blank image or a noise image scores 0. The script is a local tool and horus does not ship it.

| Image set | Mean style score |
|---|---:|
| Blank canvas and noise | 0.0 |
| SD 1.5 checkpoint with LoRA, 8 images | 13.4 |
| FLUX 3 Image Klein, 5 subjects, first attempt | 83.8 |
| Same model, one subject rewritten with the subject rules | 85.3 |
| Reference set of 15 illustrations from a commercial image service | 89.9 |

Limits of these numbers:

- The score measures style only. It does not check objects or relations. We checked objects and relations by eye.
- We fixed the navy tolerance (a colour distance of 18) before the first FLUX run. We did not change it afterwards. One subject scored low (57.9) because of this tolerance, although its content was correct.
- A trial that started from a reference picture scored 89.3. It scored high only because it adds noise to the target picture and removes it again.
- We used one seed for each subject. This is not a statistical benchmark.
- All speed numbers are for one card class. A GPU with more VRAM and a CUDA 13 driver is faster. We did not test it.

## 6. Open items

- **Newer driver.** Test a driver with CUDA 13 support. It can allow the int8 attention kernel and remove the need for `ModelAttentionBackend`.
- **Reference-picture mode.** FLUX 3 Image Klein takes reference pictures through the node `ReferenceMultiLatent`. We did not test it. It can improve palette consistency. It also costs more attention time. The IP-Adapter trial on SD 1.5 showed content leakage, so test for leakage first.
- **Fine-tuning.** A LoRA on the FLUX 3 Image Klein base model, trained on a set of approved illustrations, can close the last gap in the style score. Check the license terms of the model and of the training pictures first. Not started.
- **Faster hardware.** Measure the same workflow on a card with more VRAM. Not started.
- **More subjects.** Run more subjects and more seeds, and add an automatic check for objects and relations. Not started.

See also [running workflows](running-workflows.md#failure-modes) for the failures that you can expect, and [style to workflow](style-to-workflow.md) for the steps that add a new model.
