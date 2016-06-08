package main

import (
	"database/sql"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/blevesearch/bleve"
	bleveHttp "github.com/blevesearch/bleve/http"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
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
var indexName = flag.String("indexName", "ProductData", "index name")
var db *sql.DB = nil

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
			}
		}
	}

	return nil
}

func searchIndex(c *gin.Context) {
	// Get passed parameter
	queryString := c.Query("query") // shortcut for c.Request.URL.Query().Get("query")

	// Get index
	index := bleveHttp.IndexByName(*indexName)
	if index == nil {
		log.Printf("index null!!!")
		return
	}

	// Query data
	// We are looking to an product data with some string which match with dotGo
	query := bleve.NewMatchQuery(queryString)
	searchRequest := bleve.NewSearchRequest(query)
	searchResult, err := index.Search(searchRequest)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Search: %q", err))
		return
	}

	if searchResult.Total < 0 {
		c.String(http.StatusOK, fmt.Sprintf("No data found!"))
		return
	}

	// Output data
	var resData []DataInfo
	for _, hit := range searchResult.Hits {
		// Write JSON data to response body
		if id, err := strconv.ParseInt(hit.ID, 10, 64); err == nil {
			// Query data
			rows, err := db.Query("SELECT * FROM Products WHERE ID=$1", id)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Query data: %q", err))
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
					c.String(http.StatusInternalServerError, fmt.Sprintf("Error scanning: %q", err))
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
	// Write JSON data to response body
	c.JSON(http.StatusOK, resData)
}

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
		return
	}

	flag.Parse()

	var err error

	db, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Error opening database: %q", err)
		return
	}

	var CREATE_TABLE string = "CREATE TABLE IF NOT EXISTS Products (ID SERIAL PRIMARY KEY NOT NULL, Date timestamp DEFAULT CURRENT_TIMESTAMP, Category text, ProductDescription text, Picture text, WCOHSCode text, Country text, NationalTariffCode text, ExplanationSheet text, Vote text)"
	if _, err := db.Exec(CREATE_TABLE); err != nil {
		log.Fatalf("Error creating new table: %q", err)
		return
	}

	// open the index
	dataIndex, err := bleve.Open(*indexPath)
	if err == bleve.ErrorIndexPathDoesNotExist {
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
		if _, err := db.Exec(DELETE_ROWS); err != nil {
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
	} else if err != nil {
		log.Fatal(err)
	} else {
		log.Printf("Opening existing index...")
		// index data in the background
		go func() {
			err = indexData(dataIndex)
			if err != nil {
				log.Fatal(err)
			}
		}()
	}

	router := gin.Default()

	// add the API
	bleveHttp.RegisterIndexName(*indexName, dataIndex)

	// Query string parameters are parsed using the existing underlying request object.
	// The request responds to a url matching:  /search?query=computer
	router.GET("/search", searchIndex)

	router.Run(":" + port)
}
