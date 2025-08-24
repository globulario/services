package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	Utility "github.com/davecourtois/!utility"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/resource/resourcepb"
)

// Repository
/** Download a service from a service directory **/
func (srv *server) DownloadBundle(rqst *repositorypb.DownloadBundleRequest, stream repositorypb.PackageRepository_DownloadBundleServer) error {
	bundle := new(resourcepb.PackageBundle)
	bundle.Plaform = rqst.Platform
	bundle.PackageDescriptor = rqst.Descriptor_

	// Generate the bundle id....
	id := Utility.GenerateUUID(bundle.PackageDescriptor.PublisherId + "%" + bundle.PackageDescriptor.Name + "%" + bundle.PackageDescriptor.Version + "%" + bundle.PackageDescriptor.Id + "%" + rqst.Platform)
	path := srv.Root + "/packages-repository"

	var err error

	// the file must be a zipped archive that contain a .proto, .config and executable.
	bundle.Binairies, err = ioutil.ReadFile(path + "/" + id + ".tar.gz")
	if err != nil {
		return err
	}

	checksum, err := srv.getPackageBundleChecksum(id)
	if err != nil {
		return err
	}

	// Test if the values change over time.
	if string(Utility.CreateDataChecksum(bundle.Binairies)) != checksum {
		return errors.New("the bundle data cheksum is not valid")
	}

	const BufferSize = 1024 * 5 // the chunck size.
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer) // Will write to network.
	err = enc.Encode(bundle)
	if err != nil {
		return err
	}

	for {
		var data [BufferSize]byte
		bytesread, err := buffer.Read(data[0:BufferSize])
		if bytesread > 0 {
			rqst := &repositorypb.DownloadBundleResponse{
				Data: data[0:bytesread],
			}
			// send the data to the srv.
			err = stream.Send(rqst)
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}

	return nil
}

/** Upload a service to a service directory **/
func (srv *server) UploadBundle(stream repositorypb.PackageRepository_UploadBundleServer) error {

	// The bundle will cantain the necessary information to install the service.
	var buffer bytes.Buffer
	for {
		msg, err := stream.Recv()
		if msg == nil {
			srv.logServiceError("UploadBundle", Utility.FunctionName(), Utility.FileLine(), "the message is nil")
			return errors.New("the message is nil")
		} else if err == io.EOF || len(msg.Data) == 0 {
			// end of stream...
			err_ := stream.SendAndClose(&repositorypb.UploadBundleResponse{})
			if err_ != nil {
				fmt.Println("fail send response and close stream with error ", err_)
				return err_
			}
			err = nil
			break
		} else if err != nil {
			srv.logServiceError("UploadBundle", Utility.FunctionName(), Utility.FileLine(), err.Error())
			return err
		} else if msg != nil {
			buffer.Write(msg.Data)
		}
	}

	// The buffer that contain the
	dec := gob.NewDecoder(&buffer)
	bundle := new(resourcepb.PackageBundle)
	err := dec.Decode(bundle)
	if err != nil {
		srv.logServiceError("UploadBundle", Utility.FunctionName(), Utility.FileLine(), err.Error())
		return err
	}

	// Set the bundle descriptor id.
	bundle.PackageDescriptor.Id = Utility.GenerateUUID(bundle.PackageDescriptor.PublisherId + "%" + bundle.PackageDescriptor.Name + "%" + bundle.PackageDescriptor.Version)

	// Generate the bundle id....
	id := Utility.GenerateUUID(bundle.PackageDescriptor.PublisherId + "%" + bundle.PackageDescriptor.Name + "%" + bundle.PackageDescriptor.Version + "%" + bundle.PackageDescriptor.Id + "%" + bundle.Plaform)

	path := srv.Root + "/packages-repository"
	Utility.CreateDirIfNotExist(path)

	// the file must be a zipped archive that contain a .proto, .config and executable.
	err = ioutil.WriteFile(path+"/"+id+".tar.gz", bundle.Binairies, 0644)

	if err != nil {
		srv.logServiceError("UploadBundle", Utility.FunctionName(), Utility.FileLine(), err.Error())
		return err
	}

	srv.logServiceInfo("UploadBundle", Utility.FileLine(), Utility.FunctionName(), "bundle was save at path "+path+"/"+id+".tar.gz")

	bundle.Checksum = string(Utility.CreateDataChecksum(bundle.Binairies))
	bundle.Size = int32(len(bundle.Binairies))
	bundle.Modified = time.Now().Unix()

	// Save the bundle info...
	return srv.setPackageBundle(bundle.Checksum, bundle.Plaform, bundle.Size, bundle.Modified, bundle.PackageDescriptor)
}
