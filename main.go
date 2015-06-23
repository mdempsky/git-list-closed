// Copyright 2015 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
)

func must(out []byte, err error) []byte {
	if err != nil {
		log.Fatal(err)
	}
	return out
}

func run(cmd string, args ...string) ([]byte, error) {
	out, err := exec.Command(cmd, args...).Output()
	return out, err
}

func runLine(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).Output()
	return string(bytes.TrimSuffix(out, []byte("\n"))), err
}

func branches() []string {
	const prefix = "refs/heads/"
	out := must(run("git", "for-each-ref", "--format=%(refname)", prefix))
	var res []string
	for _, ref := range bytes.Split(out, []byte("\n")) {
		if len(ref) == 0 {
			continue
		}
		res = append(res, strings.TrimPrefix(string(ref), prefix))
	}
	return res
}

func getConfig(key string) (string, bool) {
	val, err := runLine("git", "config", "--get", key)
	if err, ok := err.(*exec.ExitError); ok && err.Sys().(syscall.WaitStatus).ExitStatus() == 1 {
		return "", false
	}
	if err != nil {
		log.Fatal(err)
	}
	return val, true
}

func issueURL(branch string) (string, bool) {
	server, ok := getConfig("branch." + branch + ".rietveldserver")
	if !ok {
		return "", false
	}
	issue, ok := getConfig("branch." + branch + ".rietveldissue")
	if !ok {
		return "", false
	}
	return server + "/api/" + issue, true
}

var client = &http.Client{
	CheckRedirect: func(req *http.Request, vias []*http.Request) error {
		return errors.New("redirect")
	},
}

func dump(branch string) {
	url, ok := issueURL(branch)
	if !ok {
		return
	}

	res, err := client.Get(url)
	if err != nil {
		log.Printf("error fetching %q for branch %q: %v", url, branch, err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Printf("unexpected status code fetching %q for branch %q: %v", url, branch, res.StatusCode)
		return
	}

	var x struct {
		Closed bool `json:"closed"`
	}
	if err := json.NewDecoder(res.Body).Decode(&x); err != nil {
		log.Printf("json decoding error for branch %q: %v", branch, err)
		return
	}
	if x.Closed {
		fmt.Println(branch)
	}
}

func main() {
	branches := branches()
	ch := make(chan bool)

	for _, branch := range branches {
		branch := branch
		go func() {
			dump(branch)
			ch <- true
		}()
	}

	for range branches {
		<-ch
	}
}
