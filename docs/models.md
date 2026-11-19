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
`horus setup it-illustration -yes` ([step 2 of setup](setup.md#step-2-set-up-the-workflow)), or let an agent call
the tool `install_workflow`.

## Vietnamese speech (VieNeu-TTS v3 Turbo with Whisper small)

- Family: text to speech on the CPU. The files do not go into ComfyUI.
- Sources on Hugging Face, each with its pinned revision and license:
  - `pnnbao-ump/VieNeu-TTS-v3-Turbo`, revision `61b85e3d937fbbacb387714180e8182823512523`, Apache-2.0
  - `OpenMOSS-Team/MOSS-Audio-Tokenizer-Nano-ONNX`, revision `ceff0d0749bfb3fa2d61149794ec6feef0d1e1ae`, Apache-2.0
  - `Systran/faster-whisper-small`, revision `536b0662742c02347bc0e980a01041f333bce120`, MIT
- Used by: [`workflows/vi-speech`](../workflows/vi-speech/models.json)
- The column Folder is a folder under the speech model folder of horus, not under ComfyUI's `models/`.

| Role | File | Folder | Bytes | SHA-256 |
|---|---|---|---:|---|
| Speech model (VieNeu-TTS v3 Turbo) | `config.json` | `vieneu` | 1,553 | `eee8e032cb936a60312f594a8156c086173a9c0255a545bd11a448f22a7c77ae` |
| Speech model (VieNeu-TTS v3 Turbo) | `denoiser.onnx` | `vieneu` | 42,661,414 | `b7621953291cfe05e695a9c0ff4255aa2f93239fc17c26627e18b7b6b8f72f0b` |
| Speech model (VieNeu-TTS v3 Turbo) | `config.json` | `vieneu/onnx_update` | 2,152 | `17d89d414ee302a82db7b330bf57b4cdf8541569392119c81f552178cafcb79b` |
| Speech model (VieNeu-TTS v3 Turbo) | `tokenizer.json` | `vieneu/onnx_update` | 22,320 | `6cc6bcbe380b8c37bd9f2514e37c5dfa3e00e122c6e3125dae5c4afe48e39158` |
| Speech model (VieNeu-TTS v3 Turbo) | `vieneu_acoustic_cached.onnx` | `vieneu/onnx_update` | 7,207,223 | `f631e3387c788c3d8b9a5ac5df94952af5bc4c4d1049ff8a751e76a246fff2d4` |
| Speech model (VieNeu-TTS v3 Turbo) | `vieneu_backbone_shared.data` | `vieneu/onnx_update` | 415,319,040 | `c7c072193db33d0542457e2612c7272c44c4279d1cafaf0aa4c379964911db2f` |
| Speech model (VieNeu-TTS v3 Turbo) | `vieneu_decode_step.onnx` | `vieneu/onnx_update` | 306,134 | `bedc379cea61ea5d616312750d95ad3924e055856662d19187a889a5edc24ceb` |
| Speech model (VieNeu-TTS v3 Turbo) | `vieneu_prefill.onnx` | `vieneu/onnx_update` | 324,499 | `27f8b064f6b57b5448e95d095f1959588c005d614678045c2b97ecccf3b7a0f7` |
| Speech model (VieNeu-TTS v3 Turbo) | `vieneu_v3_heads.npz` | `vieneu/onnx_update` | 52,219,622 | `fb22484baa424bbb775133a6e5f0d00d6299b2b256fbe3312a864b85b9aed01e` |
| Audio codec (MOSS Audio Tokenizer Nano) | `codec_browser_onnx_meta.json` | `codec` | 17,036 | `3e291c883bb7d11ff2fe8e964e3e495519760358859f35c951254c7741592731` |
| Audio codec (MOSS Audio Tokenizer Nano) | `moss_audio_tokenizer_decode_full.onnx` | `codec` | 681,902 | `0fbbafe3fd4afa2a019af5c5ced204af6e2d1db044fa40f021525d2aee95b4ac` |
| Audio codec (MOSS Audio Tokenizer Nano) | `moss_audio_tokenizer_decode_shared.data` | `codec` | 44,198,912 | `e69d52e0f4e84ca27850557ee54face46632d3a5a16c89bd246c7c408466dcad` |
| Audio codec (MOSS Audio Tokenizer Nano) | `moss_audio_tokenizer_decode_step.onnx` | `codec` | 351,400 | `9527c86a29e1837edec1f74db57d5eeaadb3a715af3382703566460afed25855` |
| Audio codec (MOSS Audio Tokenizer Nano) | `moss_audio_tokenizer_encode.data` | `codec` | 44,507,136 | `aa751265b2bab2887eac224484546b194875aa7494b607115439b3dc6b228a2c` |
| Audio codec (MOSS Audio Tokenizer Nano) | `moss_audio_tokenizer_encode.onnx` | `codec` | 815,775 | `eadea4a645abdcf98714c7aead122ee2ce7da6e080f9f80b977cd1ca8e19473a` |
| Speech recognition (Whisper small) | `config.json` | `whisper-small` | 2,370 | `b55496ac7940a7ae47d2c01eab40edfd8701feec1229d9cce3b40014383fb828` |
| Speech recognition (Whisper small) | `model.bin` | `whisper-small` | 483,546,902 | `3e305921506d8872816023e4c273e75d2419fb89b24da97b4fe7bce14170d671` |
| Speech recognition (Whisper small) | `tokenizer.json` | `whisper-small` | 2,203,239 | `fb7b63191e9bb045082c79fd742a3106a12c99513ab30df4a0d47fa6cb6fd0ab` |
| Speech recognition (Whisper small) | `vocabulary.txt` | `whisper-small` | 459,861 | `34ce3fe1c5041027b3f8d42912270993f986dbc4bb34cf27f951e34a1e453913` |

Total size: 1,094,848,490 bytes, about 1.1 GB. The three sets work only
together: the codec turns the output of the speech model into audio, and the
speech recognition model reads the transcript of each file.

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
