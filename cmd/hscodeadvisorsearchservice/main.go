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
	"database/sql"
	"flag"
	"github.com/blevesearch/bleve"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
	"log"
	"net/http"
	"os"
)

var xmlDir = flag.String("xmlDir", "data/", "xml directory")
var indexPath = flag.String("index", "hscode-search.bleve", "index path")
var batchSize = flag.Int("batchSize", 100, "batch size for indexing")

var db *sql.DB = nil
var dataIndex bleve.Index

const (
	DB_USER     = "postgres"
	DB_PASSWORD = "tuandino"
	DB_NAME     = "postgres"
)

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		log.Printf("$PORT must be set")
		return
	}

	flag.Parse()

	// Database connection
	var err error
	//dbinfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", DB_USER, DB_PASSWORD, DB_NAME)
	//db, err = sql.Open("postgres", dbinfo)
	db, err = sql.Open("postgres", "postgres://nlvgftvgexmgps:Vv8EoKoMOHjsYbtlcyjSjzGZkR@ec2-54-243-204-221.compute-1.amazonaws.com:5432/ddb7asusq3fjiu")
	if err != nil {
		log.Fatalf("Error opening database: %q", err)
	}

	// open the index
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

		// index data in the background
		go func() {
			err = indexData(dataIndex)
			if err != nil {
				log.Fatal(err)
			}
		}()
	} else if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/search", searchIndex)
	mux.HandleFunc("/dbbuilder", buildDbRequest)

	// cors.Default() setup the middleware with default options being
	// all origins accepted with simple methods (GET, POST). See
	// documentation below for more options.
	handler := cors.Default().Handler(mux)
	http.ListenAndServe(":"+port, handler)
}
