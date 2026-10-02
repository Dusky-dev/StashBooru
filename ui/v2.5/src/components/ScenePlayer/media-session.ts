import videojs, { VideoJsPlayer } from "video.js";

class MediaSessionPlugin extends videojs.getPlugin("plugin") {
  private static instances = new Set<MediaSessionPlugin>();
  private static active?: MediaSessionPlugin;
  private metadata?: MediaMetadata;

  constructor(player: VideoJsPlayer) {
    super(player);
    MediaSessionPlugin.instances.add(this);
    this.on("dispose", () => {
      MediaSessionPlugin.instances.delete(this);
      if (MediaSessionPlugin.active !== this) return;
      MediaSessionPlugin.active = undefined;
      const previous = [...MediaSessionPlugin.instances]
        .reverse()
        .find((plugin) => plugin.player && !plugin.player.isDisposed());
      if (previous) previous.activate();
      else if ("mediaSession" in navigator) {
        for (const action of [
          "play",
          "pause",
          "nexttrack",
          "previoustrack",
        ] as const) {
          navigator.mediaSession.setActionHandler(action, null);
        }
        navigator.mediaSession.metadata = null;
        navigator.mediaSession.playbackState = "none";
      }
    });

    player.ready(() => {
      if (player.isDisposed()) return;
      player.addClass("vjs-media-session");
      this.activate();
    });

    player.on("play", () => {
      this.activate();
    });

    player.on("pause", () => {
      this.updatePlaybackState();
    });
    this.updatePlaybackState();
  }

  // manually set poster since it's only set on useEffect
  public setMetadata(title: string, artist: string, poster: string): void {
    if ("mediaSession" in navigator) {
      this.metadata = new MediaMetadata({
        title,
        artist,
        artwork: [
          {
            src: poster || this.player.poster() || "",
            type: "image/jpeg",
          },
        ],
      });
      if (MediaSessionPlugin.active === this)
        navigator.mediaSession.metadata = this.metadata;
    }
  }

  public activate(): void {
    if (
      !this.player ||
      this.player.isDisposed() ||
      !("mediaSession" in navigator)
    )
      return;
    MediaSessionPlugin.active = this;
    this.setActionHandlers();
    navigator.mediaSession.metadata = this.metadata ?? null;
    this.updatePlaybackState();
  }

  private updatePlaybackState(): void {
    if (
      "mediaSession" in navigator &&
      MediaSessionPlugin.active === this &&
      this.player
    ) {
      const playbackState = this.player.paused() ? "paused" : "playing";
      navigator.mediaSession.playbackState = playbackState;
    }
  }

  private setActionHandlers(): void {
    // method initialization
    navigator.mediaSession.setActionHandler("play", () => {
      this.player.play();
    });
    navigator.mediaSession.setActionHandler("pause", () => {
      this.player.pause();
    });
    navigator.mediaSession.setActionHandler("nexttrack", () => {
      this.player.skipButtons()?.handleForward();
    });
    navigator.mediaSession.setActionHandler("previoustrack", () => {
      this.player.skipButtons()?.handleBackward();
    });
  }
}

videojs.registerPlugin("mediaSession", MediaSessionPlugin);

declare module "video.js" {
  interface VideoJsPlayer {
    mediaSession: () => MediaSessionPlugin;
  }
}

export default MediaSessionPlugin;
