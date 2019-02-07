package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/osallou/hero-file/lib/keystone"
	logs "github.com/osallou/hero-file/lib/log"
	swift "github.com/osallou/hero-file/lib/swift"
)

var logger = logs.GetLogger("hero.cli")

type arrayFlags []string

func (i *arrayFlags) String() string {
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func dirExists(name string) bool {
	if fi, err := os.Stat(name); err == nil {
		if fi.Mode().IsDir() {
			return true
		}
	}
	return false
}

var Version string

func main() {
	var token string
	var server string
	var upload = false
	var download = false
	var delete = false
	var list = false
	var stat = false
	var file string
	var bucket string
	var objName string
	var segmentSize int64
	var prefix string
	var leaveSegments bool
	var meta arrayFlags
	var ksAuth = keystone.KeystoneAuth{}
	var helpVersion = false

	flag.BoolVar(&helpVersion, "version", false, "Show version")
	flag.BoolVar(&leaveSegments, "leaveSegments", false, "On file overwrite, do not delete old segment files")
	flag.BoolVar(&upload, "upload", false, "Upload file")
	flag.BoolVar(&download, "download", false, "Download file")
	flag.BoolVar(&stat, "stat", false, "Show stats on a file/bucket/account")
	flag.BoolVar(&delete, "delete", false, "Delete file")
	flag.StringVar(&bucket, "bucket", "", "bucket to use")
	flag.StringVar(&file, "file", "", "File to upload/download")
	flag.StringVar(&objName, "object-name", "", "Upload/download as")
	flag.StringVar(&prefix, "prefix", "", "File prefix for search")
	flag.BoolVar(&list, "list", false, "List files")
	flag.Int64Var(&segmentSize, "segment-size", 1000000000, "Size of segments")
	flag.Var(&meta, "meta", "upload meta data key:value.")
	/*
			  --os-auth-url https://api.example.com/v3 \
		      --os-project-name project1 --os-project-domain-name domain1 \
		      --os-username user --os-user-domain-name domain1 \
		      --os-password password list

	*/
	flag.StringVar(&token, "os-auth-token", "", "Authentication token")
	flag.StringVar(&server, "os-storage-url", "", "Storage url https://genostack-api-swift.genouest.org/v1/AUTH_XXX")
	flag.StringVar(&ksAuth.OsAuthURL, "os-auth-url", "", "Keystone auth url https://api.example.com/v3")
	flag.StringVar(&ksAuth.OsUserDomainName, "os-user-domain-name", "", "User domain name")
	flag.StringVar(&ksAuth.OsProjectDomainName, "os-project-domain-name", "", "Project domain name")
	flag.StringVar(&ksAuth.OsProjectName, "os-project-name", "", "Project name")
	flag.StringVar(&ksAuth.OsUserName, "os-username", "", "User name")
	flag.StringVar(&ksAuth.OsPassword, "os-password", "", "User password")

	flag.Parse()

	if helpVersion {
		fmt.Printf("Version: %s\n", Version)
		return
	}

	// keystone env variables
	// OS_AUTH_URL, OS_USER_DOMAIN_NAME, OS_PROJECT_NAME, OS_USERNAME, OS_PASSWORD

	if token == "" {
		if os.Getenv("OS_AUTH_URL") != "" {
			ksAuth.OsAuthURL = os.Getenv("OS_AUTH_URL")
		}
		if os.Getenv("OS_USER_DOMAIN_ID") != "" {
			ksAuth.OsUserDomainID = os.Getenv("OS_USER_DOMAIN_ID")
		}
		if os.Getenv("OS_USER_DOMAIN_NAME") != "" {
			ksAuth.OsUserDomainName = os.Getenv("OS_USER_DOMAIN_NAME")
		}
		if os.Getenv("OS_PROJECT_DOMAIN_ID") != "" {
			ksAuth.OsProjectDomainID = os.Getenv("OS_PROJECT_DOMAIN_ID")
		}
		if os.Getenv("OS_PROJECT_DOMAIN_NAME") != "" {
			ksAuth.OsProjectDomainName = os.Getenv("OS_PROJECT_DOMAIN_NAME")
		}
		if os.Getenv("OS_PROJECT_NAME") != "" {
			ksAuth.OsProjectName = os.Getenv("OS_PROJECT_NAME")
		}
		if os.Getenv("OS_USERNAME") != "" {
			ksAuth.OsUserName = os.Getenv("OS_USERNAME")
		}
		if os.Getenv("OS_PASSWORD") != "" {
			ksAuth.OsPassword = os.Getenv("OS_PASSWORD")
		}
		var endpoint string
		token, endpoint = keystone.Auth(ksAuth)
		if token == "" {
			fmt.Printf("No os-auth-token given and failed to authenticate against keystone")
			return
		}
		if server == "" {
			logger.Debugf("no server defined, guess from keystone: %s\n", endpoint)
			server = endpoint
		}
	}

	var envToken = os.Getenv("HEROTOKEN")
	if envToken != "" {
		token = envToken
	}

	metaData := make(map[string]string)
	for m := range meta {
		kv := strings.Split(meta[m], ":")
		if len(kv) != 2 {
			continue
		}
		metaData[kv[0]] = kv[1]
		fmt.Printf("Meta %s: %s\n", kv[0], kv[1])
	}

	var options = swift.Options{
		Bucket:        bucket,
		File:          file,
		ObjectName:    objName,
		Size:          segmentSize,
		Prefix:        prefix,
		LeaveSegments: leaveSegments,
		Meta:          metaData}

	if upload {
		if bucket == "" {
			fmt.Printf("Bucket is missing\n")
			return
		}

		if file == "" {
			fmt.Printf("file option is missing")
			return
		}
		if dirExists(options.File) {
			// this is a directory upload
			// Loop over files
			err := filepath.Walk(options.File, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					fmt.Printf("failed to access path %q: %v\n", path, err)
					return err
				}
				if info.IsDir() {
					fmt.Printf("Look in dir: %+v \n", info.Name())
					return nil
				}
				subObjectName := path
				if options.ObjectName != "" {
					old := options.File
					new := strings.TrimPrefix(options.ObjectName, "/")
					subObjectName = strings.Replace(path, old, new, -1)
				}
				var subOptions = swift.Options{Bucket: bucket, File: path, ObjectName: subObjectName, Size: segmentSize, Prefix: prefix, LeaveSegments: leaveSegments, Meta: metaData}
				swift.Upload(token, server, subOptions)
				return nil
			})
			if err != nil {
				fmt.Printf("Got an error: %v", err)
				return
			}

		} else {
			swift.Upload(token, server, options)
		}
	} else if download {
		if bucket == "" {
			fmt.Printf("Bucket is missing\n")
			return
		}
		if prefix != "" {
			swift.DownloadWithPrefix(token, server, options)
		} else {
			swift.Download(token, server, options)
		}
	} else if delete {
		if prefix != "" {
			swift.DeleteWithPrefix(token, server, options)
		} else {
			if file == "" {
				fmt.Printf("file option is missing")
				return
			}
			swift.DeleteWithSegments(token, server, options)
		}
	} else if stat {
		swift.Show(token, server, options)
	} else if list {
		options.File = ""
		options.ObjectName = ""
		files := swift.List(token, server, options)
		for _, file := range files {
			fmt.Printf("%s, size: %d, last: %s\n", file.Name, file.Bytes, file.LastModified)
		}
	} else {
		fmt.Printf("No operation selected\n")
	}
}
