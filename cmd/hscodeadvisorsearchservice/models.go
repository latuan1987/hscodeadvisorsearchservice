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
	"time"
)

type jsonRecvQuery struct {
	QUERYSTRING string `json:"query"`
}

type DataInfo struct {
	ID         uint64    `json:"id"`
	DATE       time.Time `json:"Date,omitempty"`
	CATEGORY   string    `json:"Category,omitempty"`
	PRODDESC   string    `json:"ProductDescription,omitempty"`
	PICTURE    string    `json:"Picture,omitempty"`
	HSCODE     string    `json:"WCOHSCode,omitempty"`
	COUNTRY    string    `json:"Country,omitempty"`
	TARIFFCODE string    `json:"NationalTariffCode,omitempty"`
	EXPLAIN    string    `json:"ExplanationSheet,omitempty"`
	VOTE       string    `json:"Vote,omitempty"`
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
