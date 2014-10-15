// An example FTP server build on top of graval. graval handles the details
// of the FTP protocol, we just provide a persistence driver for the MG-RAST API.
//
// USAGE:
//
//    go get github.com/MG-RAST/graval
//    go install github.com/MG-RAST/graval/graval-mgrast
//    ./bin/graval-mgrast
//
package main

import (
	"github.com/MG-RAST/graval"
	"io"
	"log"
	"os"
	"strconv"
	"time"
)

const (
	fileOne = "This is the first file available for download.\n\nBy Jàmes"
)

// A minimal driver for graval that queries the MG-RAST API. The authentication
// details are fixed and the user is unable to upload, delete or rename any files.
type MemDriver struct{}

func (driver *MemDriver) Authenticate(user string, pass string) bool {
	return user == "test" && pass == "1234"
}
func (driver *MemDriver) Bytes(path string) (bytes int) {
	switch path {
	case "/one.txt":
		bytes = len(fileOne)
		break
	case "/files/mgm4582802.3-050.1":
		bytes = 2522368548
		break
	default:
		bytes = -1
	}
	return
}
func (driver *MemDriver) ModifiedTime(path string) (time.Time, error) {
	return time.Now(), nil
}
func (driver *MemDriver) ChangeDir(path string) bool {
	return path == "/" || path == "/files"
}
func (driver *MemDriver) DirContents(path string) (files []os.FileInfo) {
	files = []os.FileInfo{}
	switch path {
	case "/":
		files = append(files, graval.NewDirItem("files"))
		files = append(files, graval.NewFileItem("one.txt", len(fileOne)))
	case "/files":
		files = append(files, graval.NewFileItem("mgm4582802.3-050.1", 2522368548))
	}
	return files
}

func (driver *MemDriver) DeleteDir(path string) bool {
	return false
}
func (driver *MemDriver) DeleteFile(path string) bool {
	return false
}
func (driver *MemDriver) Rename(fromPath string, toPath string) bool {
	return false
}
func (driver *MemDriver) MakeDir(path string) bool {
	return false
}
func (driver *MemDriver) GetFile(path string) (data string, bytes string, dataIsUrl bool, err error) {	
	switch path {
	case "/one.txt":
		data = "fileOne"
		bytes = strconv.Itoa(len(fileOne))
		dataIsUrl = false
	case "/files/mgm4582802.3-050.1":
		data = "http://api.metagenomics.anl.gov//download/mgm4582802.3?file=050.1"
		bytes = "2522368548"
		dataIsUrl = true
	}
	return
}
func (driver *MemDriver) PutFile(destPath string, data io.Reader) bool {
	return false
}

// graval requires a factory that will create a new driver instance for each
// client connection. Generally the factory will be fairly minimal. This is
// a good place to read any required config for your driver.
type MemDriverFactory struct{}

func (factory *MemDriverFactory) NewDriver() (graval.FTPDriver, error) {
	return &MemDriver{}, nil
}

// it's alive!
func main() {
	factory := &MemDriverFactory{}
	ftpServer := graval.NewFTPServer(&graval.FTPServerOpts{ Factory: factory })
	err := ftpServer.ListenAndServe()
	if err != nil {
		log.Print(err)
		log.Fatal("Error starting server!")
	}
}
