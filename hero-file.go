package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/osallou/herodote-file/lib/keystone"
	logs "github.com/osallou/herodote-file/lib/log"
	swift "github.com/osallou/herodote-file/lib/swift"
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
	flag.StringVar(&objName, "object-name", "", "Upload/download as")
	flag.StringVar(&prefix, "prefix", "", "File prefix for search/delete/download")
	flag.Int64Var(&segmentSize, "segment-size", 1000000000, "Size of segments")
	flag.Var(&meta, "meta", "upload meta data with format key:value.")
	/*
			  --os-auth-url https://api.example.com/v3 \
		      --os-project-name project1 --os-project-domain-name domain1 \
		      --os-username user --os-user-domain-name domain1 \
		      --os-password password list

	*/
	flag.StringVar(&token, "os-auth-token", "", "Authentication token")
	flag.StringVar(&server, "os-storage-url", "", "Storage url https://api.example.com/v1/AUTH_XXX")
	flag.StringVar(&ksAuth.OsAuthURL, "os-auth-url", "", "Keystone auth url https://api.example.com/v3")
	flag.StringVar(&ksAuth.OsUserDomainName, "os-user-domain-name", "", "User domain name")
	flag.StringVar(&ksAuth.OsProjectDomainName, "os-project-domain-name", "", "Project domain name")
	flag.StringVar(&ksAuth.OsProjectName, "os-project-name", "", "Project name")
	flag.StringVar(&ksAuth.OsUserName, "os-username", "", "User name")
	flag.StringVar(&ksAuth.OsPassword, "os-password", "", "User password")
	var CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cmdHelp := `
Positional arguments:
	<subcommand>
		list		List content of a bucket
		stat		Show account/bucket/file metadata
		upload		Upload a file or directory to a bucket
		download	Download a file or list of files (prefix)
		delete		Delete a file or a list of files (prefix)

Examples:

  List content of *mybucket* bucket:
  hero-file --os-auth-url https://api.example.com/v3 --os-auth-token XXX list mybucket

  Download bucket file *data/myfile.txt* and save it locally with different name *myfile.out*
  hero-file --object-name myfile.out download mybucket data/myfile.txt

  Download all bucket files starting with *data*:
  hero-file --prefix data download mybucket

  Upload a local file *localfile.txt* to a bucket with remote name *data/localfile.txt*
  hero-file --object-name data/localfile.txt upload mybucket localfile.txt

  Delete a remote file:
  hero-file delete mybucket data/myfile.txt

  Delete all files with prefix *data*:
  hero-file --prefix data delete mybucket

  Delete all files:
  hero-file --prefix "**/*" delete mybucket

  Get bucket information:
  hero-file stat mybucket

  Get file information:
  hero-file stat mybucket data/myfile.txt
	`
	flag.Usage = func() {
		fmt.Fprintf(CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(CommandLine.Output(), "%s [options] <subcommand> <bucket> <file>\n", os.Args[0])
		fmt.Fprintf(CommandLine.Output(), "%s\n", cmdHelp)
		// TODO other commands
		fmt.Fprintf(CommandLine.Output(), "Optional arguments\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	tail := flag.Args()
	lenTail := len(tail)
	if lenTail == 0 {
		fmt.Printf("No command specified\n")
		flag.PrintDefaults()
		return
	}

	switch tail[0] {
	case "stat":
		stat = true
	case "upload":
		upload = true
	case "download":
		download = true
	case "delete":
		delete = true
	case "list":
		list = true
	}

	if lenTail > 1 {
		bucket = tail[1]
	}

	if lenTail > 2 {
		file = tail[2]
	}

	if lenTail > 4 && tail[3] == "--object-name" {
		objName = tail[4]
	}

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
		statInfo, err := swift.Show(token, server, options)
		if err != nil {
			fmt.Printf("An error occured: %s\n", err)
			return
		}
		for k, v := range statInfo {
			if k == "Content-Length" || k == "Last-Modified" {
				fmt.Printf("%s => %s\n", k, v)
			}
			if strings.HasPrefix(k, "X-Object-Meta-") {
				fmt.Printf("Metadata: %s => %s\n", strings.Replace(k, "X-Object-Meta-", "", -1), v)
			}
			switch k {
			case "X-Account-Container-Count":
				fmt.Printf("Account container count: %s\n", v)
			case "X-Account-Object-Count":
				fmt.Printf("Account object count: %s\n", v)
			case "X-Account-Bytes-Used":
				fmt.Printf("Account bytes count: %s\n", v)
			case "X-Account-Meta-Quota-Bytes":
				fmt.Printf("Account quota bytes: %s\n", v)
			case "X-Container-Object-Count":
				fmt.Printf("Container object count: %s\n", v)
			case "X-Container-Bytes-Used":
				fmt.Printf("Container bytes count: %s\n", v)
			case "X-Container-Meta-Quota-Bytes":
				fmt.Printf("Container quota bytes: %s\n", v)
			case "Etag":
				fmt.Printf("MD5 => %s\n", v)
			}
		}
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
