package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"github.com/blevesearch/bleve"
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
var batchSize = flag.Int("batchSize", 100, "batch size for indexing")
var dataIndex bleve.Index
var globDocId uint64

type jsonRecvQuery struct {
	QUERYSTRING string `json:"query"`
}

type DataInfo struct {
	ID         uint64    `json:"id"`
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
	ProductGroups []ProductGroup `xml:"productGroup"` // Viet Name Trade
	ListItems     []ListItem     `xml:"ListItems"`    // Alibaba
}

type ProductGroup struct {
	ProductGroupName string    `xml:"name,attr"`
	Products         []Product `xml:"product"`
}

type Product struct {
	HsCode string `xml:"hsCode"`
	Desc   string `xml:"productDesc"`
}

type ListItem struct {
	ListItemsType string `xml:"type,attr"`
	Items         []Item `xml:"Item"`
}

type Item struct {
	ImageURL string `xml:"ImageURL"`
	ItemName string `xml:"ItemName"`
	FOBPrice string `xml:"FOBPrice"`
}

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
					DATE:     time.Now(),
					CATEGORY: productGroups.ProductGroupName,
					HSCODE:   productItem.HsCode[0:6],
					PRODDESC: productItem.Desc,
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
					PICTURE:  item.ImageURL,
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
			DATE:       time.Now(),
			CATEGORY:   category,
			PRODDESC:   proddesc,
			PICTURE:    picture,
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
