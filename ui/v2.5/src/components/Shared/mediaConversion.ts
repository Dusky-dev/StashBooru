export const conversionEndpoint = "image/converter";

export interface ConversionFormat {
  id: string;
  label: string;
  extension: string;
  family: string;
  cpu: string[];
  gpu: string[];
  controls: string[];
  decodingSpeedLevels?: number;
  available: boolean;
}

export interface ConversionEncodingDefaults {
  quality: number;
  effort: number;
  decodingSpeed?: number;
  fasterDecoding?: number;
  allowLarger?: boolean;
  lossless?: boolean;
  dropAudio?: boolean;
  allowAlphaLoss?: boolean;
}

export interface ConversionConfig {
  cacheLimitBytes: number;
  formatDefaults: Record<string, string>;
  encodingDefaults: Record<string, ConversionEncodingDefaults>;
  backend: string;
}

export interface ConversionUpscaler {
  id: string;
  label: string;
  available: boolean;
  cpu: boolean;
  notice: string;
}

export interface UpscalingDefaults {
  upscaler: string;
  scale: 2 | 4;
  hardware: "auto" | "cpu" | "gpu";
  format: string;
}

const upscalingDefaultsKey = "stashbooru-upscaling-defaults";

export const defaultUpscalingDefaults: UpscalingDefaults = {
  upscaler: "waifu2x",
  scale: 2,
  hardware: "auto",
  format: "auto",
};

export function loadUpscalingDefaults(): UpscalingDefaults {
  try {
    const saved = window.localStorage.getItem(upscalingDefaultsKey);
    if (!saved) return defaultUpscalingDefaults;
    const value = JSON.parse(saved) as Partial<UpscalingDefaults>;
    return {
      upscaler:
        typeof value.upscaler === "string"
          ? value.upscaler
          : defaultUpscalingDefaults.upscaler,
      scale: value.scale === 4 ? 4 : 2,
      hardware:
        value.hardware === "cpu" || value.hardware === "gpu"
          ? value.hardware
          : "auto",
      format:
        typeof value.format === "string" && value.format
          ? value.format
          : "auto",
    };
  } catch {
    return defaultUpscalingDefaults;
  }
}

export function saveUpscalingDefaults(value: UpscalingDefaults) {
  window.localStorage.setItem(upscalingDefaultsKey, JSON.stringify(value));
}

export interface ConversionInputFormat {
  id: string;
  label: string;
  family: string;
}

export interface ConversionSettings {
  config: ConversionConfig;
  inputFormats: ConversionInputFormat[];
  outputFormats: ConversionFormat[];
}

export interface ConversionPlan {
  input: string;
  output: string;
  count: number;
  quality: number;
  effort: number;
  decodingSpeed: number;
  fasterDecoding?: number;
  allowLarger?: boolean;
  lossless?: boolean;
  dropAudio?: boolean;
  allowAlphaLoss?: boolean;
  error?: string;
}

export async function conversionResponse<T>(r: Response): Promise<T> {
  if (!r.ok) throw new Error((await r.text()) || r.statusText);
  return r.json() as Promise<T>;
}
