package cffirestore

import (
	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"context"
	"errors"
	"fmt"
	"github.com/samber/lo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"reflect"
	"strings"
	"time"
)

var IdFieldName = "id"
var UidFieldName = "uid"
var CreatedAtFieldName = "createdAt"
var UpdatedAtFieldName = "updatedAt"
var DeletedAtFieldName = "deletedAt"

type ICFFSCollection interface {
	Ref() *firestore.CollectionRef
	AddDocData(v map[string]any, docIdPrefix ...string) (*firestore.DocumentRef, *firestore.WriteResult, error)
	AddDoc(uid *string, v map[string]any, docIdPrefix ...string) (*firestore.DocumentRef, *firestore.WriteResult, error)
	AddDocWithId(id *string, uid *string, v map[string]any) (*firestore.DocumentRef, *firestore.WriteResult, error)
	ListDocs(condition []any) ([]map[string]any, error)
	FindDoc(condition []any) (map[string]any, error)
	GetDoc(id string) (map[string]any, error)
	UpdateDoc(id string, data map[string]any) (*firestore.WriteResult, error)
	DeleteDoc(id string, isSoftDelete ...bool) (*firestore.WriteResult, error)
	DeleteDocs(condition []any, isSoftDelete ...bool) ([]*firestore.WriteResult, error)
	MakeQuery(condition []any) firestore.Query
	CountDocs(condition []any) (int, error)
	Paginate(condition []any, page int, perPage int) (map[string]any, error)
	PaginateWithCount(condition []any, page int, perPage int) (map[string]any, error)
	BatchDocs(condition []any, batchFn func(map[string]any) map[string]any) ([]*firestore.WriteResult, error)
	CheckExists(condition []any) (bool, error)
}

type Collection struct {
	Path   string
	Client *firestore.Client
	ref    *firestore.CollectionRef
}

func CollectionWithPath(client *firestore.Client, path string) *Collection {
	ref := client.Collection(path)
	return &Collection{
		Path:   path,
		Client: client,
		ref:    ref,
	}
}

func (coll *Collection) Ref() *firestore.CollectionRef {
	return coll.ref
}

func (coll *Collection) AddDocData(v map[string]any, docIdPrefix ...string) (*firestore.DocumentRef, *firestore.WriteResult, error) {
	return coll.AddDoc(nil, v, docIdPrefix...)
}

func (coll *Collection) AddDoc(uid *string, v map[string]any, docIdPrefix ...string) (*firestore.DocumentRef, *firestore.WriteResult, error) {
	ref := coll.ref.NewDoc()
	idPrefix := ""
	if len(docIdPrefix) > 0 {
		idPrefix = docIdPrefix[0]
	}
	id := fmt.Sprintf("%s%s", idPrefix, ref.ID)
	return coll.AddDocWithId(&id, uid, v)
}

func (coll *Collection) AddDocWithId(id *string, uid *string, v map[string]any) (*firestore.DocumentRef, *firestore.WriteResult, error) {
	if uid != nil {
		v[UidFieldName] = *uid
	}
	v[CreatedAtFieldName] = time.Now()
	v[UpdatedAtFieldName] = time.Now()
	v[DeletedAtFieldName] = nil

	ref := coll.ref.NewDoc()
	if id != nil {
		ref = coll.ref.Doc(*id)
		v[IdFieldName] = *id
	} else {
		v[IdFieldName] = ref.ID
	}

	result, err := ref.Set(context.Background(), v)
	if err != nil {
		return nil, nil, err
	}
	return ref, result, nil
}

func (coll *Collection) ListDocs(condition []any) ([]map[string]any, error) {
	query := coll.MakeQuery(condition)

	docs, err := query.Documents(context.Background()).GetAll()

	if err != nil {
		return nil, err
	}
	return docSnapsDataToMap(docs), nil

}

func (coll *Collection) FindDoc(condition []any) (map[string]any, error) {
	lastCond, err := lo.Last(condition)
	if err != nil {
		return nil, err
	}
	switch reflect.TypeOf(lastCond).Kind() {
	case reflect.Slice:
		//	append map[string]any{limit:1} condition
		condition = append(condition, map[string]any{
			"limit": 1,
		})
	case reflect.Map:
		//	append map[string]any{limit:1} condition
		lastCondMap := lastCond.(map[string]any)
		lastCondMap["limit"] = 1
		condition[len(condition)-1] = lastCondMap
	default:
	}
	docs, err := coll.ListDocs(condition)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, nil
	}
	return docs[0], nil
}

func (coll *Collection) GetDoc(id string) (map[string]any, error) {
	doc, err := coll.ref.Doc(id).Get(context.Background())
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errors.New(fmt.Sprintf("doc not found: %s", id))
		}
		return nil, err
	}

	return makeDocResponse(doc), nil
}

func (coll *Collection) UpdateDoc(id string, data map[string]any) (*firestore.WriteResult, error) {
	data[UpdatedAtFieldName] = time.Now()
	return coll.ref.Doc(id).Set(context.Background(), data, firestore.MergeAll)
}

func (coll *Collection) BatchDocs(condition []any, batchFn func(map[string]any) map[string]any) ([]*firestore.WriteResult, error) {
	docs, err := coll.ListDocs(condition)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, errors.New("no docs to batch")
	}

	errs := make([]error, 0)
	batchResults := make([]*firestore.WriteResult, 0)

	_500Docs := lo.Chunk(docs, 500)
	for _, docs := range _500Docs {
		results, err := batchEach500Docs(coll, docs, batchFn)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		batchResults = append(batchResults, results...)
	}

	return batchResults, errors.Join(errs...)
}
func makeUpdateData(oldDoc map[string]any, batchFn func(map[string]any) map[string]any) []firestore.Update {

	var afterDoc = deepCopyMap(oldDoc).(map[string]any)
	if batchFn != nil {
		afterDoc = batchFn(afterDoc)
	}
	updateData := make([]firestore.Update, 0)

	for key, oldVal := range oldDoc {
		if key == IdFieldName {
			continue
		}
		newVal := afterDoc[key]

		if newVal != oldVal {
			//debug.Info("changes", key, oldVal, newVal)
			updateData = append(
				updateData,
				firestore.Update{
					Path:  key,
					Value: newVal,
				},
			)
		}
	}
	return updateData
}
func batchEach500Docs(coll *Collection, docs []map[string]any, batchFn func(map[string]any) map[string]any) ([]*firestore.WriteResult, error) {
	if len(docs) == 0 {
		return nil, errors.New("no docs to batch")
	}
	docs = lo.Chunk(docs, 500)[0]
	errs := make([]error, 0)
	jobs := make([]*firestore.BulkWriterJob, 0)
	batch := coll.Client.BulkWriter(context.Background())

	for _, doc := range docs {
		docRef := coll.ref.Doc(doc[IdFieldName].(string))

		updateData := makeUpdateData(doc, batchFn)
		if len(updateData) == 0 {
			continue
		}

		//
		updateData = append(
			updateData,
			firestore.Update{
				Path:  UpdatedAtFieldName,
				Value: time.Now(),
			},
		)

		job, err := batch.Update(
			docRef,
			updateData,
		)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		jobs = append(jobs, job)
	}

	results := make([]*firestore.WriteResult, 0)
	for _, job := range jobs {
		r, err := job.Results()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		results = append(results, r)
	}

	return results, errors.Join(errs...)
}

func (coll *Collection) DeleteDoc(id string, isSoftDelete ...bool) (*firestore.WriteResult, error) {
	if len(isSoftDelete) > 0 && isSoftDelete[0] {
		return coll.UpdateDoc(id, map[string]any{
			DeletedAtFieldName: time.Now(),
		})
	}
	return coll.ref.Doc(id).Delete(context.Background())
}

func (coll *Collection) DeleteDocs(condition []any, isSoftDelete ...bool) ([]*firestore.WriteResult, error) {

	docs, err := coll.ListDocs(condition)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, errors.New("not found")
	}
	var softDelete bool = (len(isSoftDelete) > 0) && isSoftDelete[0]

	batch := coll.Client.BulkWriter(context.Background())

	jobs := make([]*firestore.BulkWriterJob, 0)
	errs := make([]error, 0)
	for _, doc := range docs {
		docId := doc[IdFieldName].(string)
		var job *firestore.BulkWriterJob
		var err error
		if !softDelete {
			job, err = batch.Delete(coll.ref.Doc(docId))
		} else {
			job, err = batch.Update(coll.ref.Doc(docId), []firestore.Update{
				{
					Path:  DeletedAtFieldName,
					Value: time.Now(),
				},
				{
					Path:  UpdatedAtFieldName,
					Value: time.Now(),
				}})
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		jobs = append(jobs, job)
	}

	results := make([]*firestore.WriteResult, 0)
	for _, job := range jobs {
		result, err := job.Results()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		results = append(results, result)
	}
	return results, errors.Join(errs...)

}

func (coll *Collection) MakeQuery(condition []any) firestore.Query {
	query := coll.ref.Query

	if DebugEnabled {
		debug(coll.ref.Path)
	}

	for idx, where := range condition {
		switch v := reflect.ValueOf(where); v.Kind() {
		case reflect.Slice:
			// v = []any{"path", "op", "val"}
			vSlide := v.Interface().([]any)
			path := vSlide[0].(string)
			op := vSlide[1].(string)
			val := vSlide[2]
			if DebugEnabled {
				debug(path, op, val)
			}

			query = query.Where(
				path,
				op,
				val,
			)
		case reflect.Map:
			vMap := v.Interface().(map[string]any)
			if DebugEnabled {
				debug(vMap)
			}
			if idx != len(condition)-1 {
				for key, val := range vMap {
					query = query.Where(key, "==", val)
				}
			} else {
				//	iter functions
				for key, val := range vMap {
					switch strings.ToLower(key) {
					case "orderby":
						// orderby = string | []string
						switch reflect.TypeOf(val).Kind() {
						case reflect.String:
							orderBy := parseOrderBy(val.(string))
							if orderBy != nil && len(orderBy.Field) > 0 {
								query = query.OrderBy(orderBy.Field, orderBy.Direction)
							}
						case reflect.Slice:
							obSlide := val.([]string)
							for _, ob := range obSlide {
								orderBy := parseOrderBy(ob)
								if orderBy != nil && len(orderBy.Field) > 0 {
									query = query.OrderBy(orderBy.Field, orderBy.Direction)
								}
							}
						default:
						}
					case "limit":
						query = query.Limit(val.(int))
					case "offset":
						query = query.Offset(val.(int))
					case "startat":
						query = query.StartAt(val)
					case "startafter":
						query = query.StartAfter(val)
					case "endat":
						query = query.EndAt(val)
					case "endbefore":
						query = query.EndBefore(val)
					}
				}
			}
		default:
			panic("unhandled default case")
		}
	}
	if DebugEnabled {
		debug("--------------------")
	}
	return query
}

func (coll *Collection) CountDocs(condition []any) (int, error) {

	//remove last condition if it is a map
	lastCond, err := lo.Last(condition)
	if err != nil {
		return 0, err
	}
	switch reflect.TypeOf(lastCond).Kind() {
	case reflect.Map:
		condition = condition[:len(condition)-1]
	default:
	}
	query := coll.MakeQuery(condition)

	aggregationQuery := query.NewAggregationQuery().WithCount("all")
	results, err := aggregationQuery.Get(context.Background())
	if err != nil {
		return 0, err
	}

	count, ok := results["all"]
	if !ok {
		return 0, errors.New("firestore: couldn't get alias for COUNT from results")
	}

	countValue := count.(*firestorepb.Value)
	return int(countValue.GetIntegerValue()), nil
}

var DefaultPaginatePerPage = 25

type PaginateQueryParams struct {
	Page    int    `query:"page" form:"page" json:"page"`
	PerPage int    `query:"perPage" form:"perPage" json:"perPage"`
	Sort    string `query:"sort" form:"sort" json:"sort"`
}

func (coll *Collection) Paginate(condition []any, page int, perPage int) (map[string]any, error) {
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = DefaultPaginatePerPage
	}
	lastCond := condition[len(condition)-1]
	switch reflect.TypeOf(lastCond).Kind() {
	case reflect.Slice:
		condition = append(condition, map[string]any{
			"limit":  perPage,
			"offset": (page - 1) * perPage,
		})
	case reflect.Map:
		lastCondMap := lastCond.(map[string]any)
		condition[len(condition)-1] = lo.Assign(
			lastCondMap,
			map[string]any{
				"limit":  perPage,
				"offset": (page - 1) * perPage,
			},
		)
	default:
	}

	docs, err := coll.ListDocs(condition)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"docs":    docs,
		"page":    page,
		"perPage": perPage,
	}

	return result, nil
}

func (coll *Collection) PaginateWithCount(condition []any, page int, perPage int) (map[string]any, error) {
	if perPage == 0 {
		perPage = DefaultPaginatePerPage
	}
	if page == 0 {
		page = 1
	}
	val, err := coll.Paginate(condition, page, perPage)
	if err != nil {
		return nil, err
	}

	count, err := coll.CountDocs(condition)
	if err != nil {
		return nil, err
	}
	totalPage := 0
	if count%perPage == 0 {
		totalPage = count / perPage
	} else {
		totalPage = count/perPage + 1
	}
	return lo.Assign(val, map[string]any{
		"count":     count,
		"totalPage": totalPage,
	}), nil
}

func (coll *Collection) CheckExists(condition []any) (bool, error) {
	docs, err := coll.ListDocs(condition)
	if err != nil {
		return false, err
	}
	return len(docs) > 0, nil
}
