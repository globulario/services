/**
 * That file contain a list of function that can be use as API instead of
 * using the globular object itself.
 */

import {
  GetConfigRequest,
  SaveConfigRequest,
  InstallServiceRequest,
  InstallServiceResponse,
  StopServiceRequest,
  StartServiceRequest,
  SaveConfigResponse,
  SetEmailRequest,
  SetPasswordRequest,
  UninstallServiceRequest,
  UninstallServiceResponse,
  HasRunningProcessRequest,
  HasRunningProcessResponse,
} from "./admin/admin_pb";

import { QueryRangeRequest, QueryRequest } from "./monitoring/monitoring_pb";

import {
  RegisterAccountRqst,
  AuthenticateRqst,
  Account,
  GetAllActionsRqst,
  GetAllActionsRsp,
  AddRoleActionsRqst,
  AddRoleActionsRsp,
  RemoveRoleActionRqst,
  CreateRoleRqst,
  Role,
  CreateRoleRsp,
  DeleteRoleRqst,
  RefreshTokenRqst,
  RefreshTokenRsp,
  GetAllApplicationsInfoRqst,
  GetAllApplicationsInfoRsp,
  AddApplicationActionsRqst,
  AddApplicationActionsRsp,
  RemoveApplicationActionRqst,
  DeleteApplicationRqst,
  DeleteAccountRqst,
  AddAccountRoleRqst,
  RemoveAccountRoleRqst,
  SynchronizeLdapRqst,
  LdapSyncInfos,
  SynchronizeLdapRsp,
  UserSyncInfos,
  GroupSyncInfos,
  GetPeersRqst,
  GetPeersRsp,
  Peer,
  AddPeerActionsRqst,
  AddPeerActionsRsp,
  RemovePeerActionRqst,
  DeletePeerRqst,
  RegisterPeerRqst,
  RemoveAccountRoleRsp,
  RemoveApplicationActionRsp,
  DeleteApplicationRsp,
  RemovePeerActionRsp,
  DeletePeerRsp,
} from "./resource/resource_pb";

import {
  GetLogRqst,
  LogInfo,
  ClearAllLogRqst,
  LogLevel,
  DeleteLogRqst
} from "./log/log_pb"

import * as jwt from "jwt-decode";
import {
  InsertOneRqst,
  FindOneRqst,
  FindRqst,
  FindResp,
  FindOneResp,
  AggregateRqst,
  PingConnectionRqst,
  PingConnectionRsp,
  ReplaceOneRqst,
  ReplaceOneRsp,
} from "./persistence/persistence_pb";

import {
  FindPackagesDescriptorRequest,
  FindPackagesDescriptorResponse,
  PackageDescriptor,
  GetPackageDescriptorRequest,
  GetPackageDescriptorResponse,
  GetPackagesDescriptorRequest,
  GetPackagesDescriptorResponse,
  SetPackageDescriptorRequest,
} from "./packages/packages_pb";

import {
  RenameRequest,
  RenameResponse,
  DeleteFileRequest,
  DeleteDirRequest,
  CreateArchiveRequest,
  CreateArchiveResponse,
  CreateDirRequest,
  ReadDirRequest,
} from "./file/file_pb";

import { Globular, EventHub } from "./services";
import { IConfig, IServiceConfig } from "./services";
import {
  SearchDocumentsRequest,
  SearchDocumentsResponse,
  SearchResult,
} from "./search/search_pb";

// Here I will get the authentication information.
const domain = window.location.hostname;
const application = window.location.pathname.split("/").join("");

function mergeTypedArrays(a: any, b: any) {
  // Checks for truthy values on both arrays
  if (!a && !b) throw 'Please specify valid arguments for parameters a and b.';

  // Checks for truthy values or empty arrays on each argument
  // to avoid the unnecessary construction of a new array and
  // the type comparison
  if (!b || b.length === 0) return a;
  if (!a || a.length === 0) return b;

  // Make sure that both typed arrays are of the same type
  if (Object.prototype.toString.call(a) !== Object.prototype.toString.call(b))
    throw 'The types of the two arguments passed for parameters a and b do not match.';

  var c = new a.constructor(a.length + b.length);
  c.set(a);
  c.set(b, a.length);

  return c;
}

function uint8arrayToStringMethod(myUint8Arr) {
  return String.fromCharCode.apply(null, myUint8Arr);
}

function getToken(): string {
  let token = localStorage.getItem("user_token");
  if (token == null) {
    return "";
  }
  return token;
}

/**
 * Display the error message to the end user.
 * @param err The error message can be a simple string or a json stringnify object
 */
export function getErrorMessage(err: any): string {
  try {
    let errObj = JSON.parse(err);
    if (errObj.ErrorMsg != undefined) {
      console.log(errObj);
      return errObj.ErrorMsg;
    }
  } catch {
    console.log(err);
    return err;
  }
}

/**
 *
 * @param globular
 * @param name
 * @param callback
 */
export function hasRuningProcess(
  globular: Globular,
  name: string,
  callback: (result: boolean) => void
) {
  let rqst = new HasRunningProcessRequest();
  rqst.setName(name);

  globular.adminService
    .hasRunningProcess(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: HasRunningProcessResponse) => {
      callback(rsp.getResult());
    })
    .catch((err: any) => {
      console.log(err);
      callback(false);
    });
}

/**
 * Return the globular configuration file. The return config object
 * can contain sensible information so it must be called with appropriate
 * level of permission.
 * @param globular
 * @param application
 * @param domain
 * @param callback
 * @param errorCallback
 */
export function readFullConfig(
  globular: Globular,
  callback: (config: any) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new GetConfigRequest();
  if (globular.adminService !== undefined) {
    globular.adminService
      .getFullConfig(rqst, {
        token: getToken(),
        application: application.length > 0 ? application : globular.config.IndexApplication,
        domain: domain,
      })
      .then((rsp) => {
        let config = rsp.getResult(); // set the globular config with the full config.
        callback(config);
      })
      .catch((err) => {
        errorCallback(err);
      });
  }
}

/**
 * Save a configuration
 * @param globular
 * @param application
 * @param domain
 * @param config The configuration to be save.
 * @param callback
 * @param errorCallback
 */
export function saveConfig(
  globular: Globular,
  config: IConfig,
  callback: (config: IConfig) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new SaveConfigRequest();
  rqst.setConfig(JSON.stringify(config));
  if (globular.adminService !== undefined) {
    globular.adminService
      .saveConfig(rqst, {
        token: getToken(),
        application: application.length > 0 ? application : globular.config.IndexApplication,
        domain: domain,
      })
      .then((rsp) => {
        config = JSON.parse(rsp.getResult());
        callback(config);
      })
      .catch((err) => {
        errorCallback(err);
      });
  }
}

/**
 * Test if a process with a given name is running on the server.
 * @param globular
 * @param name
 * @param callback
 */
function hasRunningProcess(
  globular: Globular,
  name: string,
  callback: (result: boolean) => void
) {
  let rqst: HasRunningProcessRequest;
  rqst.setName(name);

  globular.adminService
    .hasRunningProcess(rqst)
    .then((rsp: HasRunningProcessResponse) => {
      callback(rsp.getResult());
    })
    .catch((err: any) => {
      console.log(err);
      callback(false);
    });
}

///////////////////////////////////// Resource & Permissions operations /////////////////////////////////

/**
 * Set/create action permission.
 * @param globular
 * @param application
 * @param domain
 * @param action
 * @param permission
 * @param callback
 * @param errorCallback
 */
export function setActionPermission(
  globular: Globular,
  action: string,
  permission: number,
  callback: (results: any) => void,
  errorCallback: (err: any) => void
) {
  /*
    const rqst = new SetActionPermissionRqst
    rqst.setAction(action)
    rqst.setPermission(permission)

    // Call set action permission.
    globular.resourceService.setActionPermission(rqst, {
        "token": token,
        "application": application, "domain": domain
    }).then(callback)
        .catch((err: any) => {
            errorCallback(err)
        })*/
}



/**
 * The permission can be assigned to
 * a User, a Role or an Application.
 */
export enum OwnerType {
  User = 1,
  Role = 2,
  Application = 3,
}


///////////////////////////////////// File operations /////////////////////////////////

/** Upload list of file on the server. 
 * 
     ex.
    let files = new Array<File>()
    for(var key in actions_map){
        let actions = actions_map[key]
        let f = new File([JSON.stringify(actions)], key, {type: 'text/plain'})
        files.push(f)
    };
    uploadFiles("/globular_tests", files);
*/
export function uploadFiles(path: string, files: File[], callback: () => void) {
  const fd = new FormData();

  // add all selected files
  for (var i = 0; i < files.length; i++) {
    let file = files[i];
    fd.append("multiplefiles", file, file.name);
    fd.append("path", path);
  }

  // create the request
  const xhr = new XMLHttpRequest();

  xhr.onload = () => {
    if (xhr.status >= 200 && xhr.status < 300) {
      // we done! I will use the rename file event to refresh the directory...
      if (callback != null) {
        callback();
      }
    }
  };

  // path to server would be where you'd normally post the form to
  xhr.open("POST", "/uploads", true);
  xhr.setRequestHeader("token", getToken());
  xhr.setRequestHeader("application", application);
  xhr.setRequestHeader("domain", domain);
  xhr.setRequestHeader("path", path);
  xhr.onerror = (err: any) => { };

  xhr.onreadystatechange = () => {
    if (xhr.readyState === 4) {
      //if complete
      if (xhr.status === 200) {
        //check if "OK" (200)
        //success
      } else {
        console.log("Permission denied to upload file " + path);
      }
    }
  };
  xhr.send(fd);
}

/**
 * Rename a file or a directorie with given name.
 * @param path The path inside webroot
 * @param newName The new file name
 * @param oldName  The old file name
 * @param callback  The success callback.
 * @param errorCallback The error callback.
 */
export function renameFile(
  globular: Globular,
  path: string,
  newName: string,
  oldName: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new RenameRequest();
  path = path.replace("/webroot", ""); // remove the /webroot part.
  if (path.length === 0) {
    path = "/";
  }
  rqst.setPath(path);
  rqst.setOldName(oldName);
  rqst.setNewName(newName);

  globular.fileService
    .rename(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
      path: path + "/" + oldName,
    })
    .then((rsp: RenameResponse) => {
      callback();
    })
    .catch((error) => {
      if (errorCallback !== undefined) {
        errorCallback(error);
      }
    });
}

/**
 * Delete a file with a given path.
 * @param path The path of the file to be deleted.
 * @param callback The success callback.
 * @param errorCallback The error callback.
 */
export function deleteFile(
  globular: Globular,
  path: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new DeleteFileRequest();
  path = path.replace("/webroot", ""); // remove the /webroot part.
  if (path.length === 0) {
    path = "/";
  }
  rqst.setPath(path);

  globular.fileService
    .deleteFile(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
      path: path,
    })
    .then((rsp: RenameResponse) => {
      callback();
    })
    .catch((error) => {
      if (errorCallback !== undefined) {
        errorCallback(error);
      }
    });
}

/**
 * Remove a given directory and all element it contain.
 * @param path The path of the directory to be deleted.
 * @param callback The success callback
 * @param errorCallback The error callback.
 */
export function deleteDir(
  globular: Globular,
  path: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new DeleteDirRequest();
  path = path.replace("/webroot", ""); // remove the /webroot part.
  if (path.length === 0) {
    path = "/";
  }
  rqst.setPath(path);
  globular.fileService
    .deleteDir(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
      path: path,
    })
    .then((rsp: RenameResponse) => {
      callback();
    })
    .catch((error) => {
      if (errorCallback !== undefined) {
        errorCallback(error);
      }
    });
}

/**
 * Create a dir archive.
 * @param path
 * @param name
 * @param callback
 * @param errorCallback
 */
export function createArchive(
  globular: Globular,
  paths: Array<string>,
  name: string,
  callback: (path: string) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new CreateArchiveRequest();
  rqst.setPathsList(paths);
  rqst.setName(name);

  globular.fileService
    .createAchive(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: CreateArchiveResponse) => {
      callback(rsp.getResult());
    })
    .catch((error) => {
      if (errorCallback !== undefined) {
        errorCallback(error);
      }
    });
}

/**
 * Download a file from the server.
 * @param urlToSend
 */
export function downloadFileHttp(
  urlToSend: string,
  fileName: string,
  callback: () => void
) {
  const req = new XMLHttpRequest();
  req.open("GET", urlToSend, true);

  // Set the token to manage downlaod access.
  req.setRequestHeader("token", getToken());
  req.setRequestHeader("application", application);
  req.setRequestHeader("domain", domain);

  req.responseType = "blob";
  req.onload = (event) => {
    const blob = req.response;
    const link = document.createElement("a");
    link.href = window.URL.createObjectURL(blob);
    link.download = fileName;
    link.click();
    callback();
  };

  req.send();
}

/**
 * Download a directory as archive file. (.tar.gz)
 * @param path The path of the directory to dowload.
 * @param callback The success callback.
 * @param errorCallback The error callback.
 */
export function downloadDir(
  globular: Globular,
  path: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const name = path.split("/")[path.split("/").length - 1];
  path = path.replace("/webroot", ""); // remove the /webroot part.

  // Create an archive-> download it-> delete it...
  createArchive(
    globular,
    [path],
    name,
    (_path: string) => {
      // display the archive path...
      downloadFileHttp(window.location.origin + _path, name, () => {
        // Here the file was downloaded I will now delete it.
        setTimeout(() => {
          // wait a little and remove the archive from the server.
          const rqst = new DeleteFileRequest();
          rqst.setPath(path + "/" + name);
          globular.fileService
            .deleteFile(rqst, {
              token: getToken(),
              application: application.length > 0 ? application : globular.config.IndexApplication,
              domain: domain,
              path: path,
            })
            .then(callback)
            .catch(errorCallback);
        }, 5000); // wait 5 second, arbritary...
      });
    },
    errorCallback
  );
}

// Merge tow array together.
function mergeTypedArraysUnsafe(a: any, b: any) {
  const c = new a.constructor(a.length + b.length);
  c.set(a);
  c.set(b, a.length);
  return c;
}

/**
 * Read the content of a dir from a given path.
 * @param path The parent path of the dir to be read.
 * @param callback  Return the path of the dir with more information.
 * @param errorCallback Return a error if the file those not contain the value.
 */
export function readDir(
  globular: Globular,
  path: string,
  callback: (dir: any) => void,
  errorCallback: (err: any) => void
) {
  path = path.replace("/webroot", ""); // remove the /webroot part.
  if (path.length === 0) {
    path = "/";
  }

  const rqst = new ReadDirRequest();
  rqst.setPath(path);
  rqst.setRecursive(true);
  rqst.setThumnailheight(256);
  rqst.setThumnailwidth(256);

  let uint8array = new Uint8Array(0);

  const stream = globular.fileService.readDir(rqst, {
    token: getToken(),
    application: application.length > 0 ? application : globular.config.IndexApplication,
    domain: domain,
    path: path,
  });

  stream.on("data", (rsp) => {
    uint8array = mergeTypedArraysUnsafe(uint8array, rsp.getData());
  });

  stream.on("status", (status) => {
    if (status.code === 0) {
      const content = JSON.parse(new TextDecoder("utf-8").decode(uint8array));
      callback(content);
    } else {
      // error here...
      errorCallback({ message: status.details });
    }
  });
}

/**
 * Test if a file is contain in a list of files.
 * @param files
 */
function fileExist(fileName: string, files: any[]): boolean {
  if (files != null) {
    for (const file of files) {
      if (file.Name === fileName) {
        return true;
      }
    }
  }
  return false;
}

/**
 * Create a new directory inside existing one.
 * @param path The path of the directory
 * @param callback The callback
 * @param errorCallback The error callback
 */
export function createDir(
  globular: Globular,
  path: string,
  callback: (dirName: string) => void,
  errorCallback: (err: any) => void
) {
  path = path.replace("/webroot", ""); // remove the /webroot part.
  if (path.length === 0) {
    path = "/";
  }

  // first of all I will read the directory content...
  readDir(
    globular,
    path,
    (dir: any) => {
      let newDirName = "New Folder";
      for (let i = 0; i < 1024; i++) {
        if (!fileExist(newDirName, dir.Files)) {
          break;
        }
        newDirName = "New Folder (" + i + ")";
      }

      // Set the request.
      const rqst = new CreateDirRequest();
      rqst.setPath(path);
      rqst.setName(newDirName);

      // Create a directory at the given path.
      globular.fileService
        .createDir(rqst, {
          token: getToken(),
          application: application.length > 0 ? application : globular.config.IndexApplication,
          domain: domain,
          path: path,
        })
        .then(() => {
          // The new directory was created.
          callback(newDirName);
        })
        .catch((err: any) => {
          errorCallback(err);
        });
    },
    errorCallback
  );
}

///////////////////////////////////// Time series Query //////////////////////////////////////

/**
 * Run a query over a time series database.
 * @param globular
 * @param application
 * @param domain
 * @param connectionId
 * @param query
 * @param ts
 * @param callback
 * @param errorCallback
 */
export function queryTs(
  globular: Globular,
  connectionId: string,
  query: string,
  ts: number,
  callback: (value: any) => void,
  errorCallback: (error: any) => void
) {
  // Create a new request.
  const rqst = new QueryRequest();
  rqst.setConnectionid(connectionId);
  rqst.setQuery(query);
  rqst.setTs(ts);

  // Now I will test with promise
  globular.monitoringService
    .query(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((resp) => {
      if (callback !== undefined) {
        callback(JSON.parse(resp.getValue()));
      }
    })
    .catch((error) => {
      if (errorCallback !== undefined) {
        errorCallback(error);
      }
    });
}

/**
 * Run query over a time series
 * @param globular
 * @param application
 * @param domain
 * @param connectionId
 * @param query
 * @param startTime
 * @param endTime
 * @param step
 * @param callback
 * @param errorCallback
 */
export function queryTsRange(
  globular: Globular,
  connectionId: string,
  query: string,
  startTime: number,
  endTime: number,
  step: number,
  callback: (values: any) => void,
  errorCallback: (err: any) => void
) {
  // Create a new request.
  const rqst = new QueryRangeRequest();
  rqst.setConnectionid(connectionId);
  rqst.setQuery(query);
  rqst.setStarttime(startTime);
  rqst.setEndtime(endTime);
  rqst.setStep(step);

  const buffer = { value: "", warning: "" };

  const stream = globular.monitoringService.queryRange(rqst, {
    token: getToken(),
    application: application.length > 0 ? application : globular.config.IndexApplication,
    domain: domain,
  });

  stream.on("data", (rsp) => {
    buffer.value += rsp.getValue();
    buffer.warning = rsp.getWarnings();
  });

  stream.on("status", (status) => {
    if (status.code === 0) {
      callback(JSON.parse(buffer.value));
    } else {
      errorCallback({ message: status.details });
    }
  });

  stream.on("end", () => {
    // stream end signal
  });
}

///////////////////////////////////// Account management action //////////////////////////////////////

/**
 * Return the list of all account on the server, guest and admin are new account...
 * @param globular
 * @param application
 * @param domain
 * @param callback
 * @param errorCallback
 */
export function getAllAccountsInfo(
  globular: Globular,
  callback: (accounts: any[]) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new FindRqst();
  rqst.setCollection("Accounts");
  rqst.setDatabase("local_resource");
  rqst.setId("local_resource");
  rqst.setQuery("{}"); // means all values.

  const stream = globular.persistenceService.find(rqst, {
    application: application.length > 0 ? application : globular.config.IndexApplication,
    domain: domain,
  });

  let data = [];

  stream.on("data", (rsp: FindResp) => {
    data = mergeTypedArrays(data, rsp.getData())
  });

  stream.on("status", (status) => {
    if (status.code === 0) {
      let accounts = JSON.parse(uint8arrayToStringMethod(data));
      callback(accounts);
    } else {
      errorCallback({ message: status.details });
    }
  });
}

/**
 * Register a new account.
 * @param userName The name of the account
 * @param email The email
 * @param password The password
 * @param confirmPassword
 * @param callback
 * @param errorCallback
 */
export function registerAccount(
  globular: Globular,
  userName: string,
  email: string,
  password: string,
  confirmPassword: string,
  callback: (value: any) => void,
  errorCallback: (err: any) => void
) {
  const request = new RegisterAccountRqst();
  const account = new Account();
  account.setPassword(password);
  account.setName(userName);
  account.setEmail(email);
  request.setAccount(account);
  request.setConfirmPassword(confirmPassword);

  // Create the user account.
  globular.resourceService
    .registerAccount(request, {
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp) => {
      callback(rsp.getResult());
    })
    .catch((err) => {
      errorCallback(err);
    });
}

/**
 * Remove an account from the server.
 * @param name  The _id of the account.
 * @param callback The callback when the action succed
 * @param errorCallback The error callback.
 */
export function deleteAccount(
  globular: Globular,
  id: string,
  callback: (value: any) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new DeleteAccountRqst();
  rqst.setId(id);

  // Remove the account from the database.
  globular.resourceService
    .deleteAccount(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp) => {
      callback(rsp.getResult());
    })
    .catch((err) => {
      errorCallback(err);
    });
}

/**
 * Remove a role from an account.
 * @param accountId The account id
 * @param roleId The role name (id)
 * @param callback The success callback
 * @param errorCallback The error callback
 */
export function removeRoleFromAccount(
  globular: Globular,
  accountId: string,
  roleId: string,
  callback: (value: any) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new RemoveAccountRoleRqst();
  rqst.setAccountid(accountId);
  rqst.setRoleid(roleId);

  globular.resourceService
    .removeAccountRole(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp) => {
      callback(rsp.getResult());
    })
    .catch((err) => {
      errorCallback(err);
    });
}

/**
 * Append a role to an account.
 * @param accountId The account id
 * @param roleId The role name (id)
 * @param callback The success callback
 * @param errorCallback The error callback.
 */
export function appendRoleToAccount(
  globular: Globular,
  accountId: string,
  roleId: string,
  callback: (value: any) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new AddAccountRoleRqst();
  rqst.setAccountid(accountId);
  rqst.setRoleid(roleId);

  globular.resourceService
    .addAccountRole(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp) => {
      callback(rsp.getResult());
    })
    .catch((err) => {
      errorCallback(err);
    });
}

/**
 * Update the account email
 * @param accountId The account id
 * @param old_email the old email
 * @param new_email the new email
 * @param callback  the callback when success
 * @param errorCallback the error callback in case of error
 */
export function updateAccountEmail(
  globular: Globular,
  accountId: string,
  oldEmail: string,
  newEmail: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new SetEmailRequest();
  rqst.setAccountid(accountId);
  rqst.setOldemail(oldEmail);
  rqst.setNewemail(newEmail);

  globular.adminService
    .setEmail(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp) => {
      callback();
    })
    .catch((err) => {
      errorCallback(err);
    });
}

/**
 * The update account password
 * @param accountId The account id
 * @param old_password The old password
 * @param new_password The new password
 * @param confirm_password The new password confirmation
 * @param callback The success callback
 * @param errorCallback The error callback.
 */
export function updateAccountPassword(
  globular: Globular,
  accountId: string,
  oldPassword: string,
  newPassword: string,
  confirmPassword: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new SetPasswordRequest();
  rqst.setAccountid(accountId);
  rqst.setOldpassword(oldPassword);
  rqst.setNewpassword(newPassword);

  if (confirmPassword !== newPassword) {
    errorCallback("password not match!");
    return;
  }

  globular.adminService
    .setPassword(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp) => {
      callback();
    })
    .catch((error) => {
      if (errorCallback !== undefined) {
        errorCallback(error);
      }
    });
}

/**
 * Authenticate the user and get the token
 * @param globular
 * @param eventHub
 * @param application
 * @param domain
 * @param userName
 * @param password
 * @param callback
 * @param errorCallback
 */
export function authenticate(
  globular: Globular,
  eventHub: EventHub,
  userName: string,
  password: string,
  callback: (value: any) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new AuthenticateRqst();
  rqst.setName(userName);
  rqst.setPassword(password);

  // Create the user account.
  globular.resourceService
    .authenticate(rqst, { application: application.length > 0 ? application : globular.config.IndexApplication, domain: domain })
    .then((rsp) => {
      // Here I will set the token in the localstorage.
      let token = rsp.getToken();
      const decoded = jwt(token);

      // here I will save the user token and user_name in the local storage.
      localStorage.setItem("user_token", token);
      localStorage.setItem("user_name", decoded.username);

      readFullConfig(
        globular,
        (config: any) => {
          // Publish local login event.
          eventHub.publish("onlogin", config, true); // return the full config...
          callback(decoded);
        },
        (err: any) => {
          errorCallback(err);
        }
      );
    })
    .catch((err) => {
      errorCallback(err);
    });
}

/**
 * Function to be use to refresh token.
 * @param globular
 * @param eventHub
 * @param application
 * @param domain
 * @param callback
 * @param errorCallback
 */
export function refreshToken(
  globular: Globular,
  eventHub: EventHub,
  callback: (token: any) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new RefreshTokenRqst();
  rqst.setToken(localStorage.getItem("user_token"));

  globular.resourceService
    .refreshToken(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: RefreshTokenRsp) => {
      // Here I will set the token in the localstorage.
      let token = rsp.getToken();
      const decoded = jwt(token);

      // here I will save the user token and user_name in the local storage.
      localStorage.setItem("user_token", token);
      localStorage.setItem("user_name", decoded.username);

      // Publish local login event.
      eventHub.publish("onlogin", globular.config, true); // return the full config...

      callback(decoded);
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Save user data into the user_data collection.
 * @param globular
 * @param application
 * @param domain
 * @param data
 * @param callback
 * @param errorCallback
 */
export function appendUserData(
  globular: Globular,
  data: any,
  callback: (id: string) => void,
  errorCallback: (err: any) => void
) {
  const userName = localStorage.getItem("user_name");
  const database = userName + "_db";
  const collection = "user_data";

  const rqst = new InsertOneRqst();
  rqst.setId(database);
  rqst.setDatabase(database);
  rqst.setCollection(collection);
  rqst.setData(JSON.stringify(data));
  rqst.setOptions("");

  // call persist data
  globular.persistenceService
    .insertOne(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: any) => {
      callback(rsp.getId());
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Read user data one result at time.
 * @param globular
 * @param application
 * @param domain
 * @param query
 * @param callback
 * @param errorCallback
 */
export function readOneUserData(
  globular: Globular,
  query: string,
  callback: (results: any) => void,
  errorCallback: (err: any) => void
) {
  const userName = localStorage.getItem("user_name");
  const database = userName + "_db";
  const collection = "user_data";

  const rqst = new FindOneRqst();
  rqst.setId(database);
  rqst.setDatabase(database);
  rqst.setCollection(collection);
  rqst.setQuery(query);
  rqst.setOptions("");

  // call persist data
  globular.persistenceService
    .findOne(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: any) => {
      callback(JSON.parse(rsp.getJsonstr()));
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Read all user data.
 * @param globular
 * @param application
 * @param domain
 * @param query
 * @param callback
 * @param errorCallback
 */
export function readUserData(
  globular: Globular,
  query: string,
  callback: (results: any) => void,
  errorCallback: (err: any) => void
) {
  const userName = localStorage.getItem("user_name");
  const database = userName + "_db";
  const collection = "user_data";

  const rqst = new FindOneRqst();
  rqst.setId(database);
  rqst.setDatabase(database);
  rqst.setCollection(collection);
  rqst.setQuery(query);
  rqst.setOptions("");

  // call persist data
  const stream = globular.persistenceService.find(rqst, {
    token: getToken(),
    application: application.length > 0 ? application : globular.config.IndexApplication,
    domain: domain,
  });

  let data = []
  // Get the stream and set event on it...
  stream.on("data", (rsp) => {
    data = mergeTypedArrays(data, rsp.getData())
  });

  stream.on("status", (status) => {
    if (status.code === 0) {
      let results = JSON.parse(uint8arrayToStringMethod(data))
      callback(results);
    } else {
      errorCallback({ message: status.details });
    }
  });
}

///////////////////////////////////// Role action //////////////////////////////////////

/**
 * Retreive all available actions on the server.
 * @param callback That function is call in case of success.
 * @param errorCallback That function is call in case error.
 */
export function getAllActions(
  globular: Globular,
  callback: (ations: string[]) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new GetAllActionsRqst();
  globular.resourceService
    .getAllActions(rqst, { application: application.length > 0 ? application : globular.config.IndexApplication, domain: domain })
    .then((rsp: GetAllActionsRsp) => {
      callback(rsp.getActionsList());
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Retreive the list of all available roles on the server.
 * @param callback That function is call in case of success.
 * @param errorCallback That function is call in case error.
 */
export function getAllRoles(
  globular: Globular,
  callback: (roles: any[]) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new FindRqst();
  rqst.setCollection("Roles");
  rqst.setDatabase("local_resource");
  rqst.setId("local_resource");
  rqst.setQuery("{}"); // means all values.

  const stream = globular.persistenceService.find(rqst, {
    application: application.length > 0 ? application : globular.config.IndexApplication,
    domain: domain,
  });

  let data = [];

  stream.on("data", (rsp: FindResp) => {
    data = mergeTypedArrays(data, rsp.getData())
  });

  stream.on("status", (status) => {
    if (status.code === 0) {
      callback(JSON.parse(uint8arrayToStringMethod(data)));
    } else {
      errorCallback({ message: status.details });
    }
  });
}

/**
 * Append Action to a given role.
 * @param action The action name.
 * @param role The role.
 * @param callback The success callback
 * @param errorCallback The error callback.
 */
export function appendActionToRole(
  globular: Globular,
  role: string,
  actions: Array<string>,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new AddRoleActionsRqst();
  rqst.setRoleid(role);
  rqst.setActionsList(actions);

  globular.resourceService
    .addRoleActions(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: AddRoleActionsRsp) => {
      callback();
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Remove the action from a given role.
 * @param action The action id
 * @param role The role id
 * @param callback success callback
 * @param errorCallback error callback
 */
export function removeActionFromRole(
  globular: Globular,
  role: string,
  action: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new RemoveRoleActionRqst();
  rqst.setRoleid(role);
  rqst.setAction(action);

  globular.resourceService
    .removeRoleAction(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: RemoveAccountRoleRsp) => {
      callback();
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Create a new Role
 * @param globular
 * @param application
 * @param domain
 * @param id
 * @param callback
 * @param errorCallback
 */
export function createRole(
  globular: Globular,
  id: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new CreateRoleRqst();
  const role = new Role();
  role.setId(id);
  role.setName(id);
  rqst.setRole(role);

  globular.resourceService
    .createRole(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: CreateRoleRsp) => {
      callback();
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Delete a given role
 * @param globular
 * @param application
 * @param domain
 * @param id
 * @param callback
 * @param errorCallback
 */
export function deleteRole(
  globular: Globular,
  id: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new DeleteRoleRqst();
  rqst.setRoleid(id);

  globular.resourceService
    .deleteRole(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: CreateRoleRsp) => {
      callback();
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

///////////////////////////////////// Application operations /////////////////////////////////

/**
 * Return the list of all application
 * @param globular
 * @param application
 * @param domain
 * @param callback
 * @param errorCallback
 */
export function getAllApplicationsInfo(
  globular: Globular,
  callback: (infos: any) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new GetAllApplicationsInfoRqst();
  globular.resourceService
    .getAllApplicationsInfo(rqst)
    .then((rsp: GetAllApplicationsInfoRsp) => {
      const infos =  rsp.getApplicationsList();
      callback(infos);
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Append action to application.
 * @param globular
 * @param application
 * @param domain
 * @param applicationId
 * @param action
 * @param callback
 * @param errorCallback
 */
export function appendActionToApplication(
  globular: Globular,
  applicationId: string,
  actions: Array<string>,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new AddApplicationActionsRqst();
  rqst.setApplicationid(applicationId);
  rqst.setActionsList(actions);
  globular.resourceService
    .addApplicationActions(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: AddApplicationActionsRsp) => {
      callback();
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Remove action from application.
 * @param globular
 * @param application
 * @param domain
 * @param action
 * @param callback
 * @param errorCallback
 */
export function removeActionFromApplication(
  globular: Globular,
  action: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new RemoveApplicationActionRqst();
  rqst.setApplicationid(application);
  rqst.setAction(action);
  globular.resourceService
    .removeApplicationAction(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: RemoveApplicationActionRsp) => {
      callback();
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Delete application
 * @param globular
 * @param application
 * @param domain
 * @param applicationId
 * @param callback
 * @param errorCallback
 */
export function deleteApplication(
  globular: Globular,
  applicationId: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new DeleteApplicationRqst();
  rqst.setApplicationid(applicationId);
  globular.resourceService
    .deleteApplication(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: DeleteApplicationRsp) => {
      callback();
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Save application
 * @param globular
 * @param eventHub
 * @param applicationId
 * @param domain
 * @param application
 * @param callback
 * @param errorCallback
 */
export function saveApplication(
  globular: Globular,
  eventHub: EventHub,
  _application: any,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new ReplaceOneRqst();
  rqst.setCollection("Applications");
  rqst.setDatabase("local_resource");
  rqst.setId("local_resource");
  rqst.setValue(JSON.stringify(_application));
  rqst.setQuery(`{"_id":"${_application._id}"}`); // means all values.

  globular.persistenceService
    .replaceOne(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: ReplaceOneRsp) => {
      eventHub.publish(
        "update_application_info_event",
        JSON.stringify(application),
        false
      );
      callback();
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

///////////////////////////////////// Services operations /////////////////////////////////

/**
 * Return a list of service descriptor related to a service host by a server.
 * @param globular
 * @param application
 * @param domain
 * @param serviceId
 * @param publisherId
 * @param callback
 * @param errorCallback
 */
export function getPackageDescriptor(
  globular: Globular,
  serviceId: string,
  publisherId: string,
  callback: (descriptors: PackageDescriptor[]) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new GetPackageDescriptorRequest();
  rqst.setServiceid(serviceId);
  rqst.setPublisherid(publisherId);

  globular.packagesDicovery
    .getPackageDescriptor(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: GetPackageDescriptorResponse) => {
      callback(rsp.getResultsList());
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Get the list of all service descriptor hosted on a server.
 * @param globular The globular object instance
 * @param application The application name who called the function.
 * @param domain The domain where the application reside.
 * @param callback
 * @param errorCallback
 */
export function GetPackagesDescriptor(
  globular: Globular,
  callback: (descriptors: PackageDescriptor[]) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new GetPackagesDescriptorRequest();

  const stream = globular.packagesDicovery.getPackagesDescriptor(rqst, {
    application: application.length > 0 ? application : globular.config.IndexApplication,
    domain: domain,
  });

  let descriptors = new Array<PackageDescriptor>();

  stream.on("data", (rsp: GetPackagesDescriptorResponse) => {
    descriptors = descriptors.concat(rsp.getResultsList());
  });

  stream.on("status", (status) => {
    if (status.code === 0) {
      callback(descriptors);
    } else {
      errorCallback({ message: status.details });
    }
  });
}

/**
 * Create or update a service descriptor.
 * @param globular
 * @param application
 * @param domain
 * @param descriptor
 * @param callback
 * @param errorCallback
 */
export function setServicesDescriptor(
  globular: Globular,
  descriptor: PackageDescriptor,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new SetPackageDescriptorRequest();
  rqst.setDescriptor(descriptor);

  globular.packagesDicovery
    .setPackageDescriptor(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then(callback)
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Find services by keywords.
 * @param query
 * @param callback
 */
export function FindPackages(
  globular: Globular,
  keywords: string[],
  callback: (results: PackageDescriptor[]) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new FindPackagesDescriptorRequest();
  rqst.setKeywordsList(keywords);

  // Find services by keywords.
  globular.packagesDicovery
    .findPackages(rqst, { application: application.length > 0 ? application : globular.config.IndexApplication, domain: domain })
    .then((rsp: FindPackagesDescriptorResponse) => {
      callback(rsp.getResultsList());
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Install a service
 * @param globular
 * @param application
 * @param domain
 * @param discoveryId
 * @param serviceId
 * @param publisherId
 * @param version
 * @param callback
 * @param errorCallback
 */
export function installService(
  globular: Globular,
  discoveryId: string,
  serviceId: string,
  publisherId: string,
  version: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new InstallServiceRequest();
  rqst.setPublisherid(publisherId);
  rqst.setDicorveryid(discoveryId);
  rqst.setServiceid(serviceId);
  rqst.setVersion(version);

  // Install the service.
  globular.adminService
    .installService(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: InstallServiceResponse) => {
      readFullConfig(globular, callback, errorCallback);
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Stop a service.
 */
export function stopService(
  globular: Globular,
  serviceId: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new StopServiceRequest();
  rqst.setServiceId(serviceId);
  globular.adminService
    .stopService(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then(() => {
      callback();
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Start a service
 * @param serviceId The id of the service to start.
 * @param callback  The callback on success.
 */
export function startService(
  globular: Globular,
  serviceId: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new StartServiceRequest();
  rqst.setServiceId(serviceId);
  globular.adminService
    .startService(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then(() => {
      callback();
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Here I will save the service configuration.
 * @param service The configuration to save.
 */
export function saveService(
  globular: Globular,
  service: IServiceConfig,
  callback: (config: any) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new SaveConfigRequest();

  rqst.setConfig(JSON.stringify(service));
  globular.adminService
    .saveConfig(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: SaveConfigResponse) => {
      // The service with updated values...
      callback(JSON.parse(rsp.getResult()));
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Uninstall a service from the server.
 * @param globular
 * @param application
 * @param domain
 * @param service
 * @param deletePermissions
 * @param callback
 * @param errorCallback
 */
export function uninstallService(
  globular: Globular,
  service: IServiceConfig,
  deletePermissions: boolean,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new UninstallServiceRequest();
  rqst.setServiceid(service.Id);
  rqst.setPublisherid(service.PublisherId);
  rqst.setVersion(service.Version);
  rqst.setDeletepermissions(deletePermissions);

  globular.adminService
    .uninstallService(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: UninstallServiceResponse) => {
      delete globular.config.Services[service.Id];
      // The service with updated values...
      callback();
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Return the list of service bundles.
 * @param callback
 */
export function getServiceBundles(
  globular: Globular,
  publisherId: string,
  serviceName: string,
  serviceId: string,
  version: string,
  callback: (bundles: Array<any>) => void,
  errorCallback: (err: any) => void
) {
  let rqst = new FindRqst();
  rqst.setCollection("ServiceBundle");
  rqst.setDatabase("local_resource");
  rqst.setId("local_resource");
  rqst.setQuery(`{}`); // means all values.

  var stream = globular.persistenceService.find(rqst, {
    application: application.length > 0 ? application : globular.config.IndexApplication,
    domain: domain + ":" + window.location.port,
  });

  let data = []

  stream.on("data", (rsp: FindResp) => {

    data = mergeTypedArrays(data, rsp.getData())
  });

  stream.on("status", function (status: any) {
    if (status.code == 0) {
      // filter localy.
      let bundles = JSON.parse(uint8arrayToStringMethod(data))

      callback(
        bundles.filter((bundle) =>
          String(bundle._id).startsWith(
            publisherId + "%" + serviceName + "%" + version + "%" + serviceId
          )
        )
      );
    } else {
      errorCallback({ message: status.details });
    }
  });
}

/**
 * Get the object pointed by a reference.
 * @param globular
 * @param application
 * @param domain
 * @param ref
 * @param callback
 * @param errorCallback
 */
export function getReferencedValue(
  globular: Globular,
  ref: any,
  callback: (results: any) => void,
  errorCallback: (err: any) => void
) {
  const database = ref.$db;
  const collection = ref.$ref;

  const rqst = new FindOneRqst();
  rqst.setId(database);
  rqst.setDatabase(database);
  rqst.setCollection(collection);
  rqst.setQuery(`{"_id":"${ref.$id}"}`);
  rqst.setOptions("");

  globular.persistenceService
    .findOne(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: FindOneResp) => {
      callback( rsp.getResult());
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

///////////////////////////// Logging Operations ////////////////////////////////////////

/**
 * Read all errors data for server log.
 * @param globular
 * @param application
 * @param domain
 * @param callback
 * @param errorCallback
 */
export function readErrors(
  globular: Globular,
  callback: (results: any) => void,
  errorCallback: (err: any) => void
) {
  const database = "local_resource";
  const collection = "Logs";

  const rqst = new FindOneRqst();
  rqst.setId(database);
  rqst.setDatabase(database);
  rqst.setCollection(collection);
  rqst.setOptions("");
  rqst.setQuery("{}");

  // call persist data
  const stream = globular.persistenceService.find(rqst, {
    token: getToken(),
    application: application.length > 0 ? application : globular.config.IndexApplication,
    domain: domain,
  });

  let data: any

  // Get the stream and set event on it...
  stream.on("data", (rsp) => {
    data = mergeTypedArrays(data, rsp.getData())
  });

  stream.on("status", (status) => {
    if (status.code === 0) {
      callback(JSON.parse(uint8arrayToStringMethod(data)));
    } else {
      errorCallback({ message: status.details });
    }
  });
}

/**
 *  Read all logs
 * @param globular
 * @param application
 * @param domain
 * @param query
 * @param callback
 * @param errorCallback
 */
export function readLogs(
  globular: Globular,
  query: string,
  callback: (results: any) => void,
  errorCallback: (err: any) => void
) {
  const rqst = new GetLogRqst();
  rqst.setQuery(query);

  // call persist data
  const stream = globular.logService.getLog(rqst, {
    token: getToken(),
    application: application.length > 0 ? application : globular.config.IndexApplication,
    domain: domain,
  });

  let results = new Array<LogInfo>();

  // Get the stream and set event on it...
  stream.on("data", (rsp) => {
    results = results.concat(rsp.getInfosList());
  });

  stream.on("status", (status) => {
    if (status.code === 0) {
      results = results.sort((t1, t2) => {
        const name1 = t1.getDate();
        const name2 = t2.getDate();
        if (name1 < name2) {
          return 1;
        }
        if (name1 > name2) {
          return -1;
        }
        return 0;
      });

      callback(results);
    } else {
      errorCallback({ message: status.details });
    }
  });
}

/**
 * Clear all log of a given type.
 * @param globular
 * @param application
 * @param domain
 * @param logType
 * @param callback
 * @param errorCallback
 */
export function clearAllLog(
  globular: Globular,
  query: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new ClearAllLogRqst();
  rqst.setQuery(query);
  globular.logService
    .clearAllLog(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then(callback)
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Delete log entry.
 * @param globular
 * @param application
 * @param domain
 * @param log
 * @param callback
 * @param errorCallback
 */
export function deleteLogEntry(
  globular: Globular,
  log: LogInfo,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new DeleteLogRqst();
  rqst.setLog(log);
  globular.logService
    .deleteLog(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then(callback)
    .catch((err: any) => {
      errorCallback(err);
    });
}

/**
 * Return the logged method and their count.
 * @param pipeline
 * @param callback
 * @param errorCallback
 */
export function getNumbeOfLogsByMethod(
  globular: Globular,
  callback: (resuts: any[]) => void,
  errorCallback: (err: any) => void
) {
  const database = "local_resource";
  const collection = "Logs";
  const rqst = new AggregateRqst();
  rqst.setId(database);
  rqst.setDatabase(database);
  rqst.setCollection(collection);
  rqst.setOptions("");

  const pipeline = `[{"$group":{"_id":{"method":"$method"}, "count":{"$sum":1}}}]`;

  rqst.setPipeline(pipeline);

  // call persist data
  const stream = globular.persistenceService.aggregate(rqst, {
    token: getToken(),
    application: application.length > 0 ? application : globular.config.IndexApplication,
    domain: domain,
  });

  let data = [];

  // Get the stream and set event on it...
  stream.on("data", (rsp) => {
    data = mergeTypedArrays(data, rsp.getData())
  });

  stream.on("status", (status) => {
    if (status.code === 0) {
      callback(JSON.parse(uint8arrayToStringMethod(data)));
    } else {
      errorCallback({ message: status.details });
    }
  });
}

///////////////////////////////////// LDAP operations /////////////////////////////////

/**
 * Synchronize LDAP and Globular/MongoDB user and roles.
 * @param info The synchronisations informations.
 * @param callback success callback.
 */
export function syncLdapInfos(
  globular: Globular,
  info: any,
  timeout: number,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  const rqst = new SynchronizeLdapRqst();
  const syncInfos = new LdapSyncInfos();
  syncInfos.setConnectionid(info.connectionId);
  syncInfos.setLdapseriveid(info.ldapSeriveId);
  syncInfos.setRefresh(info.refresh);

  const userSyncInfos = new UserSyncInfos();
  userSyncInfos.setBase(info.userSyncInfos.base);
  userSyncInfos.setQuery(info.userSyncInfos.query);
  userSyncInfos.setId(info.userSyncInfos.id);
  userSyncInfos.setEmail(info.userSyncInfos.email);
  syncInfos.setUsersyncinfos(userSyncInfos);

  const groupSyncInfos = new GroupSyncInfos();
  groupSyncInfos.setBase(info.groupSyncInfos.base);
  groupSyncInfos.setQuery(info.groupSyncInfos.query);
  groupSyncInfos.setId(info.groupSyncInfos.id);
  syncInfos.setGroupsyncinfos(groupSyncInfos);

  rqst.setSyncinfo(syncInfos);

  // Try to synchronyze the ldap service.
  globular.resourceService
    .synchronizeLdap(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: SynchronizeLdapRsp) => {
      callback();
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

///////////////////////////////////// SQL operations /////////////////////////////////

/**
 * Ping globular sql service.
 * @param globular
 * @param application
 * @param domain
 * @param connectionId
 * @param callback
 * @param errorCallback
 */
export function pingSql(
  globular: Globular,
  connectionId: string,
  callback: (pong: string) => {},
  errorCallback: (err: any) => void
) {
  const rqst = new PingConnectionRqst();
  rqst.setId(connectionId);

  globular.sqlService
    .ping(rqst, {
      token: getToken(),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: PingConnectionRsp) => {
      callback(rsp.getResult());
    })
    .catch((err: any) => {
      errorCallback(err);
    });
}

///////////////////////////// Search Operations ////////////////////////////////////////
/**
 * Search Documents from given database(s) and return results. The search engine in use is
 * xapian, so query must follow xapian query rules.
 * @param globular The server object.
 * @param paths The list of database paths.
 * @param query The query to execute.
 * @param language The language of the database.
 * @param fields The list of field to query, can be empty if all fields must be search or fields are specified in the query.
 * @param offset The offset of resultset
 * @param pageSize The number of result to return
 * @param snippetLength The length of the snippet result.
 * @param callback The success callback
 * @param errorCallback The error callback.
 */
export function searchDocuments(
  globular: Globular,
  paths: string[],
  query: string,
  language: string,
  fields: string[],
  offset: number,
  pageSize: number,
  snippetLength: number,
  callback: (results: SearchResult[]) => void,
  errorCallback: (err: any) => void
) {
  let rqst = new SearchDocumentsRequest();
  rqst.setPathsList(paths);
  rqst.setQuery(query);
  rqst.setLanguage(language);
  rqst.setFieldsList(fields);
  rqst.setOffset(offset);
  rqst.setPagesize(pageSize);
  rqst.setSnippetlength(snippetLength);

  let stream = globular.searchService.searchDocuments(rqst, {
    token: localStorage.getItem("user_token"),
    application: application.length > 0 ? application : globular.config.IndexApplication,
    domain: domain,
  });

  let results = new Array<SearchResult>();

  // Get the stream and set event on it...
  stream.on("data", (rsp: SearchDocumentsResponse) => {
    results = results.concat(rsp.getResults().getResultsList())
  });

  stream.on("status", (status) => {
    if (status.code == 0) {
      callback(results)
    } else {
      errorCallback({ message: status.details });
    }
  });
}

///////////////////////////////////////////////////////////////////////////////////////////////
// Peers
///////////////////////////////////////////////////////////////////////////////////////////////

export function createPeer(
  globular: Globular,
  name: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  let rqst = new RegisterPeerRqst();
  let peer = new Peer();
  peer.setDomain(name);

  rqst.setPeer(peer);
  globular.resourceService
    .registerPeer(rqst, {
      token: localStorage.getItem("user_token"),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then(callback)
    .catch(errorCallback);
}

export function getAllPeersInfo(
  globular: Globular,
  callback: (peers: Peer[]) => void,
  errorCallback: (err: any) => void
) {
  let rqst = new GetPeersRqst();
  rqst.setQuery("{}");
  let peers = new Array<Peer>();

  let stream = globular.resourceService.getPeers(rqst, {
    token: localStorage.getItem("user_token"),
    application: application.length > 0 ? application : globular.config.IndexApplication,
    domain: domain,
  });

  // Get the stream and set event on it...
  stream.on("data", (rsp: GetPeersRsp) => {
    peers = peers.concat(rsp.getPeersList());
  });

  stream.on("status", (status) => {
    if (status.code == 0) {
      callback(peers);
    } else {
      errorCallback({ message: status.details });
    }
  });
}

export function appendActionToPeer(
  globular: Globular,
  domain: string,
  actions: Array<string>,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  let rqst = new AddPeerActionsRqst();
  rqst.setDomain(domain);
  rqst.setActionsList(actions);
  globular.resourceService
    .addPeerActions(rqst, {
      token: localStorage.getItem("user_token"),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: AddPeerActionsRsp) => {
      callback();
    })
    .catch((err: any) => {
      console.log(err);
      errorCallback(err);
    });
}

export function removeActionFromPeer(
  globular: Globular,
  domain: string,
  action: string,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  let rqst = new RemovePeerActionRqst();
  rqst.setDomain(domain);
  rqst.setAction(action);
  globular.resourceService
    .removePeerAction(rqst, {
      token: localStorage.getItem("user_token"),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: RemovePeerActionRsp) => {
      callback();
    })
    .catch((err: any) => {
      console.log(err);
      errorCallback(err);
    });
}

export function deletePeer(
  globular: Globular,
  peer: Peer,
  callback: () => void,
  errorCallback: (err: any) => void
) {
  let rqst = new DeletePeerRqst();
  rqst.setPeer(peer);
  globular.resourceService
    .deletePeer(rqst, {
      token: localStorage.getItem("user_token"),
      application: application.length > 0 ? application : globular.config.IndexApplication,
      domain: domain,
    })
    .then((rsp: DeletePeerRsp) => {
      callback();
    })
    .catch((err: any) => {
      console.log(err);
      errorCallback(err);
    });
}
