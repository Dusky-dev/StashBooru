export const conversionEndpoint = "image/converter";

export interface ConversionFormat {
  id: string;
  label: string;
  extension: string;
  family: string;
  cpu: string[];
  gpu: string[];
  controls: string[];
  available: boolean;
}

export interface ConversionConfig {
  cacheLimitBytes: number;
  formatDefaults: Record<string, string>;
  encodingDefaults: Record<string, { quality: number; effort: number }>;
  backend: string;
}

export interface ConversionUpscaler {
  id: string;
  label: string;
  available: boolean;
  cpu: boolean;
  notice: string;
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
  error?: string;
}

export async function conversionResponse<T>(r: Response): Promise<T> {
  if (!r.ok) throw new Error((await r.text()) || r.statusText);
  return r.json() as Promise<T>;
}
