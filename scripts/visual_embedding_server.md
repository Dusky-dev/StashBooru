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

The service exposes these inference endpoints:

- `GET /v1/status` — reports EVA02 model compatibility/readiness.
- `POST /v1/embed` — accepts one image as an `application/octet-stream` request body and returns the normalized 1024-D EVA02 embedding.
- `GET /v1/camie/status` — reports optional Camie model/metadata paths and readiness.
- `POST /v1/camie/tag?threshold=0.492&limit=50` — accepts one image and returns Camie predictions with `name`, `category`, and `score`.

`limit` is a per-category result cap. StashBooru defaults to Camie's macro-optimized threshold of `0.492`.

When `STASH_EMBEDDING_SERVER_TOKEN` is set, all endpoints require `Authorization: Bearer <token>`.

Do not expose an unauthenticated worker directly to the public internet. Use a firewall, VPN, or authenticated HTTPS reverse proxy when it is not restricted to a trusted LAN.
