/*
Copyright (c) 2018 Jack Lloyd. All rights reserved.

Permission to use, copy, modify, and/or distribute this software for
any purpose with or without fee is hereby granted, provided that the
above copyright notice and this permission notice appear in all
copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL
WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE
AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL
DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR
PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER
TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
PERFORMANCE OF THIS SOFTWARE.
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

func downloadFile(w http.ResponseWriter, r *http.Request, mirrorPath, cacheDir, ondiskLoc string) {
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

	tmpfile, err := ioutil.TempFile(cacheDir, filepath.Base(ondiskLoc))
	if err != nil {
		log.Println("Error creating temp file", err)
		http.Error(w, "Error creating temp file", 500)
		return
	}

	bytesWritten, err := io.Copy(tmpfile, tee)
	tmpfile.Close()

	if err != nil {
		os.Remove(tmpfile.Name())
		log.Println("Error reading data from mirror", err)
		// Already send 200 OK so can't change our minds now
		return
	}
	if bytesWritten != resp.ContentLength {
		os.Remove(tmpfile.Name())
		log.Printf("Bytes copied (%d) did not match upstream header (%d)\n", bytesWritten, resp.ContentLength)
		// Already send 200 OK so can't change our minds now
		return
	}

	err = os.MkdirAll(filepath.Dir(ondiskLoc), os.ModePerm)
	if err != nil {
		os.Remove(tmpfile.Name())
		log.Println("Error creating dir", err)
	}
	err = os.Rename(tmpfile.Name(), ondiskLoc)

	if err != nil {
		os.Remove(tmpfile.Name())
		log.Println("Error renaming file", err)
	}

}

func (config *Config) cacheHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm() // parse arguments, you have to call this by yourself

	path := filepath.Clean(r.URL.Path)

	if filepath.IsAbs(path) == false {
		http.Error(w, "Invalid path", 403)
		return
	}

	absPath := filepath.Join(config.cacheDir, path)

	_, err := os.Stat(absPath)
	if err != nil && os.IsNotExist(err) == false {
		log.Println("Failed to stat file", err)
		http.Error(w, "Something bad happened", 500)
		return
	}

	isDb := strings.HasSuffix(absPath, ".db")
	skipCache := strings.HasSuffix(absPath, ".sig") || isDb

	if err == nil && skipCache == false {
		// It exists, sendfile it
		log.Println("Cache hit on", path)
		http.ServeFile(w, r, absPath)
	} else if skipCache || os.IsNotExist(err) {
		// Need to download
		mirrorPath := fmt.Sprintf("%s%s", config.mirror, path)
		downloadFile(w, r, mirrorPath, config.cacheDir, absPath)
	}
}

func main() {
	serverPortFlag := flag.Int("port", 8080, "server port")
	cacheDir := flag.String("cache", "/tmp/flipper_cache", "cache directory")
	mirror := flag.String("upstream", "", "the upstream mirror to use")
	flag.Parse()

	if *mirror == "" {
		log.Fatal("Must specify upstream mirror")
	}

	config := Config{*cacheDir, *mirror}
	http.HandleFunc("/", config.cacheHandler)

	port := fmt.Sprintf(":%d", *serverPortFlag)

	http.ListenAndServe(port, nil)
}
