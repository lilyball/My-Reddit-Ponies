// Updates My Reddit Ponies to the latest version from
// http://userstyles.org/styles/49858/my-reddit-ponies

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/kballard/go-osx-plist"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

/*const userstylesURL = "http://userstyles.org/styles/49858/my-reddit-ponies"*/
const userscriptURL = "http://userstyles.org/styles/js/49858/My%20Reddit%20Ponies.js"
const cssURL = "http://userstyles.org/styles/operacss/49858/My%20Reddit%20Ponies.css"
const extensionFolder = "My Reddit Ponies.safariextension"
const updatePath = "update.plist"
const downloadFormatURL = "https://github.com/downloads/kballard/My-Reddit-Ponies/My-Reddit-Ponies-%s.safariextz"

var (
	poniesPath = filepath.Join(extensionFolder, "ponies.css")
	infoPath   = filepath.Join(extensionFolder, "Info.plist")
)

func main() {
	fmt.Println("Downloading CSS...")
	if err := downloadCSS(); err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Checking version number...")
	if version, err := fetchVersion(); err != nil {
		log.Fatalln(err)
	} else {
		fmt.Printf("Found version %s\n", version)
		if err := updatePlist(version); err != nil {
			log.Fatalln(err)
		}
	}
	fmt.Println("Complete")
}

func downloadCSS() error {
	resp, err := http.Get(cssURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("expected 200 OK, got %d", resp.StatusCode))
	}
	file, err := os.Create(poniesPath)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := NewEOLConvReader(resp.Body)
	_, err = io.Copy(file, reader)
	return err
}

func fetchVersion() (version string, err error) {
	// use the userscript URL because it has a better guarantee of stability of parsing
	resp, err := http.Get(userscriptURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New(fmt.Sprintf("expected 200 OK, got %d", resp.StatusCode))
	}
	reader := bufio.NewReader(resp.Body)
	wasPrefix := false
	line, isPrefix, err := reader.ReadLine()
	inMetadata := false
	for ; err == nil; line, isPrefix, err = reader.ReadLine() {
		// skip continuations of lines
		var b bool
		b, wasPrefix = wasPrefix, isPrefix
		if b {
			continue
		}

		if bytes.HasPrefix(line, []byte("//")) {
			if !inMetadata {
				if bytes.HasPrefix(line, []byte("// ==UserScript==")) {
					inMetadata = true
				}
				continue
			} else if bytes.HasPrefix(line, []byte("// ==/UserScript==")) {
				break
			}
		}
		fields := bytes.Fields(line)[1:] // skip the //
		if string(fields[0]) == "@description" {
			if string(fields[1]) == "Version" {
				return string(fields[2]), nil
			} else {
				return "", errors.New("Unexpected @description format")
			}
		}
	}
	if err != io.EOF {
		return "", err
	}
	return "", errors.New("couldn't find version")
}

func updatePlist(version string) error {
	// Write the Info.plist
	plistData, err := ioutil.ReadFile(infoPath)
	if err != nil {
		return err
	}
	var dict map[string]interface{}
	format, err := plist.Unmarshal(plistData, &dict)
	if err != nil {
		return err
	}
	dict["CFBundleShortVersionString"] = version
	dict["CFBundleVersion"] = version
	if plistData, err = plist.Marshal(dict, format); err != nil {
		return err
	}
	if err := ioutil.WriteFile(infoPath, plistData, 0644); err != nil {
		return err
	}

	// Write the update.plist
	plistData, err = ioutil.ReadFile(updatePath)
	if err != nil {
		return err
	}
	dict = nil
	format, err = plist.Unmarshal(plistData, &dict)
	if err != nil {
		return err
	}
	subdict := dict["Extension Updates"].([]interface{})[0].(map[string]interface{})
	subdict["CFBundleShortVersionString"] = version
	subdict["CFBundleVersion"] = version
	subdict["URL"] = fmt.Sprintf(downloadFormatURL, version)
	if plistData, err = plist.Marshal(dict, format); err != nil {
		return err
	}
	if err := ioutil.WriteFile(updatePath, plistData, 0644); err != nil {
		return err
	}
	return nil
}

// EOLConvReader is a Reader that swaps EOL to \n
type EOLConvReader struct {
	reader io.Reader
	skipNL bool
}

func NewEOLConvReader(r io.Reader) *EOLConvReader {
	return &EOLConvReader{reader: r}
}

func (r *EOLConvReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	for i := 0; i < n; i++ {
		c := p[i]
		if c == '\n' && r.skipNL {
			copy(p[i:], p[i+1:])
			i--
			n--
		}
		if c == '\r' {
			r.skipNL = true
			p[i] = '\n'
		}
	}
	return
}
