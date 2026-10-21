package main

import (
	_ "embed"
	"encoding/json"
	"sort"
)

//go:embed prototype-catalog.json
var catalogJSON []byte

type moduleCatalog struct {
	Groups     []string `json:"groups"`
	Categories []struct {
		Name  string `json:"name"`
		Group string `json:"group"`
	} `json:"categories"`
}

var catalog = func() map[string]moduleCatalog {
	var v map[string]moduleCatalog
	if err := json.Unmarshal(catalogJSON, &v); err != nil {
		panic(err)
	}
	return v
}()

func categoryGroup(key, name string) string {
	for _, v := range catalog[key].Categories {
		if v.Name == name {
			return v.Group
		}
	}
	return "其他"
}
func orderCategories(key string, names []string) {
	order := map[string]int{}
	for i, v := range catalog[key].Categories {
		order[v.Name] = i
	}
	rank := func(name string) int {
		if n, ok := order[name]; ok {
			return n
		}
		return len(order)
	}
	sort.SliceStable(names, func(i, j int) bool { return rank(names[i]) < rank(names[j]) })
}
func humanVisits(key string) bool { return key != "park" && key != "online" }
