# it-illustration

Text in, flat-vector illustration of a tech topic out. The picture is 1536x1024.
The model is FLUX 3 Image Klein (int8 build) running in ComfyUI. This is the
first horus example.

| Example | Subject |
|---|---|
| ![message queue](examples/message-queue.png) | A message queue between two services |
| ![database with read replicas](examples/read-replicas.png) | A database with read replicas |
| ![load balancer](examples/load-balancer.png) | A laptop, a load balancer and three servers |

Files in this folder:

| File | What it is |
|---|---|
| `workflow.json` | The graph, in ComfyUI "UI" format. 10 nodes. Do not edit it by hand. |
| `recipe.json` | A comfy-cli recipe that builds the same graph. |
| `models.json` | The model files, with size and SHA-256. The install script reads it. |
| `house-style.json` | Two text pieces (`prefix`, `suffix`) that surround every subject. |
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

FLUX 3 expects a long, descriptive caption. Its authors ship a prompt upsampler (a language model that expands a short request into a caption). horus does not install it. The agent does the same job: it writes the subject, and the house style goes around it.

caption = `prefix` + subject + `suffix` (from `house-style.json`)

Rules for the subject:

1. Write 2 to 4 full sentences. Name each object, its colour, its position, and what connects to what.
2. Use only the palette colours: navy, white, sky blue, lilac, mint green, sun yellow, coral.
3. Describe only what you can see. Do not write the concept name. A phrase such as "these are the read replicas" came back as the words "read replicas" in the picture.
4. Never put hex colour codes such as `#2D7FF9` in the caption. The model draws them as text. Use colour names.
5. Screens and panels show "abstract coloured bars". Never ask for readable content.
6. Keep the scene centred with wide margins.

## Run it through MCP

MCP (Model Context Protocol) is how an agent calls tools. These tools come from the comfy MCP server.

1. `server_info`. Make sure that ComfyUI runs and the GPU shows.
2. Copy `workflow.json` to a scratch folder. Never change the committed file.
3. `set_workflow_slot` on the copy, with `stdout=false`: the caption (`2752976026169904.text`), the seed (`3394534790677934.seed`) and `126941226278325.filename_prefix`.
4. `validate_workflow` on the copy. Read `valid` and `converted_node_count`. Continue only when `valid` is `true` and `converted_node_count` is `10`.
5. `run_workflow(wait=False)`. It returns a `prompt_id`.
6. `job(action="wait", timeout_seconds=600)` with that `prompt_id`.
7. `fetch_outputs` with the `prompt_id` and an output folder.
8. Open the PNG. Check that it is flat vector, on subject and free of text. Change one thing in the subject and rerun when it is not.

`run_workflow(wait=True)` has a budget of about 110 s. A model load alone can exceed it, so always use `wait=False`.

## Run it with the script

```
python scripts/run-illustration.py --subject "a message queue between two services" --seed 7 --out out/queue.png
```

The script needs only the Python standard library. It reads `workflow.json`, converts it to the API format with the node definitions of the running server, sets the caption, seed and file prefix, submits the job, waits and saves the PNG. It prints one JSON line:

```
{"ok": true, "prompt_id": "...", "out": "out/queue.png", "seed": 7, "steps": 15, "width": 1536, "height": 1024, "execution_seconds": 217.5, "comfy_output": "..."}
```

On failure it prints `{"ok": false, "error": "..."}` to stderr and exits with code 1. Options: `--subject-file`, `--steps`, `--width`, `--height`, `--no-style`, `--url`, `--timeout`. The server URL is `--url`, or the environment variable `HORUS_COMFYUI_URL`, or `http://127.0.0.1:8188`.

From Python, import the function `generate(subject, seed, out, ...)`. It returns a dict with the same keys and raises `IllustrationError` on failure.

## Rebuild the graph

`recipe.json` rebuilds the graph with comfy-cli. The model files must exist on the server, because comfy-cli checks the file names. Use `comfy workflow apply` with this recipe. Then check the new file with `validate_workflow`.

## Hardware notes

- Tested on an NVIDIA GTX 1660 Ti with 6 GB VRAM, 16 GB RAM and a driver that supports CUDA 12.5. It works. ComfyUI streams the model through VRAM.
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

`validate_workflow` on `workflow.json`: `valid: true`, `converted_node_count: 10`.

The style is the same across the three examples, but the model does not follow the layout exactly. In the read-replicas picture, the sky-blue arrows reach only one of the three small cylinders.
