
import type { Metadata } from 'grpc-web';
import { config, defaultServiceLocator, type ServiceLocator } from './config';

/** Build grpc-web Metadata including Authorization, domain and application headers. */
export async function buildMetadata(extra?: Metadata): Promise<Metadata> {
  const m: Metadata = { ...(config.extraHeaders ?? {}), ...(extra ?? {}) };

  if (config.domain && !m['domain']) m['domain'] = config.domain;
  if (config.application && !m['application']) m['application'] = config.application;

  const token = config.tokenProvider ? await config.tokenProvider() : '';
  if (token && !m['authorization']) {
    m['authorization'] = token.startsWith('Bearer ') ? token : `Bearer ${token}`;
  }
  return m;
}

 /**
 * Create a typed grpc-web client given the generated constructor.
 * The constructor signature is typically (hostname: string, credentials: null, options: object)
 */
export function createGrpcClient<TClient>(
  serviceId: string,
  Ctor: new (hostname: string, credentials: null, options: { [key: string]: any }) => TClient,
  opts?: { withCredentials?: boolean; format?: 'text' | 'binary'; endpointOverride?: string; serviceLocator?: ServiceLocator; }
): TClient {
  const locator = opts?.serviceLocator || config.serviceLocator || defaultServiceLocator;
  const baseUrl = opts?.endpointOverride || locator(serviceId);

  const options = {
    withCredentials: opts?.withCredentials ?? false,
    format: opts?.format ?? 'binary',
  };
  return new Ctor(baseUrl, null, options);
}
