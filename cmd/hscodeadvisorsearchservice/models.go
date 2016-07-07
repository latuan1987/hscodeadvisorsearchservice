//  Copyright (c) 2016 Dino Group, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package main

type jsonRecvQuery struct {
	QUERYSTRING string `json:"query"`
}

type DataInfo struct {
	ID         string `json:"id"`
	DATE       string `json:"Date"`
	CATEGORY   string `json:"Category"`
	PRODDESC   string `json:"ProductDescription"`
	PICTURE    string `json:"Picture"`
	HSCODE     string `json:"WCOHSCode"`
	COUNTRY    string `json:"Country"`
	TARIFFCODE string `json:"NationalTariffCode"`
	EXPLAIN    string `json:"ExplanationSheet"`
	VOTE       string `json:"Vote"`
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
