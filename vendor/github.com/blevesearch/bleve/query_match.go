//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package bleve

import (
	"fmt"

	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/search"
)

type matchQuery struct {
	Match        string  `json:"match"`
	FieldVal     string  `json:"field,omitempty"`
	Analyzer     string  `json:"analyzer,omitempty"`
	BoostVal     float64 `json:"boost,omitempty"`
	PrefixVal    int     `json:"prefix_length"`
	FuzzinessVal int     `json:"fuzziness"`
}

// NewMatchQuery creates a Query for matching text.
// An Analyzer is chosen based on the field.
// Input text is analyzed using this analyzer.
// Token terms resulting from this analysis are
// used to perform term searches.  Result documents
// must satisfy at least one of these term searches.
func NewMatchQuery(match string) *matchQuery {
	return &matchQuery{
		Match:    match,
		BoostVal: 1.0,
	}
}

func (q *matchQuery) Boost() float64 {
	return q.BoostVal
}

func (q *matchQuery) SetBoost(b float64) Query {
	q.BoostVal = b
	return q
}

func (q *matchQuery) Field() string {
	return q.FieldVal
}

func (q *matchQuery) SetField(f string) Query {
	q.FieldVal = f
	return q
}

func (q *matchQuery) Fuzziness() int {
	return q.FuzzinessVal
}

func (q *matchQuery) SetFuzziness(f int) Query {
	q.FuzzinessVal = f
	return q
}

func (q *matchQuery) Prefix() int {
	return q.PrefixVal
}

func (q *matchQuery) SetPrefix(p int) Query {
	q.PrefixVal = p
	return q
}

func (q *matchQuery) Searcher(i index.IndexReader, m *IndexMapping, explain bool) (search.Searcher, error) {

	field := q.FieldVal
	if q.FieldVal == "" {
		field = m.DefaultField
	}

	analyzerName := ""
	if q.Analyzer != "" {
		analyzerName = q.Analyzer
	} else {
		analyzerName = m.analyzerNameForPath(field)
	}
	analyzer := m.analyzerNamed(analyzerName)

	if analyzer == nil {
		return nil, fmt.Errorf("no analyzer named '%s' registered", q.Analyzer)
	}

	tokens := analyzer.Analyze([]byte(q.Match))
	if len(tokens) > 0 {

		tqs := make([]Query, len(tokens))
		if q.FuzzinessVal != 0 {
			for i, token := range tokens {
				query := NewFuzzyQuery(string(token.Term))
				query.SetFuzziness(q.FuzzinessVal)
				query.SetPrefix(q.PrefixVal)
				query.SetField(field)
				query.SetBoost(q.BoostVal)
				tqs[i] = query
			}
		} else {
			for i, token := range tokens {
				tqs[i] = NewTermQuery(string(token.Term)).
					SetField(field).
					SetBoost(q.BoostVal)
			}
		}

		shouldQuery := NewDisjunctionQueryMin(tqs, 1).
			SetBoost(q.BoostVal)

		return shouldQuery.Searcher(i, m, explain)
	}
	noneQuery := NewMatchNoneQuery()
	return noneQuery.Searcher(i, m, explain)
}

func (q *matchQuery) Validate() error {
	return nil
}
