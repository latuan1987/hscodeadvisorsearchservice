package main

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/blevesearch/bleve"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var xmlDir = flag.String("xmlDir", "data/", "xml directory")
var indexPath = flag.String("index", "hscode-search.bleve", "index path")
var db *sql.DB = nil
var dataIndex bleve.Index
var bDBExist bool = true

const (
    DB_USER     = "postgres"
    DB_PASSWORD = "tuandino"
    DB_NAME     = "postgres"
)

type DataInfo struct {
	ID         int64     `json:"id"`
	DATE       time.Time `json:"Date"`
	CATEGORY   string    `json:"Category"`
	PRODDESC   string    `json:"ProductDescription"`
	PICTURE    string    `json:"Picture"`
	HSCODE     string    `json:"WCOHSCode"`
	COUNTRY    string    `json:"Country"`
	TARIFFCODE string    `json:"NationalTariffCode"`
	EXPLAIN    string    `json:"ExplanationSheet"`
	VOTE       string    `json:"Vote"`
}

type ImportData struct {
	ProductGroups []ProductGroup `xml:"productGroup,omitempty"` // Viet Name Trade
	ListItems     []ListItem     `xml:"ListItems,omitempty"`    // Alibaba
}

type ProductGroup struct {
	ProductGroupName string    `xml:"name,attr"`
	Products         []Product `xml:"product,omitempty"`
}

type Product struct {
	HsCode string `xml:"hsCode,omitempty"`
	Desc   string `xml:"productDesc,omitempty"`
}

type ListItem struct {
	ListItemsType string	 `xml:"type,attr"`
	Items         []Item     `xml:"Item,omitempty"`
}

type Item struct {
	ImageURL string          `xml:"ImageURL,omitempty"`
	ItemName string          `xml:"ItemName,omitempty"`
	FOBPrice string          `xml:"FOBPrice,omitempty"`
	Detail   TechnicalDetail `xml:"TechnicalDetail,omitempty"`
}

type TechnicalDetail struct {
	ScreenSize    string `xml:"screensize"`
	Certification string `xml:"certification"`
}

func xmlParse(filePath string) ImportData {
	xmlFile, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error xmlParse: %q", err)
	}
	defer xmlFile.Close()

	b, _ := ioutil.ReadAll(xmlFile)
	var d ImportData
	xml.Unmarshal(b, &d)

	return d
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
		log.Fatalf("Error findAllFiles: %q", err)
	}

	return fileList
}

func indexData(i bleve.Index) error {

	// Get all xml file in specified folder
	var importDataList []ImportData
	listFiles := findAllFiles(*xmlDir)
	for _, file := range listFiles {
		// Parsing xml
		importDataList = append(importDataList, xmlParse(file))

		// Rename file
		extension := filepath.Ext(file)
		basename := file[0 : len(file)-len(extension)]
		os.Rename(file, basename+"_done.xml")
	}
	
	// walk the directory entries for indexing
	log.Printf("Indexing...")
	count := 0
	startTime := time.Now()

	// Insert data to table and make indexing
	for _, importDataItem := range importDataList {
		// Viet Name Trade data
		for _, productGroups := range importDataItem.ProductGroups {
			for _, productItem := range productGroups.Products {
				// Make data info
				dataInfo := DataInfo{
					CATEGORY: productGroups.ProductGroupName,
					HSCODE:   productItem.HsCode,
					PRODDESC: productItem.Desc}

				// Insert to data base
				var lastID int64
				if err := db.QueryRow("INSERT INTO Products (Category, ProductDescription, WCOHSCode) VALUES ($1,$2,$3) RETURNING ID", productGroups.ProductGroupName, productItem.Desc, productItem.HsCode).Scan(&lastID); err != nil {
					log.Fatal(err)
					return err
				}

				// Index
				if err := i.Index(strconv.FormatInt(lastID, 10), dataInfo); err != nil {
					log.Fatal(err)
					return err
				}
				
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
					CATEGORY: listItems.ListItemsType,
					PRODDESC: item.ItemName,
					PICTURE:  item.ImageURL}
					
				// Insert to data base
				var lastID int64
				if err := db.QueryRow("INSERT INTO Products (Category, ProductDescription, Picture) VALUES ($1,$2,$3) RETURNING ID", listItems.ListItemsType, item.ItemName, item.ImageURL).Scan(&lastID); err != nil {
					log.Fatal(err)
					return err
				}

				// Index
				if err := i.Index(strconv.FormatInt(lastID, 10), dataInfo); err != nil {
					log.Fatal(err)
					return err
				}
				
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
	
	indexDuration := time.Since(startTime)
	indexDurationSeconds := float64(indexDuration) / float64(time.Second)
	timePerDoc := float64(indexDuration) / float64(count)
	log.Printf("Indexed %d documents, in %.2fs (average %.2fms/doc)", count, indexDurationSeconds, timePerDoc/float64(time.Millisecond))

	return nil
}

func DBbuilder(rw http.ResponseWriter, req *http.Request) {
	var err error

	var CREATE_TABLE string = "CREATE TABLE IF NOT EXISTS Products (ID SERIAL PRIMARY KEY NOT NULL, Date timestamp DEFAULT CURRENT_TIMESTAMP, Category text, ProductDescription text, Picture text, WCOHSCode text, Country text, NationalTariffCode text, ExplanationSheet text, Vote text)"
	if _, err = db.Exec(CREATE_TABLE); err != nil {
		log.Fatalf("Error creating new table: %q", err)
		return
	}

	// Creating index
	if bDBExist == false {
		log.Printf("Creating new index...")
		// create a new mapping file and create a new index
		indexMapping := bleve.NewIndexMapping()
		dataIndex, err = bleve.New(*indexPath, indexMapping)
		if err != nil {
			log.Fatalf("Error creating index: %q", err)
			return
		}
		
		// Clear the table
		var DELETE_ROWS string = "DELETE FROM Products"
		if _, err = db.Exec(DELETE_ROWS); err != nil {
			log.Fatalf("Error deleting all rows of new table: %q", err)
			return
		}

		// index data in the background
		go func() {
			err = indexData(dataIndex)
			if err != nil {
				log.Fatal(err)
			}
		}()
	}
}

func searchIndex(rw http.ResponseWriter, req *http.Request) {
	if bDBExist == false {
		log.Println("Index null")
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("true"))
		return
	}
	
	// Get passed parameter
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Fatalf("Error readall: %q", err)
		return
	}
	
	log.Println(string(body))

	// Query data
	// We are looking to an product data with some string which match with dotGo
	query := bleve.NewMatchQuery(string(body))
	searchRequest := bleve.NewSearchRequest(query)
	searchResult, err := dataIndex.Search(searchRequest)
	if err != nil {
		log.Fatalf("Error readall: %q", err)
		return
	}

	log.Println(searchResult)
	if searchResult.Total < 0 {
		log.Fatalf("Error readall: %q", err)
		return
	}

	// Output data
	var resData []DataInfo
	for _, hit := range searchResult.Hits {
		// Write JSON data to response body
		log.Println(hit.ID)
		if id, err := strconv.ParseInt(hit.ID, 10, 64); err == nil {
			log.Println(hit.ID)
			// Query data
			rows, err := db.Query("SELECT * FROM Products WHERE ID=$1", id)
			if err != nil {
				log.Fatalf("Error readall: %q", err)
				return
			}

			for rows.Next() {
				var id int64
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
					log.Fatalf("Error readall: %q", err)
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
			// Close
			rows.Close()
		}
	}
	
	encoder, err := json.Marshal(resData)
	if err != nil {
		log.Fatalf("Error marshal: %q", err)
		return
	}
	
	// Write JSON data to response body
	rw.Header().Set("Content-Type", "application/json")
	rw.Write(encoder)
}

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
		return
	}

	flag.Parse()
	
	// Database connection
	var err error
	dbinfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable",
        DB_USER, DB_PASSWORD, DB_NAME)
	//db, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	db, err = sql.Open("postgres", dbinfo)
	if err != nil {
		log.Fatalf("Error opening database: %q", err)
		return
	}
	// open the index
	dataIndex, err = bleve.Open(*indexPath)
	if err == bleve.ErrorIndexPathDoesNotExist {
		log.Printf("Index not exist. Plese make new index by calling DBbuilder function")
		bDBExist = false
	}else if err != nil {
		log.Fatalf("Error opening data index: %q", err)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/search", searchIndex)
	mux.HandleFunc("/DBbuilder", DBbuilder)

	// cors.Default() setup the middleware with default options being
	// all origins accepted with simple methods (GET, POST). See
	// documentation below for more options.
	handler := cors.Default().Handler(mux)
	http.ListenAndServe(":"+port, handler)
}
