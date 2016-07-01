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
	b64 "encoding/base64"
	"encoding/json"
	"github.com/blevesearch/bleve"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

func searchIndex(rw http.ResponseWriter, req *http.Request) {
	// Get passed parameter
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("Error reading request data: %q", err)
		http.NotFound(rw, req)
		return
	}

	// Decode bytes to json data
	var recvQuery = jsonRecvQuery{}
	err = json.Unmarshal(body, &recvQuery)
	if err != nil {
		log.Printf("Error decoding request data: %q", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	// Query data
	// We are looking to an product data with some string which match with dotGo
	query := bleve.NewMatchPhraseQuery(recvQuery.QUERYSTRING)
	searchRequest := bleve.NewSearchRequestOptions(query, 100, 0, false)
	searchResult, err := dataIndex.Search(searchRequest)
	if err != nil {
		log.Printf("Error full text search: %q", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	if searchResult.Total < 0 {
		log.Printf("Total result < 0")
		return
	}

	// Output data
	var responseData []DataInfo
	for _, hit := range searchResult.Hits {
		if id, err := strconv.ParseUint(string(hit.ID), 10, 64); err == nil {
			queryData, err := queryDataByID(id)
			if err != nil {
				log.Println(err)
				continue
			}
			// Add to array
			responseData = append(responseData, queryData)
		} else {
			log.Println(err)
		}
	}

	encoder, err := json.Marshal(responseData)
	if err != nil {
		log.Printf("Error encoding respond data: %q", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write JSON data to response body
	rw.Header().Set("Content-Type", "application/json")
	rw.Write(encoder)
}

func buildDbRequest(rw http.ResponseWriter, req *http.Request) {
	err := buildDatabase()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("true"))
}

func encodeImgUrlToBase64(url string) string {
	if url == "" {
		return url
	}

	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return url
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return url
	}
	sEnc := b64.StdEncoding.EncodeToString([]byte(body))
	return sEnc
}
