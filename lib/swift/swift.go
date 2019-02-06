package swift

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	logs "github.com/osallou/hero-file/lib/log"
)

var logger = logs.GetLogger("hero.swift")

// Options to access swift content
type Options struct {
	Bucket        string
	File          string
	ObjectName    string
	Size          int64
	Prefix        string
	LeaveSegments bool
	Meta          map[string]string
}

// SwiftFile describe a swift object
type SwiftFile struct {
	Hash         string
	LastModified string `json:"last_modified"`
	Bytes        uint64
	Name         string
	ContentType  string `json:"content_type"`
}

type Segment struct {
	From int64
	Size int64
}

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		logger.Errorf("%s", err)
		return 0
	}

	return fi.Size()
}

func uploadManifest(token string, server string, segmentPrefix string, options Options) bool {
	client := &http.Client{}
	url := []string{server, options.Bucket, options.ObjectName}
	logger.Debugf("Call %s %s\n", options.Bucket, strings.Join(url, "/"))
	logger.Debugf("Set manifest %s", segmentPrefix)
	byteData := make([]byte, 0)
	req, _ := http.NewRequest("PUT", strings.Join(url, "/"), bytes.NewReader(byteData))
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("X-Object-Manifest", segmentPrefix)
	for m := range options.Meta {
		logger.Debugf("Add metadata %s: %s\n", m, options.Meta[m])
		req.Header.Add("X-Object-Meta-"+m, options.Meta[m])
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("Failed to contact server %s\n", server)
		return false
	}
	if resp.StatusCode != 201 {
		logger.Errorf("Failed to upload file: %s", resp.Status)
		return false
	} else {
		logger.Debugf("Manifest uploaded => %s", strings.Join(url, "/"))
		return true
	}
}

func uploadSegment(ch chan bool, token string, server string, options Options, segment Segment) {
	data, derr := os.Open(options.File)
	var body *bytes.Reader

	logger.Debugf("File %s, Segment %d, %d", options.File, segment.From, segment.Size)
	data.Seek(segment.From, 0)
	byteData := make([]byte, segment.Size)
	data.Read(byteData)
	body = bytes.NewReader(byteData)

	if derr != nil {
		logger.Errorf("Failed to open file %s", options.File)
		ch <- false
		return
	}
	defer data.Close()

	client := &http.Client{}
	segurl := []string{server, options.Bucket, options.ObjectName}
	logger.Debugf("Call %s\n", strings.Join(segurl, "/"))

	req, _ := http.NewRequest("PUT", strings.Join(segurl, "/"), body)
	req.Header.Add("X-Auth-Token", token)
	for m := range options.Meta {
		req.Header.Add("X-Object-Meta-"+m, options.Meta[m])
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("Failed to contact server %s\n", server)
		ch <- false
		return
	}
	if resp.StatusCode != 201 {
		logger.Errorf("Failed to upload file: %s", resp.Status)
		ch <- false
	} else {
		ch <- true
	}
}

// Head checks if remote file is a multi-part object, return manifest value
func Head(token string, server string, options Options) string {
	if options.ObjectName == "" {
		options.ObjectName = options.File
	}
	manifest := ""
	client := &http.Client{}
	url := []string{server, options.Bucket, options.ObjectName}
	logger.Debugf("Call %s\n", strings.Join(url, "/"))
	req, _ := http.NewRequest("HEAD", strings.Join(url, "/"), nil)
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("Failed to contact server %s\n", server)
		return manifest
	}
	defer resp.Body.Close()
	manifest = resp.Header.Get("X-Object-Manifest")
	logger.Debugf("Found old manifest %s", manifest)
	if resp.StatusCode != 200 {
		logger.Debugf("Not available: %s\n", resp.Status)
	}
	return manifest

}

// Show prints object meta data
func Show(token string, server string, options Options) {
	client := &http.Client{}
	url := []string{server, options.Bucket, options.File}
	logger.Debugf("Call %s\n", strings.Join(url, "/"))
	req, _ := http.NewRequest("HEAD", strings.Join(url, "/"), nil)
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("Failed to contact server %s\n", server)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		if k == "Content-Length" || k == "Last-Modified" {
			fmt.Printf("%s => %s\n", k, v[0])
		}
		if strings.HasPrefix(k, "X-Object-Meta-") {
			fmt.Printf("Metadata: %s => %s\n", strings.Replace(k, "X-Object-Meta-", "", -1), v[0])
		}

		if options.Bucket == "" {
			switch k {
			case "X-Account-Container-Count":
				fmt.Printf("Account container count: %s\n", v[0])
			case "X-Account-Object-Count":
				fmt.Printf("Account object count: %s\n", v[0])
			case "X-Account-Bytes-Used":
				fmt.Printf("Account bytes count: %s\n", v[0])
			case "X-Account-Meta-Quota-Bytes":
				fmt.Printf("Account quota bytes: %s\n", v[0])
			}
		} else if options.File == "" {
			switch k {
			case "X-Container-Object-Count":
				fmt.Printf("Container object count: %s\n", v[0])
			case "X-Container-Bytes-Used":
				fmt.Printf("Container bytes count: %s\n", v[0])
			case "X-Container-Meta-Quota-Bytes":
				fmt.Printf("Container quota bytes: %s\n", v[0])
			}
		}
	}

	/*
			Account meta:

		X-Account-Container-Count => 2
		X-Account-Storage-Policy-Policy-0-Object-Count => 7
		Content-Type => application/json; charset=utf-8
		X-Account-Object-Count => 7
		X-Account-Storage-Policy-Policy-0-Bytes-Used => 750
		X-Timestamp => 1515515566.01706
		Server => Apache
		X-Openstack-Request-Id => txe72f1f0f6cfc4ae388296-005c5b1650
		X-Account-Project-Domain-Id => 0de861d3d0fc43eabf692d6dbb1cc257
		X-Trans-Id => txe72f1f0f6cfc4ae388296-005c5b1650
		Date => Wed, 06 Feb 2019 17:16:01 GMT
		X-Account-Storage-Policy-Policy-0-Container-Count => 2
		X-Account-Bytes-Used => 750
		X-Account-Meta-Quota-Bytes => 107374182400

			Container meta:

		X-Container-Object-Count => 7
		X-Container-Meta-Access-Control-Allow-Origin => *
		X-Container-Bytes-Used => 750
		X-Trans-Id => tx1d45754156254aab98c87-005c5b16a4
		Server => Apache
		Accept-Ranges => bytes
		Last-Modified => Mon, 04 Feb 2019 16:03:15 GMT
		Last-Modified => Mon, 04 Feb 2019 16:03:15 GMT
		Content-Type => application/json; charset=utf-8
		X-Openstack-Request-Id => tx1d45754156254aab98c87-005c5b16a4
		X-Storage-Policy => Policy-0
		X-Container-Meta-Access-Control-Expose-Headers => Content-Length,X-Object-Manifest,X-Container-Bytes-Used,X-Container-Object-Count
		X-Timestamp => 1547482736.53607


	*/

}

// Upload uploads a file to swift
func Upload(token string, server string, options Options) bool {
	if options.ObjectName == "" {
		options.ObjectName = options.File
	}
	options.ObjectName = strings.TrimPrefix(options.ObjectName, "/")
	fmt.Printf("Upload: %s => %s\n", options.File, options.ObjectName)
	url := []string{server, options.Bucket, options.ObjectName}
	logger.Debugf("Call %s\n", strings.Join(url, "/"))
	fSize := fileSize(options.File)
	if fSize == 0 {
		fmt.Printf("File not found or empty\n")
		return false
	}

	// check if exists and was a x-object-manifest
	// if yes keep list and after upload, delete old segments
	// need to query files with prefix defined in manifest to delete them
	oldManifest := Head(token, server, options)

	if fSize > options.Size {
		nbSegment := int64(math.Floor(float64(fSize)/float64(options.Size)) + 1)
		start := int64(0)
		size := int64(0)
		ch := make(chan bool)
		uploadDone := int64(0)
		project := options.Bucket
		origFile := options.ObjectName
		options.Bucket = options.Bucket + "_segments"
		ts := time.Now().UnixNano()

		segmentPrefix := []string{options.Bucket, options.ObjectName, strconv.FormatInt(ts, 10), strconv.FormatInt(fSize, 10)}
		for i := int64(0); i < nbSegment; i++ {
			segmentSize := options.Size
			if i == nbSegment-1 {
				segmentSize = int64(fSize) - size // remaining
			}
			segment := Segment{From: start, Size: segmentSize}
			logger.Debugf("create segment %d: %d [%d]", i, segment.From, segment.Size)
			index := fmt.Sprintf("%010d", i)
			segmentFileName := []string{origFile, strconv.FormatInt(ts, 10), strconv.FormatInt(fSize, 10), index}
			newObjectName := strings.Join(segmentFileName, "/")
			options.ObjectName = newObjectName
			go uploadSegment(ch, token, server, options, segment)
			start += segmentSize
			size += options.Size
		}
		for uploadDone < nbSegment {
			uploadRes := <-ch
			if !uploadRes {
				fmt.Printf("Failed to upload file segment\n")
			} else {
				fmt.Println("Segment uploaded!")
			}
			uploadDone++
		}
		close(ch)
		options.Bucket = project
		options.ObjectName = origFile
		uploadManifest(token, server, strings.Join(segmentPrefix, "/"), options)

	} else {
		ch := make(chan bool)
		segment := Segment{From: 0, Size: fSize}
		go uploadSegment(ch, token, server, options, segment)
		uploadRes := <-ch
		if !uploadRes {
			fmt.Printf("Failed to upload file\n")
		} else {
			fmt.Println("Uploaded!")
		}
		close(ch)
	}

	if oldManifest != "" && options.LeaveSegments == false {
		logger.Debugf("Delete old segments")
		logger.Debugf("List with manifest prefix in _segments")
		options.Prefix = strings.Replace(oldManifest, options.Bucket+"_segments/", "", -1)
		options.Bucket = options.Bucket + "_segments"
		oldFiles := List(token, server, options)
		logger.Debugf("Delete old segment files")
		for _, file := range oldFiles {
			options.File = file.Name
			fmt.Printf("Delete segment %s, size: %d, last: %s\n", file.Name, file.Bytes, file.LastModified)
			DeleteFile(token, server, options)
		}
	}
	return true
}

// DeleteWithPrefix deletes all files matching prefix
func DeleteWithPrefix(token string, server string, options Options) {
	files := List(token, server, options)
	for _, file := range files {
		options.File = file.Name
		DeleteWithSegments(token, server, options)
	}
}

// DeleteWithSegments deletes a file and segments if any from swift
func DeleteWithSegments(token string, server string, options Options) {
	if options.ObjectName == "" {
		options.ObjectName = options.File
	}
	manifest := Head(token, server, options)
	bucket := options.Bucket
	prefix := options.Prefix
	file := options.File
	if manifest != "" && options.LeaveSegments == false {
		options.Prefix = strings.Replace(manifest, options.Bucket+"_segments/", "", -1)
		options.Bucket = options.Bucket + "_segments"
		oldFiles := List(token, server, options)
		logger.Debugf("Delete old segment files")
		for _, file := range oldFiles {
			options.File = file.Name
			fmt.Printf("Delete segment %s, size: %d, last: %s\n", file.Name, file.Bytes, file.LastModified)
			DeleteFile(token, server, options)
		}
	}
	options.Bucket = bucket
	options.Prefix = prefix
	options.File = file
	DeleteFile(token, server, options)
}

// Delete deletes a file from swift
func DeleteFile(token string, server string, options Options) bool {
	client := &http.Client{}
	if options.ObjectName == "" {
		options.ObjectName = options.File
	}
	url := []string{server, options.Bucket, options.File}
	logger.Debugf("Call %s\n", strings.Join(url, "/"))
	req, _ := http.NewRequest("DELETE", strings.Join(url, "/"), nil)
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("Failed to contact server %s\n", server)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 {
		logger.Errorf("Error: %s\n", resp.Status)
		return false
	}
	return true
}

// DownloadWithPrefix downloads all files matching prefix from swift
func DownloadWithPrefix(token string, server string, options Options) {
	files := List(token, server, options)
	objectName := options.ObjectName
	for i := range files {
		options.File = files[i].Name
		if options.ObjectName != "" {
			localPath := []string{objectName, options.File}
			options.ObjectName = strings.Join(localPath, "/")
		}
		fmt.Printf("Download %s => %s\n", options.File, options.ObjectName)
		Download(token, server, options)
	}
}

// Download downloads a file from swift
func Download(token string, server string, options Options) bool {
	client := &http.Client{}
	if options.ObjectName == "" {
		options.ObjectName = options.File
	}
	url := []string{server, options.Bucket, options.File}
	logger.Debugf("Call %s\n", strings.Join(url, "/"))
	req, _ := http.NewRequest("GET", strings.Join(url, "/"), nil)
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("Failed to contact server %s\n", server)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		logger.Errorf("Error: %s\n", resp.Status)
		return false
	}
	if resp.StatusCode == 204 {
		fmt.Printf("No content\n")
		return true
	}
	mkerr := os.MkdirAll(filepath.Dir(options.ObjectName), 0755)
	if mkerr != nil {
		logger.Errorf("Error: %s", mkerr)
		return false
	}
	out, err := os.Create(options.ObjectName)
	if err != nil {
		logger.Errorf("Error: %s", err)
		return false
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return true
}

// List list swift content
func List(token string, server string, options Options) []SwiftFile {
	var files []SwiftFile
	client := &http.Client{}
	url := []string{server, options.Bucket}
	logger.Debugf("Call %s\n", strings.Join(url, "/"))
	logger.Debugf("Prefix: %s", options.Prefix)
	req, _ := http.NewRequest("GET", strings.Join(url, "/"), nil)
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Accept", "application/json")
	if options.Prefix != "" {
		q := req.URL.Query()
		q.Add("prefix", options.Prefix)
		req.URL.RawQuery = q.Encode()
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("Failed to contact server %s\n", server)
		return files
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		logger.Errorf("Error: %s\n", resp.Status)
		return files
	}
	if resp.StatusCode == 204 {
		fmt.Printf("No content\n")
		return files
	}
	body, errBody := ioutil.ReadAll(resp.Body)
	if errBody != nil {
		logger.Errorf("Failed to read server response\n")
		return files
	}
	jerr := json.Unmarshal(body, &files)
	if jerr != nil {
		logger.Errorf("Failed to decode answer\n")
		return files
	}
	return files

}
