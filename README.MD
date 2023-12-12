## CFFIRESTORE

Helpers for Firestore

### Installation

```bash
go get github.com/classfunc/cffirestore
```

### Usage

```go
package main

import (
	"github.com/classfunc/cffirestore"
)

func main() {
	// Initialize a Firestore client
	// firebaseApp *firebase.App
	// fsClient *firestore.Client
	
	fsClient := firebaseApp.Firestore(context.Background())
	myCollection := cffirestore.CollectionWithPath(fsClient, "myCollection")
	
	// add doc
	myCollection.AddDocData(map[string]any{"name": "John Doe"})
	
	// list docs
	docs, err := myCollection.ListDocs([]any{[]any{"name", "==", "John Doe"}})
	
	// check exists
	exists, err := myCollection.CheckExists([]any{
		[]any{"deletedAt", "==", nil},
		[]any{"name", "==", "John Doe"},
	})
	
	//paginate
	// query /?perPage=3&page=1&sort=createdAt:desc
	var q cffirestore.PaginateQueryParams
	//var ctx *gin.Context
	_ = ctx.BindQuery(&q)
	keys, err := myCollection.PaginateWithCount([]any{
		[]any{"deletedAt", "==", nil},
		[]any{"uid", "==", "abc"},
		map[string]any{
			"orderBy": q.Sort,
		},
	}, q.Page, q.PerPage)
}

```

#### ICFFSCollection Interface
The ICFFSCollection interface in Go provides a set of methods for interacting with a database collection. This abstraction is great because it allows your code to be decoupled from a specific implementation of accessing a database collection.
```go
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
```

#### Method descriptions
- Ref(): returns a reference to the Firestore collection.
- AddDocData(v, docIdPrefix): adds a document to the collection with optional docIdPrefix.
- AddDoc(uid, v, docIdPrefix): adds a document to the collection under a user ID uid, an optional docIdPrefix can also be provided.
- AddDocWithId(id, uid, v): adds a document to the collection with a specific document id and user uid.
- ListDocs(condition): takes an array condition and lists all documents that meet the condition in the database collection.
- FindDoc(condition): takes an array condition and returns the first document that meets the condition.
- GetDoc(id): takes a document id and returns the document with given id.
- UpdateDoc(id, data): takes a document id and data mapping, updates the document with the provided data.
- DeleteDoc(id, isSoftDelete): deletes a document with optional soft delete.
- DeleteDocs(condition, isSoftDelete): deletes documents that meet the condition with optional soft delete.
- MakeQuery(condition): makes a database query according to condition.
- CountDocs(condition): counts the number of documents that meet the condition.
- Paginate(condition, page, perPage): paginates the document entries that meet the condition, each page contains perPage entries.
- PaginateWithCount(condition, page, perPage): is similar to Paginate, but it also returns the total count of documents that meet the condition.
- BatchDocs(condition, batchFn): takes a batch function batchFn and applies it to all documents that meet the condition.
- CheckExists(condition): checks whether any document exists that meet the condition.

NOTE: The condition parameter is an array which could be a composite condition based on more than one field of the documents in the database. 

This interface is heavily dependent on Firestore's types and methods, a Firestore-specific implementation needs to be created for use. Any function that implements this interface can then interact with a Firestore database collection.

