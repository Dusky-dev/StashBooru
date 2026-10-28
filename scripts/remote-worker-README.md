# StashBooru remote processing worker

This ZIP contains the complete **matching source modules** for the remote
worker. Extract the whole ZIP into one directory. Do **not** copy just
`remote_worker_server.py`: it imports all the bundled companion modules.

## Start

Requires Python 3.11 or newer. Install Python on the worker PC,
then from the **extracted directory**:

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
# Install ffmpeg and ffprobe from your operating system for conversion, decoding,
# Video overlap sampling and model input fallback.
export STASH_REMOTE_WORKER_TOKEN='choose-a-long-random-secret'
python3 remote_worker_server.py --host 0.0.0.0 --port 8000
```

On Windows activate with `.venv\\Scripts\\activate` and use `python`.
For NVIDIA GPU ONNX inference, replace the CPU ONNX Runtime with the matching
GPU build **in the same virtual environment**:

```bash
pip uninstall -y onnxruntime
pip install onnxruntime-gpu
export STASH_REMOTE_WORKER_ONNX_PROVIDERS=CUDAExecutionProvider,CPUExecutionProvider
```

Run this read-only capability check on the worker machine (or another machine
that can reach the worker) after it starts:

```bash
python3 remote_worker_check.py --url http://127.0.0.1:8000
```

The checker reads `STASH_REMOTE_WORKER_TOKEN` for bearer authentication.
The original `visual_embedding_server.py` launcher and
`STASH_EMBEDDING_SERVER_*` / `STASH_EMBEDDING_ONNX_PROVIDERS` settings are
still accepted for older installations. New variable names take priority.
It checks EVA02, Camie, PixAI/foidfinder, conversion/upscaling, Video overlap and restoration.
It does not submit media, run inference, download models or change any files.

Then in **StashBooru → Settings → Processing → Worker**, save the worker URL
and the same bearer token. Configure each tool's worker choice separately.

## What is included

- EVA02 embeddings and tagging (model required separately)
- Camie Tagger v2 (optional ONNX model and metadata, installed on demand)
- PixAI Tagger v1 (optional quantized model and isolated CUDA/MLX runtime, installed on demand)
- Conversion (FFmpeg, libjxl and other installed codecs; availability is probed)
- Waifu2x / SeedVR2 upscaling (external executables and weights required separately)
- Video overlap sampling (FFmpeg required)
- Mask restoration (Diffusers/PyTorch, SD1.5 inpainting weights required separately)

This is a **source archive, not a compiled executable or a bundled GPU setup**.
Capabilities are conditional on installed codecs, runtimes and model files.
No model weights, GPU drivers, secrets or download-on-startup behavior are bundled.
Use a firewall or VPN/HTTPS reverse proxy if the worker is outside a trusted LAN.
Avoid exposing an unauthenticated worker to the public internet.

For model paths, feature-specific requirements, API endpoints and limits, see:
https://github.com/Dusky-dev/StashBooru/blob/develop/scripts/remote_worker_server.md

Use the bundle from the **same StashBooru release** as your server to avoid
version/protocol mismatches; development builds use the `latest_develop` asset.

## Optional tagger installation

In StashBooru Settings → Processing → Similarity and tagging, choose **Install Camie**
or **Install PixAI Tagger v1**. Neither is downloaded at worker startup.
Downloads execute on the machine selected for inference. A remote worker must
have `STASH_REMOTE_WORKER_TOKEN` configured for installation or deletion.

Camie (GPL-3.0): https://huggingface.co/Camais03/camie-tagger-v2

PixAI / foidfinder (Apache-2.0): https://github.com/AlpinDale/foidfinder

foidfinder requires Python 3.12+ (or uv to provision it) and on NVIDIA
CUDA Toolkit 12.8+ with nvcc or precompiled CUDA kernels. The installer installs
its backend from a pinned upstream Git commit in a dedicated environment.
Both model downloads verify Hugging Face content hashes before installation.
