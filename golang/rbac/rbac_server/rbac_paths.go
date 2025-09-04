// rbac_paths.go: path and file service utilities.

package main

import (
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/file/file_client"
	"github.com/globulario/services/golang/globular_client"
	Utility "github.com/globulario/utility"
	"net/url"
	"strings"
)

func (srv *server) getFileClient() (*file_client.File_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewFileService_Client", file_client.NewFileService_Client)

	client, err := globular_client.GetClient(address, "file.FileService", "NewFileService_Client")
	if err != nil {
		logPrintln("fail to connect to file client with error: ", err)
		return nil, err
	}

	return client.(*file_client.File_Client), nil
}

func (srv *server) getPublicDirs() ([]string, error) {
	client, err := srv.getFileClient()
	if err != nil {
		return nil, err
	}

	// Get the public dir.
	public, err := client.GetPublicDirs()
	if err != nil {
		return nil, err
	}

	return public, nil
}

func (srv *server) isPublic(path string) bool {
	path = strings.ReplaceAll(path, "\\", "/")
	publics, err := srv.getPublicDirs()
	if err != nil {
		return false
	}

	if Utility.Exists(path) {
		for i := range publics {
			if strings.HasPrefix(path, publics[i]) {
				return true
			}
		}
	}
	return false
}

func (srv *server) formatPath(path string) string {
	path, _ = url.PathUnescape(path)
	path = strings.ReplaceAll(path, "\\", "/")

	if strings.HasPrefix(path, "/") {

		if len(path) > 1 {
			if strings.HasPrefix(path, "/") {
				if !srv.isPublic(path) {
					// Must be in the root path if it's not in public path.
					if Utility.Exists(srv.Root + path) {
						path = srv.Root + path
					} else if Utility.Exists(config.GetWebRootDir() + path) {
						path = config.GetWebRootDir() + path

					} else if strings.HasPrefix(path, "/users/") || strings.HasPrefix(path, "/applications/") {
						path = config.GetDataDir() + "/files" + path
					} else if Utility.Exists("/" + path) { // network path...
						path = "/" + path
					} else {
						path = srv.Root + "/" + path
					}
				}
			} else {
				path = srv.Root + "/" + path
			}
		} else {
			// '/' represent the root path
			path = srv.Root
		}
	}

	return path
}

func isPublic(path string, exact_match bool) bool {
	public := config.GetPublicDirs()
	path = strings.ReplaceAll(path, "\\", "/")
	if Utility.Exists(path) {
		for i := range public {
			if !exact_match {
				if strings.HasPrefix(path, public[i]) {
					return true
				}
			} else {
				if path == public[i] {
					return true
				}
			}
		}
	}
	return false
}