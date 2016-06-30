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
	"time"
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
	var resData []DataInfo

	var id uint64
	var date time.Time
	var category string
	var proddesc string
	var picture string
	var hscode string
	var country string
	var tariffcode string
	var explain string
	var vote string

	for _, hit := range searchResult.Hits {
		doc, _ := dataIndex.Document(hit.ID)

		for _, field := range doc.Fields {
			switch name := field.Name(); name {
			case "id":
				id, _ = strconv.ParseUint(string(hit.ID), 10, 64)
			case "Date":
				date, err = time.Parse(time.RFC3339, string(field.Value()[:]))
				if err != nil {
					log.Println(err)
				}
			case "Category":
				category = string(field.Value()[:])
			case "ProductDescription":
				proddesc = string(field.Value()[:])
			case "Picture":
				picture = string(field.Value()[:])
			case "WCOHSCode":
				hscode = string(field.Value()[:])
			case "Country":
				country = string(field.Value()[:])
			case "NationalTariffCode":
				tariffcode = string(field.Value()[:])
			case "ExplanationSheet":
				explain = string(field.Value()[:])
			case "Vote":
				vote = string(field.Value()[:])
			default:
			}
		}
		// Write JSON data to response body
		dataInfo := DataInfo{
			ID:         id,
			DATE:       date,
			CATEGORY:   category,
			PRODDESC:   proddesc,
			PICTURE:    encodeImgUrlToBase64(picture),
			HSCODE:     hscode,
			COUNTRY:    country,
			TARIFFCODE: tariffcode,
			EXPLAIN:    explain,
			VOTE:       vote,
		}

		// Add to array
		resData = append(resData, dataInfo)
	}

	encoder, err := json.Marshal(resData)
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
