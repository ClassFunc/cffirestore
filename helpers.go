package cffirestore

import (
	"cloud.google.com/go/firestore"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/samber/lo"
	"reflect"
	"strings"
)

// helpers

func docSnapsDataToMap(docSnaps []*firestore.DocumentSnapshot) []map[string]any {
	var data = make([]map[string]any, 0)
	for _, doc := range docSnaps {
		data = append(data, makeDocResponse(doc))
	}
	return data
}

func makeDocResponse(doc *firestore.DocumentSnapshot) map[string]any {
	return lo.Assign(
		doc.Data(),
		map[string]any{
			"_id":  doc.Ref.ID,
			"_ref": doc.Ref.Path,
		},
	)
}

func structToMap(v any) map[string]any {
	var inInterface map[string]interface{}
	inrec, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	err = json.Unmarshal(inrec, &inInterface)
	if err != nil {
		return nil
	}
	return inInterface
}

func FilterDocs(docs []map[string]any, filter func(doc map[string]any) bool) []map[string]any {
	filtered := make([]map[string]any, 0)
	for _, doc := range docs {
		if filter(doc) {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

func TransformsDocs(docs []map[string]any, transform func(doc map[string]any) map[string]any) []map[string]any {
	transformed := make([]map[string]any, 0)
	for _, doc := range docs {
		transformed = append(transformed, transform(doc))
	}
	return transformed
}

// orderBy functions

var DefaultOrderByString = fmt.Sprintf("%s:%s", CreatedAtFieldName, "desc")

type OrderBy struct {
	Field     string
	Direction firestore.Direction
}

func parseOrderBy(orderBy string) *OrderBy {
	if lo.IsEmpty(orderBy) {
		return nil
	}
	orderBySlice := strings.Split(orderBy, ":")
	if len(orderBySlice) == 1 {
		orderBySlice = append(orderBySlice, "asc")
	}
	f := strings.TrimSpace(orderBySlice[0])
	switch strings.TrimSpace(strings.ToLower(orderBySlice[1])) {
	case "asc":
		return &OrderBy{f, firestore.Asc}
	case "desc":
		return &OrderBy{f, firestore.Desc}
	default:
		return &OrderBy{f, firestore.Asc}
	}
}

func deepCopyMap(src interface{}) interface{} {
	srcVal := reflect.ValueOf(src)

	switch srcVal.Kind() {
	case reflect.Map:
		dstMap := reflect.MakeMap(srcVal.Type())
		for _, key := range srcVal.MapKeys() {
			dstMap.SetMapIndex(key, reflect.ValueOf(deepCopyMap(srcVal.MapIndex(key).Interface())))
		}
		return dstMap.Interface()

	case reflect.Slice:
		dstSlice := reflect.MakeSlice(srcVal.Type(), srcVal.Len(), srcVal.Cap())
		reflect.Copy(dstSlice, srcVal)
		for i := 0; i < srcVal.Len(); i++ {
			dstSlice.Index(i).Set(reflect.ValueOf(deepCopyMap(srcVal.Index(i).Interface())))
		}
		return dstSlice.Interface()

	default:
		return src
	}
}

func debug(msg ...any) {
	color.Yellow("CFFIRESTORE DEBUG: %v", msg)
}
