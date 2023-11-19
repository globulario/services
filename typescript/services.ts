// Here is the list of services from the backend.
import { AuthenticationServicePromiseClient } from './authentication/authentication_grpc_web_pb';
import { EventServicePromiseClient } from './event/event_grpc_web_pb';
import { EchoServicePromiseClient } from './echo/echo_grpc_web_pb';
import { CatalogServicePromiseClient } from './catalog/catalog_grpc_web_pb';
import { FileServicePromiseClient } from './file/file_grpc_web_pb';
import { LdapServicePromiseClient } from './ldap/ldap_grpc_web_pb';
import { PersistenceServicePromiseClient } from './persistence/persistence_grpc_web_pb';
import { MailServicePromiseClient } from './mail/mail_grpc_web_pb';
import { SpcServicePromiseClient } from './spc/spc_grpc_web_pb';
import { SqlServicePromiseClient } from './sql/sql_grpc_web_pb';
import { StorageServicePromiseClient } from './storage/storage_grpc_web_pb';
import { MonitoringServicePromiseClient } from './monitoring/monitoring_grpc_web_pb';
import { SearchServicePromiseClient } from './search/search_grpc_web_pb';
import { AdminServicePromiseClient } from './admin/admin_grpc_web_pb';
import { ResourceServicePromiseClient } from './resource/resource_grpc_web_pb';
import { RbacServicePromiseClient } from './rbac/rbac_grpc_web_pb'
import { LogServicePromiseClient } from './log/log_grpc_web_pb';
import { SubscribeRequest, UnSubscribeRequest, PublishRequest, Event, OnEventRequest, SubscribeResponse } from './event/event_pb';
import { ConversationServicePromiseClient } from './conversation/conversation_grpc_web_pb';
import { BlogServicePromiseClient } from './blog/blog_grpc_web_pb';
import { ServicesManagerServicePromiseClient } from './services_manager/services_manager_grpc_web_pb';
import { ApplicationManagerServicePromiseClient } from './applications_manager/applications_manager_grpc_web_pb';
import { PackageDiscoveryPromiseClient } from './discovery/discovery_grpc_web_pb';
import { PackageRepositoryPromiseClient } from './repository/repository_grpc_web_pb';
import { TitleServicePromiseClient } from './title/title_grpc_web_pb';
import { TorrentServicePromiseClient } from './torrent/torrent_grpc_web_pb';

/**
 * The service configuration information.
 */
export interface IServiceConfig {
  Id: string;
  Name: string;
  Mac: string;
  State: string;
  Domain: string;
  Address: string;
  Port: number;
  Proxy: number;
  TLS: boolean;
  KeepUpToDate: boolean;
  KeepAlive: boolean;
  PublisherId: string;
  Version: string;
  Description: string;
  Keywords: Array<string>;
  Discoveries: Array<string>;
  Repositories: Array<string>;
  Proto: String;
  Path: string;
  CertAuthorityTrust: string;
  CertFile: string;
  KeyFile: string;
}

/**
 * Define a map of services.
 */
export interface IServices {
  [key: string]: IServiceConfig;
}


export interface IPeer {
  Domain: string;
  Address: string;
  Mac: string;
  Port: number;
}

/**
 * The application server informations.
 */
export interface IConfig {
  Name: string;
  Mac: string;
  Domain: string;
  PortHttp: number;
  PortHttps: number;
  BackendPort: number;
  BackendStore: number;
  AdminEmail: string;
  SessionTimeout: number;
  Protocol: string;
  Discoveries: string[];
  DNS: string[];
  CertExpirationDelay: number;
  CertStableURL: string;
  CertURL: string;
  IdleTimeout: number;
  IndexApplication: string;
  Root: string;
  WebRoot: string;
  Public: string[];
  AlternateDomains: string[],
  Path: string;
  DataPath: string;
  ConfigPath: string;
  Vesion: string;
  Platform: string;

  // The map of service object.
  Services: IServices;

  // The array of Peers.
  Peers: IPeer[];
}

/**
 * Create a "version 4" RFC-4122 UUID (Universal Unique Identifier) string.
 * @returns {string} A string containing the UUID.
 */
function randomUUID(): string {
  const s = new Array();
  const itoh = '0123456789abcdef'; // Make array of random hex digits. The UUID only has 32 digits in it, but we

  // allocate an extra items to make room for the '-'s we'll be inserting.
  for (let i = 0; i < 36; i++) s[i] = Math.floor(Math.random() * 0x10); // Conform to RFC-4122, section 4.4
  s[14] = 4; // Set 4 high bits of time_high field to version
  s[19] = s[19] & 0x3 | 0x8; // Specify 2 high bits of clock sequence
  // Convert to hex chars
  for (let i = 0; i < 36; i++) s[i] = itoh[s[i]]; // Insert '-'s
  s[8] = s[13] = s[18] = s[23] = '-';
  return s.join('');
}


window.onbeforeunload = function (e: any) {

  console.log("cleanup network resources...");

  return undefined;
};

/**
 * That local and distant event hub.
 */
export class EventHub {
  private globular: Globular;
  private subscribers: any;
  private uuid: string;
  private refs: any;


  /**
   * @param {*} service If undefined only local event will be allow.
   */
  constructor(g: Globular) {

    // The parent globular object.
    this.globular = g;
    // Subscriber function map.
    this.subscribers = {}
    // Keep references of object that call subscribe...
    this.refs = {}

    // This is the client uuid
    this.uuid = randomUUID();

    // This will stay there until disconnect will be call...
    this.connect(null);

    // Disconnect null object
    this.removeDeletedListeners()
  }

  // Disconnect null listeners...
  removeDeletedListeners() {
    setInterval(() => {
      for (var uuid in this.refs) {
        if (this.refs[uuid] == null) {
          this.unSubscribe(uuid.split(":")[0], uuid.split(":")[1])
        }
      }
    }, 5000)
  }

  reinitRemoteListeners() {

    let subscriptions = []
    for (const name in this.subscribers) {
      for (var uuid in this.subscribers[name]) {
        let subscription = this.subscribers[name][uuid]
        if (!subscription.local) {
          if (subscriptions.indexOf(name) == -1) {
            subscriptions.push(name)
          }
        }
      }
    }

    let subscribe = () => {
      let name = subscriptions.pop()
      const rqst = new SubscribeRequest
      rqst.setName(name)
      rqst.setUuid(this.uuid)
      this.globular.eventService.subscribe(rqst).then((rsp: SubscribeResponse) => {
        if (subscriptions.length > 0) {
          subscribe();
        }
      }).catch(err => { console.log(err) })
    }

    subscribe()
  }

  /** Connect to the remote server. */
  connect(callback: () => void) {
    // Open the connection with the server.
    if (this.globular.eventService !== undefined) {

      // The first step is to subscribe to an event channel.
      const rqst = new OnEventRequest()
      rqst.setUuid(this.uuid)

      const stream = this.globular.eventService.onEvent(rqst, {});
      let last_timeout = -1;

      // Get the stream and set event on it...
      stream.on('data', (rsp: any) => {
        if (rsp.hasEvt()) {
          const evt = rsp.getEvt()
          let bytes = evt.getData()
          let data = ""
          if (bytes != null) {
            if (bytes.length > 0) {
              data = new TextDecoder("utf-8").decode(bytes);
            }
          }

          // dispatch the event localy.
          this.dispatch(evt.getName(), data)

        } else if (rsp.hasKa()) {
          /**  */
          if (last_timeout > 0) {
            clearTimeout(last_timeout)
          }

          // wait for the next timeout...
          last_timeout = setTimeout(() => {

            stream.cancel() // cancel the stream if it was active
            this.globular.resetEventService(); // reset listeners...
            this.connect(() => {
              this.reinitRemoteListeners();
            });

          }, 25 * 1000) // if signal was received for 25 second I will try to reconnect...
        }
      });

      stream.on('status', (status: any) => {
        if (status.code != 0) {
          /** nothing here. */
        }
      });

      stream.on('end', () => {
        /** Nothing here */
      });

      if (callback != undefined) {
        callback();
      }

    }/* else {
      // Wait a second before try to connect agin...
      setTimeout(() => {
        this.connect(() => {
          this.reinitRemoteListeners()
        })

      }, 1000)
    }*/
  }


  /**
   * 
   * @param {*} name The name of the event to subcribe to. 
   * @param {*} onsubscribe That function return the uuid of the subscriber.
   * @param {*} onevent That function is call when the event is use.
   * @param local if true means the event run localy, true by default
   * @param ref The reference to the object that subscribe to this event...
   */
  subscribe(name: string, onsubscribe: (uuid: string) => any, onevent: (data: any) => any, local = true, ref = null) {

    // Register the local subscriber.
    const uuid = randomUUID()
    if (ref != null) {

      if (ref[name] != null) {
        // A subscription already exist...
        return
      }
      this.refs[uuid + ":" + name] = ref

      // keep the listner uuid...
      ref[name] = uuid

    }


    if (!local) {
      if (this.subscribers[name] == undefined) {
        this.subscribers[name] = {}
        const rqst = new SubscribeRequest
        rqst.setName(name)
        rqst.setUuid(this.uuid)
        this.globular.eventService.subscribe(rqst, {}).then((rsp: SubscribeResponse) => {
          if (this.subscribers[name] == undefined) {
            this.subscribers[name] = {} // create if it not exist.
          }
          this.subscribers[name][uuid] = { onsubscribe: onsubscribe, onevent: onevent, local: local }
          onsubscribe(uuid)
        })
      } else {
        this.subscribers[name][uuid] = { onsubscribe: onsubscribe, onevent: onevent, local: local }
        onsubscribe(uuid)
      }
    } else {
      // create a uuid and call onsubscribe callback.
      if (this.subscribers[name] === undefined) {
        this.subscribers[name] = {}
      }
      this.subscribers[name][uuid] = { onsubscribe: onsubscribe, onevent: onevent, local: local }
      onsubscribe(uuid)
    }
  }

  /**
   * 
   * @param {*} name 
   * @param {*} uuid 
   */
  unSubscribe(name: string, uuid: string) {
    if (this.subscribers[name] === undefined) {
      return
    }

    if (this.subscribers[name][uuid] === undefined) {
      return
    }

    // remove references...
    let ref = this.refs[name + ":" + uuid]
    if (ref != null) {
      delete ref[name]
      delete this.refs[name + ":" + uuid]
    }

    const subscription = this.subscribers[name][uuid]

    // Remove the local subscriber.
    delete this.subscribers[name][uuid]

    if (Object.keys(this.subscribers[name]).length === 0) {
      delete this.subscribers[name]
      // disconnect from the distant server.
      if (this.globular.eventService !== undefined && !subscription.local) {

        const rqst = new UnSubscribeRequest();
        rqst.setName(name);
        rqst.setUuid(this.uuid)

        // Now I will test with promise
        this.globular.eventService.unSubscribe(rqst, {})
          .then((resp: any) => {
          })
          .catch((error: any) => {
            console.log(error)
          })
      }
    }
  }

  /**
   * Publish an event on the bus, or locally in case of local event.
   * @param {*} name The  name of the event to publish
   * @param {*} data The data associated with the event
   * @param {*} local If the event is not local the data must be seraliaze before sent.
   */
  publish(name: string, data: any, local: boolean) {
    if (local === true) {
      this.dispatch(name, data)
    } else {

      // Create a new request.
      const rqst = new PublishRequest();
      const evt = new Event();
      evt.setName(name)

      const enc = new TextEncoder(); // always utf-8
      // encode the string to a array of byte
      if (data != null) {
        if (data.length > 0) {
          evt.setData(enc.encode(data))
        }
      }

      rqst.setEvt(evt);

      // Now I will test with promise
      this.globular.eventService.publish(rqst, {})
        .then((resp: any) => {
          /** Nothing to do here. */
        })
        .catch((error: any) => {
          console.log(error)
        })
    }
  }

  /** Dispatch the event localy */
  dispatch(name: string, data: any) {
    for (const uuid in this.subscribers[name]) {
      // call the event callback function.
      if (this.subscribers !== undefined) {
        if (this.subscribers[name] !== undefined) {
          if (this.subscribers[name][uuid] !== undefined) {
            this.subscribers[name][uuid].onevent(data);
          }
        }
      }
    }
  }

}

// Get the configuration from url
function getFileConfig(url: string, callback: (obj: any) => void, errorcallback: (err: any) => void) {

  // so I will try to get the configuration from the server...
  var xmlhttp = new XMLHttpRequest();
  xmlhttp.timeout = 1500;
  xmlhttp.onreadystatechange = function () {
    if (this.readyState == 4 && this.status == 201) {
      var obj = JSON.parse(this.responseText);
      callback(obj);
    } else if (this.readyState == 4) {
      errorcallback("fail to get the configuration file at url " + url + " status " + this.status)
    }
  };

  // Create a URL object
  var url_ = new URL(url);
  xmlhttp.open("GET", `/config?host=${url_.hostname}&port=${url_.port}`, true);
  xmlhttp.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
  xmlhttp.send();
}

/**
 * Globular regroup all serivces in one object that can be use by
 * application to get access to sql, ldap, persistence... service.
 */
export class Globular {
  private _eventHub: EventHub;
  private _services: any;

  public get eventHub(): EventHub {
    return this._eventHub;
  }

  /** The configuation. */
  constructor(url: string, callback: () => void, errorcallback: (err: any) => void) {

    // Keep the config...
    getFileConfig(url, (config: any) => {

      this.config = config;

      // force event hub initialysation...
      this._services = {};

      // here I will connect the event hub...
      this._eventHub = new EventHub(this);

      // I will subscribe on services configuration update.
      this._eventHub.subscribe("update_globular_service_configuration_evt", () => { }, (evt: any) => {
        let config = JSON.parse(evt);
        this._services[config.Id] = null
        this.config.Services[config.Id] = config;
        // console.log("service has change !", config)

      }, false, this)

      callback();
    }, errorcallback)
  }

  private _config: IConfig;
  public get config(): IConfig {
    return this._config;
  }
  public set config(value: IConfig) {
    this._config = value;
  }

  // Return the domain of a globule from it configuration.
  public get address(): string {
    let domain = this._config.Name
    if (domain.length > 0 && !this._config.Domain.startsWith(this._config.Name)) {
      if (this._config.Domain.length > 0) {
        domain += "." + this._config.Domain;
      }
    } else {
      domain = this._config.Domain;
    }
    return domain;
  }

  // Return the domain of a globule from it configuration.
  public get domain(): string {
    return this._config.Domain;
  }

  /** The admin service to access to other configurations. */
  private _adminService: AdminServicePromiseClient
  public get adminService(): AdminServicePromiseClient | undefined {
    // refresh the config.
    if (this._adminService == null) {
      let configs = this.getConfigs('admin.AdminService')
      configs.forEach((config: IServiceConfig) => {
        this._adminService = new AdminServicePromiseClient(
          this.config.Protocol +
          '://' +
          config.Address.split(":")[0] +
          ':' +
          config.Proxy,
          null,
          null,
        );
        this._services[config.Id] = this._adminService
      });
    }
    return this._adminService;
  }

  /** The services manager service */
  private _servicesManagerService: ServicesManagerServicePromiseClient
  public get servicesManagerService(): ServicesManagerServicePromiseClient | undefined {
    // refresh the config.
    if (this._servicesManagerService == null) {
      let configs = this.getConfigs('services_manager.ServicesManagerService')
      configs.forEach((config: IServiceConfig) => {
        this._servicesManagerService = new ServicesManagerServicePromiseClient(
          this.config.Protocol +
          '://' +
          config.Address.split(":")[0] +
          ':' +
          config.Proxy,
          null,
          null,
        );
        this._services[config.Id] = this._servicesManagerService
      });
    }
    return this._servicesManagerService;
  }

  /** The application manager service */
  private _applicationsManagerService: ApplicationManagerServicePromiseClient
  public get applicationsManagerService(): ApplicationManagerServicePromiseClient | undefined {
    // refresh the config.
    if (this._applicationsManagerService == null) {
      let configs = this.getConfigs('applications_manager.ApplicationManagerService')
      configs.forEach((config: IServiceConfig) => {
        this._applicationsManagerService = new ApplicationManagerServicePromiseClient(
          this.config.Protocol +
          '://' +
          config.Address.split(":")[0] +
          ':' +
          config.Proxy,
          null,
          null,
        );
        this._services[config.Id] = this._applicationsManagerService
      });
    }
    return this._applicationsManagerService;
  }

  /** The authentication services */
  private _authenticationService: AuthenticationServicePromiseClient
  public get authenticationService(): AuthenticationServicePromiseClient | undefined {
    // refresh the config.
    if (this._authenticationService == null) {
      let configs = this.getConfigs('authentication.AuthenticationService')
      configs.forEach((config: IServiceConfig) => {
        this._authenticationService = new AuthenticationServicePromiseClient(
          this.config.Protocol +
          '://' +
          config.Address.split(":")[0] +
          ':' +
          config.Proxy,
          null,
          null,
        );
        this._services[config.Id] = this._authenticationService
      });
    }
    return this._authenticationService;
  }

  /** The resource promise client */
  private _resourceService: ResourceServicePromiseClient
  public get resourceService(): ResourceServicePromiseClient | undefined {
    // refresh the config.
    if (this._resourceService == null) {
      let configs = this.getConfigs('resource.ResourceService')
      configs.forEach((config: IServiceConfig) => {
        this._resourceService = new ResourceServicePromiseClient(
          this.config.Protocol +
          '://' +
          config.Address.split(":")[0] +
          ':' +
          config.Proxy,
          null,
          null,
        );
        this._services[config.Id] = this._resourceService
      });
    }
    return this._resourceService;
  }

  private _logService: LogServicePromiseClient
  public get logService(): LogServicePromiseClient | undefined {
    // refresh the config.
    if (this._logService == null) {
      let configs = this.getConfigs('log.LogService')
      configs.forEach((config: IServiceConfig) => {
        this._logService = new LogServicePromiseClient(
          this.config.Protocol +
          '://' +
          config.Address.split(":")[0] +
          ':' +
          config.Proxy,
          null,
          null,
        );
        this._services[config.Id] = this._logService
      })
    }
    return this._logService;
  }

  private _blogService: BlogServicePromiseClient
  public get blogService(): BlogServicePromiseClient | undefined {
    // refresh the config.
    if (this._blogService == null) {
      let configs = this.getConfigs('blog.BlogService')
      configs.forEach((config: IServiceConfig) => {
        this._blogService = new BlogServicePromiseClient(
          this.config.Protocol +
          '://' +
          config.Address.split(":")[0] +
          ':' +
          config.Proxy,
          null,
          null,
        );
        this._services[config.Id] = this._blogService
      })
    }
    return this._blogService;
  }

  private _conversationService: ConversationServicePromiseClient
  public get conversationService(): ConversationServicePromiseClient | undefined {
    // refresh the config.
    if (this._conversationService == null) {
      let configs = this.getConfigs('conversation.ConversationService')
      
      configs.forEach((config: IServiceConfig) => {
        this._conversationService = new ConversationServicePromiseClient(
          this.config.Protocol +
          '://' +
          config.Address.split(":")[0] +
          ':' +
          config.Proxy,
          null,
          null,
        );
        this._services[config.Id] = this._conversationService
      });
    }
    return this._conversationService;
  }

  private _rbacService: RbacServicePromiseClient
  public get rbacService(): RbacServicePromiseClient | undefined {
    // refresh the config.
    if (this._rbacService == null) {
      let configs = this.getConfigs('rbac.RbacService')
      configs.forEach((config: IServiceConfig) => {
        this._rbacService = new RbacServicePromiseClient(
          this.config.Protocol +
          '://' +
          config.Address.split(":")[0] +
          ':' +
          config.Proxy,
          null,
          null,
        );
        this._services[config.Id] = this._rbacService
      });
    }
    return this._rbacService;
  }

  private _packagesDicovery: PackageDiscoveryPromiseClient
  public get packagesDicovery(): PackageDiscoveryPromiseClient | undefined {
    // refresh the config.
    if (this._packagesDicovery == null) {
      let configs = this.getConfigs('discovery.PackageDiscovery')
      configs.forEach((config: IServiceConfig) => {
        this._packagesDicovery = new PackageDiscoveryPromiseClient(
          this.config.Protocol +
          '://' +
          config.Address.split(":")[0] +
          ':' +
          config.Proxy,
          null,
          null,
        );
        this._services[config.Id] = this._packagesDicovery
      });
    }
    return this._packagesDicovery;
  }

  private _servicesRepository: PackageRepositoryPromiseClient
  public get servicesRepository(): PackageRepositoryPromiseClient | undefined {
    // refresh the config.
    if (this._servicesRepository == null) {
      let configs = this.getConfigs('repository.PackageRepository')
      configs.forEach((config: IServiceConfig) => {
        this._servicesRepository = new PackageRepositoryPromiseClient(
          this.config.Protocol +
          '://' +
          config.Address.split(":")[0] +
          ':' +
          config.Proxy,
          null,
          null,
        );
        this._services[config.Id] = this._servicesRepository
      });
    }
    return this._servicesRepository;
  }

  // list of services.
  private _catalogService: CatalogServicePromiseClient
  public get catalogService(): CatalogServicePromiseClient | undefined {
    if (this._catalogService == null) {
      let configs = this.getConfigs('catalog.CatalogService')
      configs.forEach((config: IServiceConfig) => {
        if (this._catalogService == null) {
          this._catalogService = new CatalogServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._catalogService
        }
      });
    }
    return this._catalogService
  }

  private _echoService: EchoServicePromiseClient
  public get echoService(): EchoServicePromiseClient | undefined {
    if (this._echoService == null) {
      let configs = this.getConfigs('echo.EchoService')
      configs.forEach((config: IServiceConfig) => {
        if (this._echoService == null) {
          this._echoService = new EchoServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._echoService
        }
      });
    }
    return this._echoService;
  }

  private _eventService: EventServicePromiseClient
  public get eventService(): EventServicePromiseClient | undefined {
    if (this._eventService == null) {
      let configs = this.getConfigs('event.EventService')
      configs.forEach((config: IServiceConfig) => {
        if (this._eventService == null) {
          this._eventService = new EventServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._eventService
        }
      });
    }
    return this._eventService
  }

  public resetEventService() {
    this._eventService = undefined;
  }

  private _fileService: FileServicePromiseClient
  public get fileService(): FileServicePromiseClient | undefined {
    if (this._fileService == null) {
      let configs = this.getConfigs('file.FileService')
      configs.forEach((config: IServiceConfig) => {
        if (this._fileService == null) {
          this._fileService = new FileServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._fileService
        }

      })
    }
    return this._fileService;
  }

  private _ldapService: LdapServicePromiseClient
  public get ldapService(): LdapServicePromiseClient | undefined {
    if (this._ldapService == null) {
      let configs = this.getConfigs('ldap.LdapService')
      configs.forEach((config: IServiceConfig) => {
        if (this._ldapService == null) {
          this._ldapService = new LdapServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._ldapService
        }
      })
    }
    return this._ldapService
  }

  private _persistenceService: PersistenceServicePromiseClient
  public get persistenceService(): PersistenceServicePromiseClient | undefined {
    if (this._persistenceService == null) {
      let configs = this.getConfigs('persistence.PersistenceService')
      configs.forEach((config: IServiceConfig) => {
        if (this._persistenceService == null) {
          this._persistenceService = new PersistenceServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._persistenceService
        }
      });
    }
    return this._persistenceService
  }

  private _mailService: MailServicePromiseClient
  public get mailService(): MailServicePromiseClient | undefined {
    if (this._mailService == null) {
      let configs = this.getConfigs('mail.MailService')
      configs.forEach((config: IServiceConfig) => {
        if (this._mailService == null) {
          this._mailService = new MailServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._mailService
        }

      });
    }
    return this._mailService;
  }

  private _sqlService: SqlServicePromiseClient
  public get sqlService(): SqlServicePromiseClient | undefined {
    if (this._sqlService == null) {
      let configs = this.getConfigs('sql.SqlService')
      configs.forEach((config: IServiceConfig) => {
        if (this._sqlService == null) {
          this._sqlService = new SqlServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._sqlService
        }
      });
    }
    return this._sqlService
  }

  private _storageService: StorageServicePromiseClient
  public get storageService(): StorageServicePromiseClient | undefined {
    if (this._storageService == null) {
      let configs = this.getConfigs('storage.StorageService')
      configs.forEach((config: IServiceConfig) => {
        if (this._storageService == null) {
          this._storageService = new StorageServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._storageService
        }
      });
    }
    return this._storageService;
  }

  private _monitoringService: MonitoringServicePromiseClient
  public get monitoringService(): MonitoringServicePromiseClient | undefined {
    if (this._monitoringService == null) {

      let configs = this.getConfigs('monitoring.MonitoringService')
      configs.forEach((config: IServiceConfig) => {
        if (this._monitoringService == null) {
          this._monitoringService = new MonitoringServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._monitoringService
        }
      })
    }
    return this._monitoringService
  }

  private _spcService: SpcServicePromiseClient
  public get spcService(): SpcServicePromiseClient | undefined {
    if (this._spcService == null) {
      let configs = this.getConfigs('spc.SpcService')
      configs.forEach((config: IServiceConfig) => {
        if (this._spcService == null) {
          this._spcService = new SpcServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._spcService
        }
      })
    }
    return this._spcService
  }
  private _titleService: TitleServicePromiseClient
  public get titleService(): TitleServicePromiseClient | undefined {
    if (this._titleService == null) {
      let configs = this.getConfigs('title.TitleService')
      configs.forEach((config: IServiceConfig) => {
        if (this._titleService == null) {
          this._titleService = new TitleServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._titleService
        }
      })
    }
    return this._titleService
  }
  private _torrentService: TorrentServicePromiseClient
  public get torrentService(): TorrentServicePromiseClient | undefined {
    if (this._torrentService == null) {
      let configs = this.getConfigs('torrent.TorrentService')
      configs.forEach((config: IServiceConfig) => {
        if (this._torrentService == null) {
          this._torrentService = new TorrentServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._torrentService
        }
      })
    }
    return this._torrentService
  }
  private _searchService: SearchServicePromiseClient
  public get searchService(): SearchServicePromiseClient | undefined {
    if (this._searchService == null) {
      let configs = this.getConfigs('search.SearchService')
      configs.forEach((config: IServiceConfig) => {
        if (this._searchService == null) {
          this._searchService = new SearchServicePromiseClient(
            this.config.Protocol +
            '://' +
            config.Address.split(":")[0] +
            ':' +
            config.Proxy,
            null,
            null,
          );
          this._services[config.Id] = this._searchService
        }
      })
    }
    return this._searchService
  }

  /**
   * Return a service instance
   * @param name The name of the service.
   * @param id The id of the service.
   * 
   * as example to get a specific instance of the file services.
   *  globular.getServiceById("file.fileService", "applications")
   */
  getServiceById(name: string, id: string): any {
    for (const id_ in this.config.Services) {
      const service = this.config.Services[id]
      if (service.Name == name && id_ == id) {
        return service;
      }
    }
  }

  // Return the first configuration that match the given name.
  // The load balancer will be in charge to select the correct service instance from the list
  // The first instance is the entry point of the services.
  getConfigs(name: string): Array<IServiceConfig> {
    let services = new Array<IServiceConfig>();
    for (const id in this.config.Services) {
      const service = this.config.Services[id]
      if (service.Name == name) {
        services.push(service);
      }
    }
    return services;
  }
}

