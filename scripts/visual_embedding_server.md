# Remote ML inference worker

`visual_embedding_server.py` lets a separate machine perform EVA02 embedding inference for StashBooru and, optionally, Camie Tagger v2 knowledge inference. StashBooru still owns the job queue, image metadata, SQLite embedding index, and any future metadata application; the remote service only receives image bytes and returns inference results.

## GPU machine setup

From a StashBooru checkout:

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install numpy pillow onnxruntime-gpu
```

`ffmpeg` is optional but recommended as a fallback decoder for formats Pillow cannot open.

Place the same DeepGHS EVA02 embedding model used by StashBooru somewhere on the GPU machine and point the worker at it:

```bash
export STASH_EMBEDDING_MODEL_PATH=/path/to/model.onnx
export STASH_EMBEDDING_ONNX_PROVIDERS=CUDAExecutionProvider,CPUExecutionProvider
export STASH_EMBEDDING_SERVER_TOKEN='replace-with-a-long-random-token'
python3 scripts/visual_embedding_server.py --host 0.0.0.0 --port 8000
```

The expected EVA02 model is the pinned embedding-enabled DeepGHS file used by `visual_embedding_worker.py`, not the ordinary tagger-only ONNX file.

If EVA02 is not installed yet, the server can use the existing pinned downloader once at startup:

```bash
python3 scripts/visual_embedding_server.py --download-model --host 0.0.0.0 --port 8000
```

The HTTP API itself does not expose model download or any Stash database operations.

## Optional Camie Tagger v2

Camie is deliberately **not bundled and never downloaded automatically by StashBooru**. To enable it, keep `camie_tagger_worker.py` beside `visual_embedding_server.py` and `visual_embedding_worker.py`, then manually supply both files from `Camais03/camie-tagger-v2`:

```text
camie-tagger-v2.onnx
camie-tagger-v2-metadata.json
```

By default the worker expects them under its visual-embedding cache in a `camie` subdirectory. The exact expected paths are reported by `GET /v1/camie/status` and by **Settings > System > Visual Similarity**.

You can also set explicit paths on the GPU machine:

```bash
export STASH_CAMIE_MODEL_PATH=/path/to/camie-tagger-v2.onnx
export STASH_CAMIE_METADATA_PATH=/path/to/camie-tagger-v2-metadata.json
```

Camie uses the same `STASH_EMBEDDING_ONNX_PROVIDERS` selection as EVA02, so a CUDA worker can run both models. Inference is serialized so the two models do not compete for the GPU at the same time.

## StashBooru setup

Open **Settings > System > Visual Similarity** and enter the remote worker URL, for example:

```text
http://192.168.1.50:8000
```

Enter the same bearer token if one was configured, then click **Save worker**. EVA02 status should change to **Remote**, **Worker ready**, and **Model installed**. The optional Camie section uses the same URL and token and reports whether its two manually supplied files are present.

Leave the URL blank and save to switch both inference clients back to local workers.

## API

The same worker also supports local/remote media conversion. Update
`media_conversion_worker.py` and `media_upscale_worker.py` alongside this server and install FFmpeg, libjxl
tools and Pillow. See [media converter setup and recovery](../docs/media-converter.md)
for formats, CPU/GPU support, video upload limits and the converter endpoints.

The service exposes these inference endpoints:

- `GET /v1/status` — reports EVA02 model compatibility/readiness.
- `POST /v1/embed` — accepts one image as an `application/octet-stream` request body and returns the normalized 1024-D EVA02 embedding.
- `POST /v1/tag?threshold=0.35&limit=100` — EVA02 tag predictions using its installed tag CSV.
- `GET /v1/convert/capabilities` — installed codecs, tested GPU encoders and optional upscaler installation status.
- `POST /v1/convert` — streams verified converted bytes. Options are JSON in `X-Stash-Conversion-Options`; optional `upscaler` is `waifu2x` or `seedvr2`, with `upscaleScale` 2 or 4. Length, MD5 and base64 JSON metadata are returned in response headers.
- `GET /v1/camie/status` — reports optional Camie model/metadata paths and readiness.
- `POST /v1/camie/tag?threshold=0.492&limit=50` — accepts one image and returns Camie predictions with `name`, `category`, and `score`.

`limit` is a per-category result cap. StashBooru defaults to Camie's macro-optimized threshold of `0.492`.

When `STASH_EMBEDDING_SERVER_TOKEN` is set, all endpoints require `Authorization: Bearer <token>`.

Do not expose an unauthenticated worker directly to the public internet. Use a firewall, VPN, or authenticated HTTPS reverse proxy when it is not restricted to a trusted LAN.

## Update an existing embeddings-only worker

Replace the server and worker together, and put both converter modules in the
same directory. Keep your existing model paths, token and launch environment.
The provided bundle also includes the optional Camie module. Restart your worker:

```bash
python3 visual_embedding_server.py --host 0.0.0.0 --port 8000 --max-upload-mb 4096
```

On Debian install `ffmpeg libjxl-tools python3-pil`; inside a Python virtual
environment also install `pillow` there. On Arch use `ffmpeg libjxl` and install
Pillow in the worker environment. The existing NumPy/ONNX Runtime installation
continues to serve embeddings. Conversion itself does not require model weights.
Use **Recheck worker** in the converter after restarting.

## Optional waifu2x

Install the portable binary and models from the
[waifu2x-ncnn-vulkan project](https://github.com/nihui/waifu2x-ncnn-vulkan).
Set paths in the same environment that starts this server:

```bash
export STASH_WAIFU2X=/absolute/path/waifu2x-ncnn-vulkan
export STASH_WAIFU2X_MODELS=/absolute/path/models-cunet
```

The converter uses 2×/4× upscaling without denoising (`-n -1`). Prefer GPU tries
Vulkan then CPU; explicit CPU passes `-g -1`. The files must be installed before
the option becomes available. Model execution is checked when a job runs.

## Optional SeedVR2

Install the standalone CLI from
[ComfyUI-SeedVR2_VideoUpscaler](https://github.com/numz/ComfyUI-SeedVR2_VideoUpscaler#-run-as-standalone-cli)
in a separate Python environment following its current GPU requirements. A
running ComfyUI server is not needed. Download the selected DiT weights and
`ema_vae_fp16.safetensors` into the same model directory before enabling it.

```bash
export STASH_SEEDVR2_CLI=/absolute/path/seedvr2/inference_cli.py
export STASH_SEEDVR2_PYTHON=/absolute/path/seedvr2/.venv/bin/python
export STASH_SEEDVR2_MODELS=/absolute/path/seedvr2/models/SEEDVR2
export STASH_SEEDVR2_MODEL=seedvr2_ema_3b_fp8_e4m3fn.safetensors
export STASH_SEEDVR2_BLOCKS_TO_SWAP=32
```

This integration uses batch size 1, VAE tiling, CPU offload and block swapping
for the 3B model. The upstream project documents FP8 with offload/tiling for
12–16 GB GPUs; actual VRAM needs depend on dimensions and installation. It does
not promise a particular maximum resolution on an RTX 3060. Only installed
models are advertised; Hugging Face offline mode is enabled during invocation.
SeedVR2 upscaling and tagging are serialized to avoid concurrent model inference.

Choose the upscaler in the converter, then 2× or 4×. This first integration
supports **still images only** and verifies the requested dimensions and frame
count before encoding. Animated/video SeedVR2 restoration is not enabled in this
path. CLI adapters and validation are tested without weights; validate real GPU
inference on the worker before applying a large batch.
