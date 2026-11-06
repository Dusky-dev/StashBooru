# Processing

Configure processing services, saved defaults and optional models in **Settings → Processing**. Run new processing jobs from selected Images or Videos, or from **Settings → Tasks**. Save each section separately.

## Processing worker

The optional remote worker can handle visual similarity, tagging, conversion, upscaling, restoration and Video indexing. Each operation checks the capabilities it requires. Leave **Worker URL** empty when you only want supported local processing.

1. Download the complete [remote worker ZIP](https://github.com/Dusky-dev/StashBooru/releases/download/latest_develop/stash-remote-worker.zip) or a matching [stable release](https://github.com/Dusky-dev/StashBooru/releases).
2. Extract it on the worker machine, install its Python requirements and start `visual_embedding_server.py`. Run the included `remote_worker_check.py` to check installed services.
3. Enter the URL and optional bearer token, then **Save worker**. Leave the token input blank to keep an existing token, or choose **Clear token** to remove it.

Optional GPU runtimes, models and encoders are installed separately on the machine that actually performs the work. Keep worker and server versions compatible. See the [worker setup and capability guide](https://github.com/Dusky-dev/StashBooru/blob/develop/scripts/visual_embedding_server.md). Development ZIP updates follow successful release builds.

## Similarity and tagging

**Same image / variants (pHash)** is for visually matching copies and variants; it requires Image perceptual hashes generated from Tasks. **Related content (EVA02)** finds visually related subjects or compositions and uses a separate model and Image index. Neither method proves identical bytes. Open **Review similar images** to inspect matches before acting.

**Related-content index:** Install the EVA02 model if needed and generate embeddings. Unchanged Images are skipped. The index resides on the StashBooru server even when remote inference is used. Model downloads begin only after a user action, and the details panel shows the installed revision and model path.

**Tagging defaults:** Image tagging and Video frame tagging share saved thresholds. Higher thresholds retain fewer proposed tags. Filename metadata can work without Camie; supported filename tokens are `%artist%`, `%copyright%`, `%character%`, `%md5%` and `%ext%`. Filename identities take precedence over inferred identities. Hierarchy-inheritance defaults are shared with System and Tagging.

**Bulk auto-tagging:** When enabled, reviewing metadata for multiple Images can start a background job after one confirmation, applying configured Camie, EVA02 and filename predictions without a separate approval for each Image. Booru and library suggestions are not included. When disabled, bulk selection offers a per-Image review path.

**Camie Tagger v2:** Optional Character, Copyright, Artist and general Tag predictions. Install downloads the model and metadata to the selected worker when requested.

**PixAI Tagger v1:** Optional higher-resolution anime tagging. Its installer downloads the model and optimized runtime on the selected worker. NVIDIA installations require CUDA Toolkit 12.8+ (`nvcc`) or compatible precompiled inference kernels. Model/runtime licensing is Apache-2.0. Both taggers have status and details panels so installations remain opt-in.

## Media converter

Conversion rules configure new jobs. Run **Estimate**, a verified **Trial**, or a conversion from selected media to review output before replacing original files. Original-file restore history is accessible from the converter.

**Format rules:** Each input format has an output, quality, effort, optional decode speed and codec-specific options. The **Enabled** checkbox determines whether that saved rule can be selected automatically.

- **Enabled:** Automatic conversion uses that rule for its matching format.
- **Disabled:** The rule and all its settings are retained, but automatic conversion is *not* allowed for an input with that disabled rule. In the Media converter itself, an automatic job reports that the selected rule is disabled; choose an output format explicitly or re-enable the rule. In the **Booru Browser**, downloads using converter defaults still import normally: conversion is skipped for disabled formats and the original is kept.
- **Missing rule:** An input without a specific saved rule uses **Other images** or **Other videos**, or the existing safe animation fallback. If the applicable category rule is disabled, automatic conversion is blocked for inputs relying on it. A format rule that is disabled is *not* silently replaced by a generic rule.

Already running jobs retain the settings selected when they started. Older conversion configs without enabled flags treat all existing rules as enabled. Removing a rule is different from disabling it: removal discards its saved rule definition, while disabling preserves its settings.

**Encoding settings:** Higher quality retains more detail and higher effort takes longer. JPEG XL quality 100 is lossless; higher JXL decoding-speed tiers (0–4) may reduce compression efficiency or quality. Compatible AV1/libaom encoders expose a low-complexity decode toggle. Format-specific controls, such as lossless WebP, appear only when supported by the selected output. Unsupported controls are omitted at job time, while saved preferences remain editable.

**Run on:** Automatic mode prefers the configured remote worker when available, otherwise runs locally. Explicit local-only or remote-only selections do not fall back to the other machine. Processing hardware normally prefers GPU and can fall back to CPU.

**Compression savings:** Configure minimum saved bytes and percent before applying results. Allowing larger output is a per-rule override; check an individual trial first. Animation detection distinguishes APNG and animated WebP from still files. For older library entries, run **Animation inspection** under Tasks.

**Saved trials:** Trial outputs use storage separate from original-file restore backups. A trial cache limit of zero disables saved trials. Expired outputs are cleaned during startup and converter jobs; reducing the cache limit can evict older reviewed trials on a subsequent job. Trial expiry is configured in hours.

## Image upscaling

Select **waifu2x** or **SeedVR2**, scale, hardware and output format for new upscaling jobs. These job defaults are stored in your browser; the optional local model paths are stored on the server. New upscaled copies preserve source metadata and the source Image. Replacing an Image is a separate action in the job dialog, with its own restore cache. Converter compression savings thresholds do not apply to upscaling.

**Local model setup:** The optional path fields configure this StashBooru server, not a remote worker. Leave fields blank to use process-environment or worker defaults. When remote processing is selected, configure equivalent paths on the remote machine.

- **waifu2x:** `STASH_WAIFU2X` executable path (blank uses `waifu2x-ncnn-vulkan` from PATH); `STASH_WAIFU2X_MODELS` points to model files including `.param` and `.bin`.
- **SeedVR2:** `STASH_SEEDVR2_CLI` locates `inference_cli.py`; `STASH_SEEDVR2_PYTHON` chooses Python (blank uses the converter worker Python); `STASH_SEEDVR2_MODELS` should contain `ema_vae_fp16.safetensors`; `STASH_SEEDVR2_MODEL` overrides the DiT model filename. `STASH_SEEDVR2_BLOCKS_TO_SWAP` uses worker default 32 when set to 0.

**Save local paths** checks the worker's readiness. **Reload** discards unsaved local path changes and reads the saved server settings; it does not change browser upscaling job defaults.
