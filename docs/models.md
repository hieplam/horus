# Verified model catalog

This catalog lists model sets that someone downloaded, checked against the
publisher's hash, and ran end to end. An agent can trust a row here. An agent
must not add a row for a file that has no publisher hash.

Model files never go into this repository. They are gigabytes of binary data.
Record them here instead. To add a model, follow
[style to workflow](style-to-workflow.md#phase-b-get-the-model).

## Format of a row

Each model set has one section. The section has a header list and one table.
The table has one row for each file.

| Column | Meaning |
|---|---|
| Role | What the file does: diffusion model, checkpoint, text encoder, VAE, or LoRA. |
| File | Exact filename. |
| Folder | Folder under ComfyUI's `models/` directory. |
| Bytes | Exact size. Compare it with the size on disk. |
| SHA-256 | Hash from the publisher. Compare it with the hash of the downloaded file. |

Each section also records the source repository, the pinned revision, the
license, the family, and the pairing rules.

## FLUX 3 Image Klein (int8 set)

- Family: FLUX 3 (latent space of FLUX.2)
- Source: `black-forest-labs/flux-3-image-klein-comfy` on Hugging Face
- Revision: `9d36114cf7a2e03a45c28f5cc4987d20ebfdb6c1`
- License: FLUX Kommunity License. The repository is gated. Accept the license on the repository page before you download.
- Used by: [`workflows/it-illustration`](../workflows/it-illustration/README.md)

| Role | File | Folder | Bytes | SHA-256 |
|---|---|---|---:|---|
| Diffusion model (step-distilled, int8) | `flux3-image-klein-int8-convrot.safetensors` | `diffusion_models` | 5,827,874,779 | `847d8933548afd93df4af2cac2498e70891bf28d9dae06d695f536992e168fe2` |
| Text encoder (Qwen3-VL 4B, int8) | `qwen3vl_4b_int8_convrot.safetensors` | `text_encoders` | 4,858,559,240 | `32de113cbf8282b02cbff0794b637b9d6676da09314a1db5a30bcea9bf4ee01c` |
| VAE | `flux2-vae.safetensors` | `vae` | 336,211,292 | `868fe7b343cc8f3a19dbcfcafbc3d5f888802be3f89bd81b65b3621a066ce8f3` |

Total size: about 11 GB. Install it with
[`scripts/install-workflow-models.ps1`](../scripts/install-workflow-models.ps1).

## Pairing rules

A pairing rule says which files work together. Models from different families
are not interchangeable.

1. **Use all three files from one set.** The diffusion model, the text encoder, and the VAE of a set were trained together.
2. **The VAE must match the family.** FLUX 3 uses the `flux2-vae` latent space. A different VAE decodes the latent to noise or mush.
3. **The text encoder type setting must match the family.** For FLUX 3, the loader node setting `type` must be `flux3`. The file name does not set it. A text encoder with the wrong `type` gives garbage output and no error.
4. **Keep distilled-model settings.** A distilled model needs few steps (4 to 16) and CFG 1. A higher CFG burns the image.
5. **Check for bundled parts.** A checkpoint can contain its own VAE or text encoders. If it does, do not load them again. See [check fit and needs](style-to-workflow.md#6-check-fit-and-needs).
6. **One family for LoRAs.** A LoRA works only with the family it was trained for.

## Known hardware limit for this set

The int8 build asks for an attention kernel that needs a recent CUDA driver. On
an older NVIDIA driver the first sampler step fails. The fix is the core node
`ModelAttentionBackend` set to `pytorch attention`. See
[running workflows](running-workflows.md#failure-modes).
