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
  /** For still-or-animated choices: whether still images can use it. */
  stillAvailable?: boolean;
}

/** Upscaling writes still images only, so the animated encoder is optional. */
export const availableForStill = (format: ConversionFormat) =>
  format.stillAvailable ?? format.available;

export interface ConversionEncodingDefaults {
  quality: number;
  effort: number;
  autoEffort?: boolean;
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
  formatRulesEnabled?: Record<string, boolean>;
  encodingDefaults: Record<string, ConversionEncodingDefaults>;
  backend: string;
  savings: SavingsThresholds;
  trialCacheLimitBytes: number;
  trialTTLHours: number;
}

export interface SavingsThresholds {
  minimumSavedBytes: number;
  minimumSavedPercent: number;
}

export interface ConversionTrial {
  id: string;
  batch: string;
  target: { kind: "image" | "scene"; id: number };
  createdAt: string;
  expiresAt: string;
  status: string;
  error?: string;
  before: {
    image?: {
      path: string;
      basename: string;
      size: number;
      width?: number;
      height?: number;
    };
    video?: {
      path: string;
      basename: string;
      size: number;
      width?: number;
      height?: number;
    };
  };
  result: {
    width: number;
    height: number;
    frames: number;
    size: number;
    seconds: number;
    encoder: string;
  };
  format: ConversionFormat;
  options: {
    quality: number;
    effort: number;
    hardware: string;
    decodingSpeed?: number;
    lossless: boolean;
    dropAudio: boolean;
    allowAlphaLoss: boolean;
  };
  savings: SavingsThresholds;
  verified: boolean;
  retained: boolean;
  backend: string;
  recordID?: string;
  notices: string[];
}

export interface TrialStats {
  verified: number;
  skipped: number;
  failed: number;
  applied: number;
  potentialSavedBytes: number;
  weightedSavedPercent: number;
  cacheBytes: number;
}

export interface ConversionEstimate {
  total: number;
  sampled: number;
  covered: boolean;
  estimatedSavedBytes: number;
  observedLowBytes: number;
  observedHighBytes: number;
  strata: {
    name: string;
    count: number;
    sampled: number;
    failed: number;
    sourceBytes: number;
    estimatedSavedBytes: number;
  }[];
}

export interface ConversionDiskReservation {
  sourceBytes: number;
  outputBytes: number;
  stagingBytes: number;
  backupBytes: number;
  requiredBytes: number;
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

// JPEG XL and PNG became single still-or-animated choices; earlier saved
// preferences may still name the separate animated variants.
export function choiceFormat(format: string): string {
  return format === "ajxl" ? "jxl" : format === "apng" ? "png" : format;
}

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
          ? choiceFormat(value.format)
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

export const isJXLOutput = (format: string) =>
  format === "jxl" || format === "ajxl";

export interface ConversionPlan {
  input: string;
  output: string;
  count: number;
  quality: number;
  effort: number;
  autoEffort?: boolean;
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
