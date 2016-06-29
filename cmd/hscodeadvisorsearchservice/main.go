//  Copyright (c) 2016 Dino Group, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package main

import (
	"flag"
	"github.com/blevesearch/bleve"
	"github.com/rs/cors"
	"log"
	"net/http"
	"os"
)

var xmlDir = flag.String("xmlDir", "data/", "xml directory")
var indexPath = flag.String("index", "hscode-search.bleve", "index path")
var batchSize = flag.Int("batchSize", 100, "batch size for indexing")
var dataIndex bleve.Index
var globDocId uint64

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		log.Printf("$PORT must be set")
		return
	}

	flag.Parse()

	// open the index
	var err error
	dataIndex, err = bleve.Open(*indexPath)
	if err == bleve.ErrorIndexPathDoesNotExist {
		log.Printf("Creating new index...")
		// create a mapping
		indexMapping, err := buildIndexMapping()
		if err != nil {
			log.Fatal(err)
		}
		dataIndex, err = bleve.New(*indexPath, indexMapping)
		if err != nil {
			log.Fatal(err)
		}

		globDocId = 1

		// index data in the background
		go func() {
			err = indexData(dataIndex)
			if err != nil {
				log.Fatal(err)
			}
		}()
	} else if err != nil {
		log.Fatal(err)
	} else {
		log.Printf("Opening existing index...")
		globDocId, err = dataIndex.DocCount()
		globDocId++
		if err != nil {
			log.Fatal(err)
		}
		// index data in the background
		go func() {
			err = indexData(dataIndex)
			if err != nil {
				log.Fatal(err)
			}
		}()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/search", searchIndex)

	// cors.Default() setup the middleware with default options being
	// all origins accepted with simple methods (GET, POST). See
	// documentation below for more options.
	handler := cors.Default().Handler(mux)
	http.ListenAndServe(":"+port, handler)
}
