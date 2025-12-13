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

	if srv.storageExists(path) {
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

	if srv.isMinioPath(path) {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}
