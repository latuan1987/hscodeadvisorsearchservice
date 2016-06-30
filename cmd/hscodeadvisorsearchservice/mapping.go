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
	"github.com/blevesearch/bleve"
	"log"
	"strconv"
	"time"
)

func buildIndexMapping() (*bleve.IndexMapping, error) {
	indexMapping := bleve.NewIndexMapping()

	return indexMapping, nil
}

func indexData(i bleve.Index) error {

	// Fetching all data from database
	var dataList []DataInfo
	var err error

	dataList, err = fetchAllFromProduct()
	if err != nil {
		return err
	}

	if len(dataList) == 0 {
		log.Println("Database empty")
		return nil
	}

	// walk the directory entries for indexing
	log.Printf("Indexing...")
	count := 0
	startTime := time.Now()
	batch := i.NewBatch()
	batchCount := 0

	// Insert data to table and make indexing
	for _, dataInfo := range dataList {
		// Index
		if err = batch.Index(strconv.FormatUint(dataInfo.ID, 10), dataInfo); err != nil {
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

		count++
		if count%1000 == 0 {
			indexDuration := time.Since(startTime)
			indexDurationSeconds := float64(indexDuration) / float64(time.Second)
			timePerDoc := float64(indexDuration) / float64(count)
			log.Printf("Indexed %d documents, in %.2fs (average %.2fms/doc)", count, indexDurationSeconds, timePerDoc/float64(time.Millisecond))
		}
	}
	// flush the last batch
	if batchCount > 0 {
		err = i.Batch(batch)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	indexDuration := time.Since(startTime)
	indexDurationSeconds := float64(indexDuration) / float64(time.Second)
	timePerDoc := float64(indexDuration) / float64(count)
	log.Printf("Indexed %d documents, in %.2fs (average %.2fms/doc)", count, indexDurationSeconds, timePerDoc/float64(time.Millisecond))

	return nil
}
