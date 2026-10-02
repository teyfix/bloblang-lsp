// Package fileuri converts local paths to editor file URIs and back.
package fileuri

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

func FromPath(path string) string {
	path = filepath.ToSlash(path)
	if len(path) >= 2 && path[1] == ':' {
		path = "/" + path
	}
	u := &url.URL{Scheme: "file", Path: path}
	if strings.HasPrefix(path, "//") {
		host, rest, _ := strings.Cut(path[2:], "/")
		u.Host, u.Path = host, "/"+rest
	}
	return u.String()
}

func Path(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	path := u.Path // url.Parse already unescapes the path.
	if runtime.GOOS == "windows" {
		if u.Host != "" && u.Host != "localhost" {
			path = "//" + u.Host + path
		} else if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}
	}
	return filepath.FromSlash(path), nil
}
