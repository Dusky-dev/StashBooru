# Docker Installation (for most 64-bit GNU/Linux systems)
StashBooru is supported on most systems that support Docker. Your OS likely ships with or makes available the necessary packages.

## Dependencies
Only `docker` is required for CPU-only use. For the most part your understanding of the technologies can be superficial. So long as you can follow commands and are open to reading a bit, you should be fine.

Installation instructions are available below, and if your distribution's repository ships a current version of docker, you may use that.
https://docs.docker.com/engine/install/

On some distributions, `docker compose` is shipped separately, usually as `docker-cli-compose`. docker-compose is not recommended.

### Get the Docker Compose files

The production compose file uses `ghcr.io/dusky-dev/stashbooru:develop`, which is built from this repository's x86_64 Dockerfile.

```
mkdir stashbooru && cd stashbooru
curl -o docker-compose.yml https://raw.githubusercontent.com/Dusky-dev/StashBooru/develop/docker/production/docker-compose.yml
```

Once you have that file where you want it, modify the settings as you please, and then run:

```
docker compose up -d
```

StashBooru will by default bind to port 9999. This is available in your web browser locally at http://localhost:9999 or on your network at http://YOUR-LOCAL-IP:9999.

### NVIDIA GPU / Vulkan upscaling

`waifu2x-ncnn-vulkan` needs a working Vulkan runtime even when its CPU processing mode (`-g -1`) is selected. StashBooru's x86_64 Alpine image therefore includes `gcompat`, `libstdc++`, and `vulkan-loader`, but the host GPU driver still needs to be exposed to the container.

For NVIDIA, install and configure the NVIDIA Container Toolkit on the Docker host, download the supplied overlay, then recreate StashBooru with it:

```
curl -o docker-compose.nvidia.yml https://raw.githubusercontent.com/Dusky-dev/StashBooru/develop/docker/production/docker-compose.nvidia.yml
docker compose -f docker-compose.yml -f docker-compose.nvidia.yml up -d
```

The overlay requests all NVIDIA GPUs and exposes the `compute`, `video`, `utility`, and `graphics` driver capabilities. `graphics` is required for Vulkan. You can verify the container sees the devices with:

```
docker exec stash sh -c 'ls -l /dev/nvidia* 2>/dev/null || true'
```

The official waifu2x Linux binary also identifies its model family from the model directory path. Configure a directory whose path contains one of:

- `models-cunet`
- `models-upconv_7_anime_style_art_rgb`
- `models-upconv_7_photo`

If waifu2x reports `vkCreateInstance failed`, fix the container/host Vulkan setup first. CPU fallback cannot bypass that particular failure because upstream initializes Vulkan before selecting the CPU processing device.

### Docker
Docker is effectively a cross-platform software package repository. It allows you to ship an entire environment in what's referred to as a container. Containers are intended to hold everything that is needed to run an application from one place to another, making it easy for everyone along the way to reproduce the environment.

The StashBooru Docker image includes StashBooru, ffmpeg, the local inference/conversion workers, and the runtime libraries needed by the supported local tooling. External model weights and optional third-party upscaler executables remain user-managed.

### docker compose
Docker Compose lets you specify how and where to run containers and manage their environment. The `docker-compose.yml` file in this folder provides the normal StashBooru service; optional overlays such as `docker-compose.nvidia.yml` add host-specific capabilities without making them mandatory for everyone.

The latest `develop` image is recommended when testing current StashBooru development builds.
