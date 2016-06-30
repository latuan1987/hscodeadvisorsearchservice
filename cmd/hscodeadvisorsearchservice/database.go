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
	"encoding/xml"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var CREATE_TABLE string = `CREATE TABLE IF NOT EXISTS Products (ID SERIAL PRIMARY KEY NOT NULL,
																Date timestamp DEFAULT CURRENT_TIMESTAMP,
																Category text,
																ProductDescription text,
																Picture text,
																WCOHSCode text,
																Country text,
																NationalTariffCode text,
																ExplanationSheet text,
																Vote text)`
var DELETE_ROWS string = "DELETE FROM Products"
var FETCH_ALL string = "SELECT * FROM Products"
var INSERT_STMT string = `INSERT INTO Products (Category,
												ProductDescription,
												Picture,
												WCOHSCode,
												Country,
												NationalTariffCode,
												ExplanationSheet,
												Vote) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`

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

func buildDatabase() error {
	var err error

	// Creating table
	if _, err = db.Exec(CREATE_TABLE); err != nil {
		log.Printf("Error creating new table: %q", err)
		return err
	}

	// Clear the table
	if _, err = db.Exec(DELETE_ROWS); err != nil {
		log.Printf("Error deleting all rows of new table: %q", err)
		return err
	}

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
	log.Printf("Inserting...")
	stmt, err := db.Prepare(INSERT_STMT)
	if err != nil {
		log.Printf("Error preparing insertion statement: %q", err)
		return err
	}

	// Insert data to table
	for _, importDataItem := range importDataList {
		// Viet Name Trade data
		for _, productGroups := range importDataItem.ProductGroups {
			for _, productItem := range productGroups.Products {
				// Insert VietName Trade data
				_, err = stmt.Exec(productGroups.ProductGroupName, productItem.Desc, "", productItem.HsCode[0:6], "", productItem.HsCode, "", "")
				if err != nil {
					log.Printf("Error inserting: %q", err)
					return err
				}
			}
		}
		// Alibaba data
		for _, listItems := range importDataItem.ListItems {
			for _, item := range listItems.Items {
				// Insert Alibaba data
				_, err = stmt.Exec(listItems.ListItemsType, item.ItemName, item.ImageURL, "", "", "", "", "")
				if err != nil {
					log.Printf("Error inserting: %q", err)
					return err
				}
			}
		}
	}
	log.Println("Insertion completed")

	return nil
}

func fetchAllFromProduct() ([]DataInfo, error) {
	var resData []DataInfo
	var err error

	// Creating table
	if _, err = db.Exec(CREATE_TABLE); err != nil {
		log.Printf("Error creating new table: %q", err)
		return resData, err
	}

	rows, err := db.Query(FETCH_ALL)
	if err != nil {
		log.Printf("Error fetching all data: %q", err)
		return resData, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uint64
		var date time.Time
		var category sql.NullString
		var proddesc sql.NullString
		var picture sql.NullString
		var hscode sql.NullString
		var country sql.NullString
		var tariffcode sql.NullString
		var explain sql.NullString
		var vote sql.NullString

		if err := rows.Scan(&id, &date, &category, &proddesc, &picture, &hscode, &country, &tariffcode, &explain, &vote); err != nil {
			log.Printf("Error scaning rows: %q", err)
			break
		} else {
			dataInfo := DataInfo{
				ID:         id,
				DATE:       date,
				CATEGORY:   category.String,
				PRODDESC:   proddesc.String,
				PICTURE:    picture.String,
				HSCODE:     hscode.String,
				COUNTRY:    country.String,
				TARIFFCODE: tariffcode.String,
				EXPLAIN:    explain.String,
				VOTE:       vote.String,
			}
			// Add to array
			resData = append(resData, dataInfo)
		}
	}

	return resData, nil
}
