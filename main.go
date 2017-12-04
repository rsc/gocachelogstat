// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gocachelogstat prints basic statistics about the go build cache.
// The goal is to inform the decision about cache expiration policy.
//
// Please run:
//
//	go get -u rsc.io/gocachelogstat
//	gocachelogstat
//
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type entry struct {
	created int64
	size    int64
	reused  bool
	data    *entry
}

func main() {
	log.SetPrefix("gocachelogstat:")
	log.SetFlags(0)

	out, err := exec.Command("go", "env", "GOCACHE").CombinedOutput()
	if err != nil {
		log.Fatalf("go env GOCACHE: %v\n%s", err, out)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		log.Fatalf("go env GOCACHE: no output (old Go version?)")
	}
	if dir == "off" {
		log.Fatalf("go env GOCACHE: GOCACHE=off")
	}

	data, err := ioutil.ReadFile(filepath.Join(dir, "log.txt"))
	if err != nil {
		log.Fatal(err)
	}

	var totalA, totalReusedA, totalD, totalReusedD int64

	var reuseA, reuseD []int
	var firstTime, lastTime int64
	cache := make(map[string]*entry)
	for _, line := range bytes.Split(data, []byte("\n")) {
		f := strings.Fields(string(line))
		if len(f) == 0 {
			continue
		}
		if len(f) < 3 || f[1] == "put" && len(f) != 5 {
			log.Fatalf("invalid log.txt line: %v", string(line))
		}
		t, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			log.Fatalf("invalid log.txt time: %v", string(line))
		}
		if firstTime == 0 {
			firstTime = t
		}
		lastTime = t
		switch f[1] {
		case "put":
			size, err := strconv.ParseInt(f[4], 10, 64)
			if err != nil {
				log.Fatalf("invalid log.txt size: %v", string(line))
			}
			e1 := cache[f[3]+"-d"]
			if e1 == nil {
				e1 = new(entry)
				e1.created = t
				e1.size = size
				cache[f[3]+"-d"] = e1
				totalD += size
			}
			e := cache[f[2]+"-a"]
			if e == nil {
				e = new(entry)
				e.created = t
				e.size = 154
				e.data = e1
				cache[f[2]+"-a"] = e
				totalA += 154
			}

		case "get", "miss":
			e := cache[f[2]+"-a"]
			if e == nil {
				continue
			}
			if !e.reused {
				e.reused = true
				totalReusedA += e.size
			}
			if !e.data.reused {
				e.data.reused = true
				totalReusedD += e.data.size
			}
			reuseA = append(reuseA, int(t-e.created))
			reuseD = append(reuseD, int(t-e.data.created))
		}
	}

	sort.Ints(reuseA)
	sort.Ints(reuseD)

	fmt.Printf("Please add the following output (including the quotes) to https://golang.org/issue/22990\n\n")
	fmt.Printf("```\n")
	defer fmt.Printf("```\n")

	fmt.Printf("cache age: %.2f days\n", float64(lastTime-firstTime)/86400)
	printCache("action", totalA, totalReusedA, reuseA)
	printCache("data", totalD, totalReusedD, reuseD)
}

func printCache(name string, total, totalReused int64, reuse []int) {
	fmt.Printf("%s cache: %d bytes, %d reused\n", name, total, totalReused)
	if len(reuse) == 0 {
		fmt.Printf("\tno reuse\n")
	} else {
		fmt.Printf("\treuse time percentiles\n")
		for i := 10; i <= 90; i += 10 {
			j := len(reuse) * i / 100
			fmt.Printf("\t%d%% %.2f days\n", i, float64(reuse[j])/86400)
		}
		fmt.Printf("\t95%% %.2f days\n", float64(reuse[len(reuse)*95/100])/86400)
		fmt.Printf("\t99%% %.2f days\n", float64(reuse[len(reuse)*99/100])/86400)
		fmt.Printf("\t99.9%% %.2f days\n", float64(reuse[len(reuse)*999/1000])/86400)
		fmt.Printf("\tmax %.2f days\n", float64(reuse[len(reuse)-1])/86400)
	}
}
