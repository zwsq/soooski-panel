export type Cert = {
  domain: string;
  state: string;
  issuer?: string;
  not_after?: string;
  days_left?: number;
  error?: string;
};

export type Dashboard = {
  core_running: boolean;
  core_error?: string;
  users_total: number;
  users_active: number;
  users_online: number;
  traffic_up: number;
  traffic_down: number;
  traffic_error?: string;
  public_host: string;
  inbounds_on: number;
  domains: number;
  admin_url: string;
  client_path: string;
  data_dir: string;
  certs?: Cert[];
  acme_email?: string;
};

export type User = {
  id: number;
  username: string;
  sub_token: string;
  enable: boolean;
  expire_at?: string | null;
  traffic_limit: number;
  traffic_up: number;
  traffic_down: number;
  note?: string;
  telegram_secret?: string;
  created_at?: string;
  last_seen_at?: string | null;
  online?: boolean;
};

export type Link = { name: string; uri: string; tag: string; mode: string; protocol: string; xray?: boolean };

export type Inbound = {
  id: number;
  tag: string;
  protocol: string;
  transport: string;
  security: string;
  mode: string;
  listen_port: number;
  internal_port: number;
  path?: string;
  enable: boolean;
  remark: string;
};

export type Domain = { id: number; domain: string; mode: string; enable: boolean; provider: string };

export type Settings = {
  public_host: string;
  reality_public_key: string;
  reality_short_id: string;
  reality_server_name: string;
  wg_public_key: string;
  hy2_obfs: string;
  acme_email: string;
  admin_path: string;
  client_path: string;
  telegram_enabled: boolean;
  telegram_fake_domain: string;
};
