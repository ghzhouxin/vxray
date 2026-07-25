interface GeoSource {
  geoip: string
  geosite: string
}

export interface SpeedTestTarget {
  name: string
  url: string
  icon?: string
  latency: number
  error?: string
  tested_at?: number
}

export interface UserSettings {
  speedtest: {
    target_url: string
    timeout: number
    concurrency: number
    website_targets: SpeedTestTarget[]
  }
  geo: {
    selected_source: string
  }
}

export interface SystemMeta {
  home: string
  paths: {
    geo_dir: string
    xray_config_path: string
  }
  server: {
    host: string
    port: number
  }
  xray: {
    binary: string
  }
  assets: {
    geo_sources: Record<string, GeoSource>
  }
}
