/*
Copyright (c) 2018 Jack Lloyd. All rights reserved.

ISC license
*/

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	cacheDir string
	mirror   string
}

func (config *Config) cacheHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm() // parse arguments, you have to call this by yourself

	path := filepath.Clean(r.URL.Path)

	if filepath.IsAbs(path) == false {
		http.Error(w, "Invalid path", 403)
		return
	}

	absPath := filepath.Join(config.cacheDir, path)

	// The nginx config does this, but why?
	skipCache := strings.HasSuffix(absPath, ".sig")
	_, err := os.Stat(absPath)

	if err == nil && skipCache == false {
		// It exists!
		log.Println("Cache hit on", path)
		contents, err := ioutil.ReadFile(absPath)
		if err == nil {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(contents)
		} else {
			log.Println("Error reading file", err)
			http.Error(w, "Error reading file", 404)
			return
		}
	} else if os.IsNotExist(err) {
		// Need to download

		mirrorPath := fmt.Sprintf("%s%s", config.mirror, path)

		log.Printf("Downloading %s", mirrorPath)
		resp, err := http.Get(mirrorPath)

		if err != nil {
			log.Println("Error reading ", mirrorPath, " from mirror: ", err)
			http.Error(w, "Error reading from mirror", 500)
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			log.Printf("Error reading %s from mirror, status code %d status '%s'\n", mirrorPath, resp.StatusCode, resp.Status)
			http.Error(w, resp.Status, resp.StatusCode)
			return
		}

		log.Printf("Success reading from %s\n", mirrorPath)
		w.Header().Set("Content-Type", "application/octet-stream")
		if resp.ContentLength >= 0 {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
		}
		tee := io.TeeReader(resp.Body, w)
		// FIXME this keeps entire thing in memory, lame on small machines
		// CUDA package is over a GB
		contents, err := ioutil.ReadAll(tee)
		if err != nil {
			log.Println("Error reading data from mirror", err)
			http.Error(w, "Error reading data from mirror", 500)
			return
		}
		tmpfile, err := ioutil.TempFile(config.cacheDir, filepath.Base(path))
		if err != nil {
			log.Println("Error creating temp file", err)
			http.Error(w, "Error creating temp file", 500)
			return
		}
		tmpfile.Write(contents)
		tmpfile.Close()

		err = os.MkdirAll(filepath.Dir(absPath), os.ModePerm)
		if err != nil {
			os.Remove(tmpfile.Name())
			log.Println("Error creating dir", err)
		}
		err = os.Rename(tmpfile.Name(), absPath)

		if err != nil {
			os.Remove(tmpfile.Name())
			log.Println("Error renaming file", err)
		}

	} else {
		log.Println("Something bad happened", err)
		http.Error(w, "Some error occured", 500)
		return
	}
}

func main() {
	serverPortFlag := flag.Int("port", 8080, "server port")
	cacheDir := flag.String("cache", "/tmp/flipper_cache", "cache directory")
	mirror := flag.String("upstream", "", "the upstream mirror to use")
	flag.Parse()

	config := Config{*cacheDir, *mirror}
	http.HandleFunc("/", config.cacheHandler)

	port := fmt.Sprintf(":%d", *serverPortFlag)

	http.ListenAndServe(port, nil)
}
