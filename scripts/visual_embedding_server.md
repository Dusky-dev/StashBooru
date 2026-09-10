# Remote visual embedding worker

`visual_embedding_server.py` lets a separate machine perform EVA02 inference for StashBooru. StashBooru still owns the job queue, image metadata, and SQLite embedding index; the remote service only receives image bytes and returns a 1024-D embedding.

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

The expected model is the pinned embedding-enabled EVA02 file used by `visual_embedding_worker.py`, not the ordinary tagger-only ONNX file.

If the model is not installed yet, the server can use the existing pinned downloader once at startup:

```bash
python3 scripts/visual_embedding_server.py --download-model --host 0.0.0.0 --port 8000
```

The HTTP API itself does not expose model download or any Stash database operations.

## StashBooru setup

Open **Settings > System > Visual Similarity** and enter the remote worker URL, for example:

```text
http://192.168.1.50:8000
```

Enter the same bearer token if one was configured, then click **Save worker**. The status section should change to **Remote**, **Worker ready**, and **Model installed**.

Leave the URL blank and save to switch back to the local worker.

## API

The service intentionally has only two endpoints:

- `GET /v1/status` — reports model compatibility/readiness.
- `POST /v1/embed` — accepts one image as an `application/octet-stream` request body and returns the normalized embedding.

When `STASH_EMBEDDING_SERVER_TOKEN` is set, both endpoints require `Authorization: Bearer <token>`.

Do not expose an unauthenticated worker directly to the public internet. Use a firewall, VPN, or authenticated HTTPS reverse proxy when it is not restricted to a trusted LAN.
