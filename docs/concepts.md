# Concepts

A glossary for people who are new to local image AI. Each entry says what the
thing is and what breaks when it is wrong. Agents read this page too.

## ComfyUI

**What it is.** A program that makes images from AI models. It runs on your
computer and shows a web page at `http://127.0.0.1:8188`. You build a workflow
from nodes, and ComfyUI runs it.

**If wrong.** If ComfyUI is not running, every tool call fails. Call
`server_info` first to check.

## Node

**What it is.** One step in a workflow. A node has inputs, outputs, and
settings. Examples: load a model, encode text, sample, decode an image.

**If wrong.** A node name that does not exist on the server fails validation.
Take node names from the `nodes` tool, never from memory.

## Graph and workflow

**What it is.** A graph is nodes joined by links. A workflow is a saved graph.
ComfyUI has two file formats:

- **UI format.** It stores node positions and the layout. The web page saves and loads it.
- **API format.** It stores only nodes, settings, and links. The server runs it.

**If wrong.** An old UI file can fail to convert to the API format. Then
`validate_workflow` checks zero nodes and still says `valid: true`. Compare
`converted_node_count` with the real node count.

## Model family

**What it is.** A group of models with the same design and the same latent
space, for example FLUX 3 or SDXL. Parts of one family fit each other.

**If wrong.** Parts from different families give noise, an error, or a
silent bad image. Never mix families. See [models](models.md#pairing-rules).

## Checkpoint

**What it is.** One file that holds the model weights. Older families put the
diffusion model, the text encoder, and the VAE in one checkpoint. Newer
families split them into separate files.

**If wrong.** If you load a separate VAE for a checkpoint that already has one,
you can get wrong colors. If a part is missing, you get an error.

## Diffusion model (UNet)

**What it is.** The part that makes the image. It starts from noise and
removes the noise step by step. The noise is guided by the text. Some models
call this part a UNet. Newer models use a transformer.

**If wrong.** The wrong diffusion model for the text encoder gives noise.
It is also the largest file, so it sets most of the memory use.

## Text encoder

**What it is.** The part that turns your prompt into numbers. The diffusion
model reads those numbers. Some models use one text encoder. Some use two or
more.

**If wrong.** A wrong text encoder file, or a wrong `type` setting in the
loader node, gives garbage images and no error.

## VAE

**What it is.** The Variational Autoencoder. It converts between pixels and the
small latent image. The workflow uses it to decode the final latent into a
picture.

**If wrong.** A VAE from another family decodes to noise, blur, or strange
colors.

## Latent

**What it is.** A small, compressed form of an image. The diffusion model works
on latents because they are cheaper than pixels. The size of the latent is the
image size divided by a fixed factor.

**If wrong.** An empty latent with a size that the model does not support
gives broken composition or an error. Use sizes that are multiples of 16.

## LoRA

**What it is.** A small file that changes a model to add a style or a subject.
LoRA means Low-Rank Adaptation.

**If wrong.** A LoRA from another family has no effect or breaks the image.
A LoRA with a strength that is too high ruins the image.

## Sampler and scheduler

**What they are.** The sampler is the method that removes noise at each step.
The scheduler sets how big each step is.

**If wrong.** A sampler or scheduler that the model was not made for gives soft
or noisy images. Use the values that the model card or the template gives.

## Steps

**What it is.** The number of denoising passes. More steps cost more time.

**If wrong.** Too few steps give blurry or confused shapes. Too many steps
waste time. A distilled model needs only 4 to 16 steps.

## CFG

**What it is.** Classifier-free guidance scale. A higher value makes the image
follow the prompt more closely.

**If wrong.** A distilled model needs CFG 1. A higher value burns the image
with too much contrast and color.

## Seed

**What it is.** A number that starts the random noise. The same seed with the
same settings gives the same image.

**If wrong.** If you change any other setting, the same seed gives a different
image. Record the seed when you want to repeat a result.

## VRAM and RAM

**What they are.** VRAM is the memory on the graphics card. RAM is the memory
of the computer. The model must fit in VRAM, or ComfyUI moves parts between
VRAM and RAM. A Mac with Apple Silicon has one memory that the CPU and the GPU
share. On a Mac, the GPU backend of PyTorch is MPS (Metal Performance Shaders).
In the Mac test run, ComfyUI used 9.1 GB of the 16 GB, and the Mac swapped.

**If wrong.** A file size is not the peak VRAM use. Activations, text encoders,
image size, and batch size add to it. When memory is short, runs are slow or
the process stops. Do a small test render before a large one.

## Safetensors

**What it is.** A safe file format for model weights. It does not run code when
you load it. The file starts with a header that lists the tensors by name.

**If wrong.** Do not load unknown `.ckpt` or pickle files. They can run code.
Read the header to see what the file holds.

## Quantised model (int8, GGUF)

**What it is.** A model with weights stored in fewer bits. int8 uses 8 bits for
each weight. GGUF is another format that ComfyUI loads through a custom node.
Quantised files are smaller and use less memory.

**If wrong.** A quantised file can need a special kernel or node. If the driver
or the node is missing, the run fails. Quality can also drop a little.

## MCP and the comfy MCP server

**What they are.** MCP is the Model Context Protocol. It lets an AI agent call
tools. The comfy MCP server gives an agent tools to control ComfyUI, such as
`server_info`, `validate_workflow`, and `run_workflow`. The file `.mcp.json`
starts it as `horus comfy-mcp`, which runs the program `comfy-mcp` that
`horus install-comfy-mcp` installs.

**If wrong.** If the client cannot start the server, it shows
`CONNECTION_CLOSED`. See [running workflows](running-workflows.md#failure-modes).

## Slot

**What it is.** One setting of a workflow that a tool can read and change. A
slot address has the form `<node_id>.<widget>`, for example `3.text` for a
prompt. Use `list_workflow_slots` and `set_workflow_slot`.

**If wrong.** A wrong address changes nothing or fails. Addresses differ for each
workflow, so list them each time.
