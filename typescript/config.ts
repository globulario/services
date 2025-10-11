
export type TokenProvider = () => Promise<string> | string;

export type Ports = {
  http?: number;
  https?: number;
  // optional per-service overrides (e.g. { 'rbac.RbacService': 9080 })
  services?: Record<string, number>;
};

export type ServiceLocator = (serviceId: string) => string;

/**
 * Global configuration for the client kit.
 */
export interface GlobularConfig {
  protocol: 'http' | 'https';
  domain: string;             // e.g. 'example.com' (no scheme)
  ports?: Ports;
  application?: string;       // header 'application' if your backend expects it
  tokenProvider?: TokenProvider;
  /** Extra headers that should be sent with HTTP helpers and gRPC metadata */
  extraHeaders?: Record<string, string>;
  /** Optional service locator to compute endpoint by service id */
  serviceLocator?: ServiceLocator;
}

/**
 * Default config can be overridden at app bootstrap with setConfig().
 */
export const config: GlobularConfig = {
  protocol: 'https',
  domain: typeof window !== 'undefined' ? window.location.hostname : 'localhost',
  ports: { https: 443, http: 80 },
  application: 'globular-admin',
  tokenProvider: async () => {
    try {
      return localStorage.getItem('access_token') || '';
    } catch {
      return '';
    }
  },
};

export function setConfig(next: Partial<GlobularConfig>) {
  Object.assign(config, next);
  if (next.ports) {
    config.ports = Object.assign({}, config.ports, next.ports);
  }
  if (next.extraHeaders) {
    config.extraHeaders = Object.assign({}, config.extraHeaders, next.extraHeaders);
  }
  if (next.serviceLocator) {
    config.serviceLocator = next.serviceLocator;
  }
}

/**
 * Default service locator:
 * - if ports.services has an entry for the service id, use that port
 * - else use protocol default port
 * - endpoint shape: `${protocol}://${domain}:${port}`
 */
export const defaultServiceLocator: ServiceLocator = (serviceId: string) => {
  const { protocol, domain, ports } = config;
  const byService = ports?.services?.[serviceId];
  const port = byService ?? (protocol === 'https' ? ports?.https : ports?.http);
  return `${protocol}://${domain}${port ? `:${port}` : ''}`;
};
