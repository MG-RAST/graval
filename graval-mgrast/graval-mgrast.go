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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/MG-RAST/graval"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"
)

var (
	dirProjects = regexp.MustCompile(`^\/projects\/(mgp\d+)$`)
	dirMetagenomes = regexp.MustCompile(`^\/projects\/(mgp\d+)\/(mgm\d+\.\d+)$`)
	fileDownload = regexp.MustCompile(`^\/projects\/mgp\d+\/mgm\d+\.\d+/(mgm\d+\.\d+)_(\d+\.\d+)$`)
)

// This is the struct for a project item in http://api.metagenomics.anl.gov/project?limit=0
type ProjectListItem struct {
	Created	string
	Version	int
	Status	string
	Url		string
	Name	string
	Pi		string
	Id		string
}

// This is the outer struct for the project list from http://api.metagenomics.anl.gov/project?limit=0
type ProjectList struct {
	Next		string
	Prev		string
	Url			string
	Data		[]ProjectListItem
	Limit		int
	Total_count	int
	Offset		int
}

// This is the outer struct returned when calling http://api.metagenomics.anl.gov/project/mgp9?verbosity=full
type ProjectResource struct {
	Version			int
	Status			string
	Name			string
	Description		string
	Libraries		[][]string
	Metagenomes		[][]string
	Created			string
	Samples			[][]string
	Funding_source	string
	Url				string
	Metadata		interface{}
	Id				string
	Pi				string
}

// This is the struct for a downloadable file in http://api.metagenomics.anl.gov/download/mgm4441619.3
type DownloadListItem struct {
	Stage_name		string
	File_size		int
	Seq_format		string
	Statistics		interface{}
	Data_type 		string
	File_name		string
	Node_id			string
	Url				string
	Id				string
	File_id			string
	File_format		string
	File_md5		string
	Stage_id		string
}

// This is the outer struct returned when calling http://api.metagenomics.anl.gov/download/mgm4441619.3
type DownloadList struct {
	Url		string
	Data	[]DownloadListItem
	Id		string
}

// A minimal driver for graval that queries the MG-RAST API. The authentication
// details are fixed and the user is unable to upload, delete or rename any files.
type MemDriver struct{}

func (driver *MemDriver) Authenticate(user string, pass string) bool {
	return user == "test" && pass == "1234"
}
func (driver *MemDriver) Bytes(path string) (bytes int) {
	bytes = -1
	if matches := fileDownload.FindStringSubmatch(path); len(matches) == 3 {
		// Retrieving list of downloadable files for this metagenome
		resp, err := http.Get("http://api.metagenomics.anl.gov/download/" + matches[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not retrieve download listing for metagenome, received err: %v\n", err.Error())
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not parse response body, received err: %v\n", err.Error())
		}

		var dList DownloadList
		err = json.Unmarshal(body, &dList)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not unmarshal response body, received err: %v\n", err.Error())
		}

		// Add file item for each downloadable file in this metagenome
		for _, v := range dList.Data {
			if v.File_id == matches[2] {
				bytes = v.File_size
				break
			}
		}
	}
	return
}
func (driver *MemDriver) ModifiedTime(path string) (time.Time, error) {
	return time.Now(), nil
}
func (driver *MemDriver) ChangeDir(path string) bool {
	return path == "/" || path == "/projects" || dirProjects.MatchString(path) || dirMetagenomes.MatchString(path)
}
func (driver *MemDriver) DirContents(path string) (files []os.FileInfo) {
	files = []os.FileInfo{}
	if path == "/" {
		files = append(files, graval.NewDirItem("projects"))
	} else if path == "/projects" {
		// Retrieving list of public projects
		resp, err := http.Get("http://api.metagenomics.anl.gov/project?limit=0")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not retrieve project listing, received err: %v\n", err.Error())
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not parse response body, received err: %v\n", err.Error())
		}

		var pList ProjectList
		err = json.Unmarshal(body, &pList)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not unmarshal response body, received err: %v\n", err.Error())
		}

		// Add directory item for each public project
		for _, v := range pList.Data {
			files = append(files, graval.NewDirItem(v.Id))
		}
	} else if matches := dirProjects.FindStringSubmatch(path); len(matches) == 2 {
		// Retrieving list of metagenomes for this project
		resp, err := http.Get("http://api.metagenomics.anl.gov/project/" + matches[1] + "?verbosity=full")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not retrieve list of metagenomes for project, received err: %v\n", err.Error())
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not parse response body, received err: %v\n", err.Error())
		}

		var pRes ProjectResource
		err = json.Unmarshal(body, &pRes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not unmarshal response body, received err: %v\n", err.Error())
		}
		
		// Add directory item for each metagenome in this project
		for _, v := range pRes.Metagenomes {
			files = append(files, graval.NewDirItem(v[0]))
		}
	} else if matches := dirMetagenomes.FindStringSubmatch(path); len(matches) == 3 {
		// Retrieving list of downloadable files for this metagenome
		resp, err := http.Get("http://api.metagenomics.anl.gov/download/" + matches[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not retrieve download listing for metagenome, received err: %v\n", err.Error())
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not parse response body, received err: %v\n", err.Error())
		}

		var dList DownloadList
		err = json.Unmarshal(body, &dList)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not unmarshal response body, received err: %v\n", err.Error())
		}
		
		// Add file item for each downloadable file in this metagenome
		for _, v := range dList.Data {
			files = append(files, graval.NewFileItem(dList.Id + "_" + v.File_id, v.File_size))
		}
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
	if matches := fileDownload.FindStringSubmatch(path); len(matches) == 3 {
		// Retrieving list of downloadable files for this metagenome
		resp, err := http.Get("http://api.metagenomics.anl.gov/download/" + matches[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not retrieve download listing for metagenome, received err: %v\n", err.Error())
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not parse response body, received err: %v\n", err.Error())
		}

		var dList DownloadList
		err = json.Unmarshal(body, &dList)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not unmarshal response body, received err: %v\n", err.Error())
		}

		// Add file item for each downloadable file in this metagenome
		for _, v := range dList.Data {
			if v.File_id == matches[2] {
				data = "http://shock.metagenomics.anl.gov/node/" + v.Node_id + "?download"
				bytes = strconv.Itoa(v.File_size)
				dataIsUrl = true
				break
			}
		}
	} else {
		err = errors.New("Could not retrieve file from MG-RAST API.")
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
