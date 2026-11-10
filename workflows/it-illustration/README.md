# it-illustration

Text in, image out. The picture is 1536x1024. The model is FLUX 3 Image Klein
(int8 build) running in ComfyUI. This is the first horus example.

The workflow has no style. horus sends the subject to the model exactly as
written. The caller writes the look (style, palette, light, composition) in the
subject. The examples below are flat vector because their subjects ask for flat
vector, with the sentences in
[`prompts/flux3-klein/flat-vector-tech-style.md`](../../prompts/flux3-klein/flat-vector-tech-style.md).
The name `it-illustration` is older than this rule. It does not mean a style.

| Example | Subject |
|---|---|
| ![message queue](examples/message-queue.png) | A message queue between two services |
| ![database with read replicas](examples/read-replicas.png) | A database with read replicas |
| ![load balancer](examples/load-balancer.png) | A laptop, a load balancer and three servers |

Files in this folder:

| File | What it is |
|---|---|
| `manifest.json` | The public contract for the horus MCP server: summary, sizes, subject rules, and the graph inputs that horus sets. |
| `workflow.json` | The graph, in ComfyUI "UI" format. 10 nodes. Do not edit it by hand. |
| `recipe.json` | A comfy-cli recipe that builds the same graph. |
| `models.json` | The model files, with size and SHA-256. `horus workflow install` reads it. |
| `examples/` | Pictures made with this workflow. |

## Models and pairings

| Node | File | Folder |
|---|---|---|
| `UNETLoader` | `flux3-image-klein-int8-convrot.safetensors` (5.83 GB) | `models/diffusion_models/` |
| `CLIPLoader` | `qwen3vl_4b_int8_convrot.safetensors` (4.86 GB) | `models/text_encoders/` |
| `VAELoader` | `flux2-vae.safetensors` (0.34 GB) | `models/vae/` |

The three files must belong together.

- `CLIPLoader.type` must be `flux3`. Another type makes the text encoder output a format that the model cannot read. The result is noise or an unrelated picture, and ComfyUI shows no error.
- The VAE must be `flux2-vae`. A VAE (variational autoencoder) turns the model's internal image (the latent) into pixels. Another VAE decodes the FLUX 3 latent wrongly. The result is coloured noise.

The model license is gated. Accept it on the Hugging Face repo page before the download. See `models.json` for the repo, the revision and the license name.

## The graph

Slot addresses below come from `list_workflow_slots`. ComfyUI gave each node a long random ID when the graph was built. Copy the address from this table or from `list_workflow_slots`.

| Node | Class | Job | Slot address |
|---|---|---|---|
| Load Diffusion Model | `UNETLoader` | Loads the FLUX 3 Klein model. | `1145655848684232.unet_name` |
| Attention | `ModelAttentionBackend` | Selects the attention code (see below). | `1303296545618025.attention` |
| Load CLIP | `CLIPLoader` | Loads the Qwen3-VL text encoder. | `2943881201999604.clip_name`, `2943881201999604.type` |
| Load VAE | `VAELoader` | Loads the VAE. | `1003919775051822.vae_name` |
| **Prompt** | `CLIPTextEncode` | Turns the caption into conditioning. | **`2752976026169904.text`** |
| Negative | `ConditioningZeroOut` | An empty negative prompt. CFG 1 ignores it. | none |
| **Canvas** | `EmptyFlux2LatentImage` | Sets the picture size. | **`3955927623628478.width`**, **`3955927623628478.height`** |
| **Sampler** | `KSampler` | 15 steps, CFG 1, sampler `er_sde`, scheduler `simple`. | **`3394534790677934.seed`**, `3394534790677934.steps` |
| VAE Decode | `VAEDecode` | Latent to pixels. | none |
| **Save Image** | `SaveImage` | Writes the PNG to the ComfyUI `output/` folder. | **`126941226278325.filename_prefix`** |

Bold rows are the slots that a normal run changes.

### Why the attention node exists

The int8 checkpoint stores `comfy_kitchen_int8` as its attention method. That kernel is built for the CUDA 13 runtime, so it needs an NVIDIA driver that supports CUDA 13. On an older driver, the first sampler step fails with `CUDA driver version is insufficient for CUDA runtime version`.

`ModelAttentionBackend` set to `pytorch attention` makes ComfyUI use the standard PyTorch attention instead. The int8 weights still work. Keep the node until your driver supports CUDA 13 and a test run proves that the kernel loads.

## How to write the caption

FLUX 3 expects a long, descriptive caption. Its authors ship a prompt upsampler (a language model that expands a short request into a caption). horus does not install it. The caller does the same job: it writes the whole caption.

caption = subject (horus adds nothing)

Rules for the subject:

1. Write full descriptive sentences. Name the medium and the style first. Then name each object with its position, colour and light.
2. Write the look yourself: medium, palette, light, mood, background and framing. Without them the model picks its own.
3. Describe only what you can see. Do not write the concept name. A phrase such as "these are the read replicas" came back as the words "read replicas" in the picture.
4. Never put hex colour codes such as `#2D7FF9` in the caption. The model draws them as text. Use colour names.
5. If you want no text in the picture, say so in the caption. The model draws text badly.

## Run it through MCP

MCP (Model Context Protocol) is how an agent calls tools. These tools come from the comfy MCP server.

1. `server_info`. Make sure that ComfyUI runs and the GPU shows.
2. Copy `workflow.json` to a scratch folder. Never change the committed file.
3. `set_workflow_slot` on the copy, with `stdout=false`: the caption (`2752976026169904.text`), the seed (`3394534790677934.seed`) and `126941226278325.filename_prefix`.
4. `validate_workflow` on the copy. Read `valid` and `converted_node_count`. Continue only when `valid` is `true` and `converted_node_count` is `10`.
5. `run_workflow(wait=False)`. It returns a `prompt_id`.
6. `job(action="wait", timeout_seconds=600)` with that `prompt_id`.
7. `fetch_outputs` with the `prompt_id` and an output folder.
8. Open the PNG. Check that it matches the subject and the look that the caption asks for. Change one thing in the subject and rerun when it is not.

`run_workflow(wait=True)` has a budget of about 110 s. A model load alone can exceed it, so always use `wait=False`.

## Run it without an agent

```
horus run-illustration -subject "a message queue between two services" -seed 7 -out out/queue.png
```

`horus run-illustration` uses the same code as the MCP tool `generate_image`. It reads `workflow.json`, converts it to the API format with the node definitions of the running server, sets the caption, seed and file prefix, submits the job, waits and saves the PNG. When ComfyUI does not answer, it starts ComfyUI (`HORUS_AUTOSTART=0` turns this off). It prints one JSON line:

```
{"ok":true,"prompt_id":"...","out":"out/queue.png","seed":7,"steps":15,"width":1536,"height":1024,"execution_seconds":217.5,"comfy_output":"..."}
```

On failure it prints `{"ok":false,"error":...}` to stderr and exits with code 1. Options: `-subject-file`, `-steps`, `-width`, `-height`, `-style-file` (an optional JSON `{prefix, suffix}` that you own; default: none), `-url`, `-timeout`, `-workflow`. The server URL is `-url`, or the environment variable `HORUS_COMFYUI_URL`, or `http://127.0.0.1:8188`. `horus run-illustration -h` shows every option.

## Rebuild the graph

`recipe.json` rebuilds the graph with comfy-cli. The model files must exist on the server, because comfy-cli checks the file names. Use `comfy workflow apply` with this recipe. Then check the new file with `validate_workflow`.

## Hardware notes

- Tested on a GTX 1660 Ti class card (6 GB VRAM, 16 GB RAM) with a driver that supports CUDA 12.x only. It works. ComfyUI streams the model through VRAM.
- This GPU has no native bf16, so ComfyUI computes in fp32. That is slow.
- Measured: 217 s to 284 s per picture at 1536x1024 and 15 steps. One run took 470 s. The cause of that run is not verified. A job that ran just before it had been cancelled.
- A GPU with more VRAM and a CUDA 13 driver is faster. Not tested.

## Evidence

All runs on 2026-11-05. Graph: `workflow.json`, 15 steps, CFG 1, 1536x1024.

| Route | Subject | Seed | Seconds | Result |
|---|---|---|---|---|
| MCP | Database with read replicas, hex codes in the house style | 1001 | 217.2 | Fails the check. The picture shows hex codes as text. A first attempt failed with `hostbuf_file_reader_read failed` and passed on the same input. Cause not verified. |
| Script | Message queue | 2024 | 469.9 | Passes. Flat vector, on subject, no text. |
| Script | Database with read replicas | 1001 | 262.4 | Passes after one retry. The first try drew the words "read replicas". |
| MCP | Laptop, load balancer, three servers | 77 | 284.5 | Passes. Flat vector, on subject, no text. |

Runs on 2026-11-06 through the horus MCP server (`generate_image`). A separate headless Claude Code agent (Sonnet,
no file tools, only the horus tools) got a brief for 4 video images. It called `list_workflows`, `describe_workflow`,
then `generate_image` 4 times, and wrote each subject itself. Another agent's jobs shared the GPU queue.

| Route | Subject | Seed | Seconds of the call | Result |
|---|---|---|---|---|
| horus MCP | A window frame over a row of tiles | 3001 | 519 (queue wait and model load included) | Passes. Flat vector, on subject, no text. |
| horus MCP | A cafe counter, a cup, coins, a bar chart | 3002 | 738 (queue wait included) | Passes, with one gap: the subject asked for two coral bars below a baseline; the picture has four green bars. |
| horus MCP | One tile leaves the frame on the left, one enters on the right | 3003 | 219 | Passes. Flat vector, on subject, no text. |
| horus MCP | The frame at the end of the row, a flag, a cup | 3004 | 221 | Passes. Flat vector, on subject, no text. |

`validate_workflow` on `workflow.json`: `valid: true`, `converted_node_count: 10`.

The style is the same across the three examples, but the model does not follow the layout exactly. In the read-replicas picture, the sky-blue arrows reach only one of the three small cylinders.

### Style-neutral check (2026-11-09)

horus used to add a flat-vector house style around every subject. A caller that
asked for a dark space scene got a pale pastel picture. horus now sends the
subject as written. We sent 5 dark-scene prompts from another project, word for
word, with fixed seeds, before and after the change. "Dark pixels" is the share
of pixels with a luminance below 60 of 255. The reference is an image of the
same prompt from a different model.

| Prompt (seed) | Reference | Before: house style | After: subject only | Seconds (after) |
|---|---|---|---|---|
| Cookies on a table in a dark space station (2001) | 95.1% | 20.6%, pale flat vector | 80.3%, dark, photographic | 296.2 |
| Four children before a round window (2002) | 95.6% | 30.7%, pale flat vector | 88.6%, dark silhouettes | 215.8 |
| A child on a dark moon holds a cookie (2003) | 98.7% | not run | 86.2%, dark | 215.8 |
| A space station and a tree of lit nodes (1001) | 98.1% | not run | 98.3%, dark | 218.0 |
| A stone ring gate in deep space (1005) | 98.2% | not run | 93.4%, dark | 217.8 |

Limits that remain are model limits, not style: the node tree has 1, 3 and 4
nodes (the prompt asks for 1, 2 and 4), and a wall panel shows small marks that
look like text.
