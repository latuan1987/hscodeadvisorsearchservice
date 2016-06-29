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
	"encoding/xml"
	"github.com/blevesearch/bleve"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func xmlParse(filePath string) (ImportData, error) {
	var d ImportData

	xmlFile, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error xmlParse: %q", err)
		return d, err
	}
	defer xmlFile.Close()

	b, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		log.Printf("Error reading data from xml file: %q", err)
		return d, err
	}
	err = xml.Unmarshal(b, &d)
	if err != nil {
		log.Printf("Error decoding xml data to json: %q", err)
		return d, err
	}

	return d, nil
}

func findAllFiles(searchDir string) []string {
	fileList := []string{}
	err := filepath.Walk(searchDir, func(path string, f os.FileInfo, err error) error {
		if (strings.Compare(filepath.Ext(path), ".xml") == 0) && (strings.Contains(path, "_done") == false) {
			fileList = append(fileList, path)
		}
		return nil
	})
	if err != nil {
		log.Printf("Error findAllFiles: %q", err)
	}

	return fileList
}

func buildIndexMapping() (*bleve.IndexMapping, error) {
	indexMapping := bleve.NewIndexMapping()

	return indexMapping, nil
}

func indexData(i bleve.Index) error {

	// Get all xml file in specified folder
	var importDataList []ImportData
	listFiles := findAllFiles(*xmlDir)
	for _, file := range listFiles {
		// Parsing xml
		importData, err := xmlParse(file)
		if err != nil {
			continue
		}

		// Add to slide
		importDataList = append(importDataList, importData)

		// Rename file
		extension := filepath.Ext(file)
		basename := file[0 : len(file)-len(extension)]
		os.Rename(file, basename+"_done.xml")
	}

	// walk the directory entries for indexing
	log.Printf("Indexing...")
	count := 0
	startTime := time.Now()
	batch := i.NewBatch()
	batchCount := 0
	var err error

	// Insert data to table and make indexing
	for _, importDataItem := range importDataList {
		// Viet Name Trade data
		for _, productGroups := range importDataItem.ProductGroups {
			for _, productItem := range productGroups.Products {
				// Make data info
				dataInfo := DataInfo{
					DATE:       time.Now(),
					CATEGORY:   productGroups.ProductGroupName,
					HSCODE:     productItem.HsCode[0:6],
					PRODDESC:   productItem.Desc,
					TARIFFCODE: productItem.HsCode,
				}

				// Index
				if err = batch.Index(strconv.FormatUint(globDocId, 10), dataInfo); err != nil {
					log.Println(err)
					return err
				}
				batchCount++

				if batchCount >= *batchSize {
					err = i.Batch(batch)
					if err != nil {
						return err
					}
					batch = i.NewBatch()
					batchCount = 0
				}

				globDocId++
				count++
				if count%1000 == 0 {
					indexDuration := time.Since(startTime)
					indexDurationSeconds := float64(indexDuration) / float64(time.Second)
					timePerDoc := float64(indexDuration) / float64(count)
					log.Printf("Indexed %d documents, in %.2fs (average %.2fms/doc)", count, indexDurationSeconds, timePerDoc/float64(time.Millisecond))
				}
			}
		}
		// Alibaba data
		for _, listItems := range importDataItem.ListItems {
			for _, item := range listItems.Items {
				// Make data info
				dataInfo := DataInfo{
					DATE:     time.Now(),
					CATEGORY: listItems.ListItemsType,
					PRODDESC: item.ItemName,
					PICTURE:  encodeImgUrlToBase64(item.ImageURL),
				}

				// Index
				if err = batch.Index(strconv.FormatUint(globDocId, 10), dataInfo); err != nil {
					log.Println(err)
					return err
				}
				batchCount++

				if batchCount >= *batchSize {
					err = i.Batch(batch)
					if err != nil {
						log.Println(err)
						return err
					}
					batch = i.NewBatch()
					batchCount = 0
				}

				globDocId++
				count++
				if count%1000 == 0 {
					indexDuration := time.Since(startTime)
					indexDurationSeconds := float64(indexDuration) / float64(time.Second)
					timePerDoc := float64(indexDuration) / float64(count)
					log.Printf("Indexed %d documents, in %.2fs (average %.2fms/doc)", count, indexDurationSeconds, timePerDoc/float64(time.Millisecond))
				}
			}
		}
	}
	// flush the last batch
	if batchCount > 0 {
		err = i.Batch(batch)
		if err != nil {
			log.Println(err)
		}
	}
	indexDuration := time.Since(startTime)
	indexDurationSeconds := float64(indexDuration) / float64(time.Second)
	timePerDoc := float64(indexDuration) / float64(count)
	log.Printf("Indexed %d documents, in %.2fs (average %.2fms/doc)", count, indexDurationSeconds, timePerDoc/float64(time.Millisecond))

	return nil
}

func encodeImgUrlToBase64(url string) string {
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
