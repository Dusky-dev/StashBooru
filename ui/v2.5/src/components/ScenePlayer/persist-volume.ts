import videojs, { VideoJsPlayer } from "video.js";
import localForage from "localforage";

const levelKey = "volume-level";
const mutedKey = "volume-muted";

interface IPersistVolumeOptions {
  enabled?: boolean;
}

class PersistVolumePlugin extends videojs.getPlugin("plugin") {
  enabled: boolean;

  constructor(player: VideoJsPlayer, options?: IPersistVolumeOptions) {
    super(player, options);

    this.enabled = options?.enabled ?? true;

    player.on("volumechange", () => {
      if (this.enabled) {
        localForage.setItem(levelKey, player.volume());
        localForage.setItem(mutedKey, player.muted());
      }
    });

    player.ready(() => {
      this.ready();
    });
  }

  private ready() {
    // Storage reads can finish after a preview is closed or its Video changes.
    // VideoJS clears this.player during disposal, so retain and check the owner.
    const player = this.player;
    localForage.getItem<number>(levelKey).then((value) => {
      if (value !== null && !player.isDisposed()) {
        player.volume(value);
      }
    });

    localForage.getItem<boolean>(mutedKey).then((value) => {
      if (value !== null && !player.isDisposed()) {
        player.muted(value);
      }
    });
  }
}

// Register the plugin with video.js.
videojs.registerPlugin("persistVolume", PersistVolumePlugin);

declare module "video.js" {
  interface VideoJsPlayer {
    persistVolume: () => PersistVolumePlugin;
  }
  interface VideoJsPlayerPluginOptions {
    persistVolume?: IPersistVolumeOptions;
  }
}

export default PersistVolumePlugin;
