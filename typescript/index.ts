
import type { Metadata } from 'grpc-web';
import { config as _config, setConfig as _setConfig, type GlobularConfig, type ServiceLocator } from './config';
import { buildMetadata as _buildMetadata, createGrpcClient } from './transport';
export { http } from './http';
export type { GlobularConfig, ServiceLocator } from './config';

class Globular {
  get config(): GlobularConfig { return _config; }
  setConfig(next: Partial<GlobularConfig>) { _setConfig(next); }

  setServiceLocator(locator: ServiceLocator) {
    _setConfig({ serviceLocator: locator });
  }

  async metadata(extra?: Metadata) {
    return _buildMetadata(extra);
  }

  client<TClient>(
    serviceId: string,
    Ctor: new (hostname: string, credentials: null, options: { [key: string]: any }) => TClient,
    opts?: { withCredentials?: boolean; format?: 'text' | 'binary'; endpointOverride?: string; serviceLocator?: ServiceLocator; }
  ): TClient {
    return createGrpcClient(serviceId, Ctor, opts);
  }
}

export const globular = new Globular();
export const setConfig = (next: Partial<GlobularConfig>) => globular.setConfig(next);
