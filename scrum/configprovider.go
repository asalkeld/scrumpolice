package scrum

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/asalkeld/scrumpolice/common"
	"github.com/mitchellh/mapstructure"
	"github.com/nitrictech/go-sdk/api/documents"
	"github.com/nitrictech/go-sdk/faas"
	"github.com/nitrictech/go-sdk/resources"
)

type ConfigurationProvider interface {
	Config() *Config
	OnChange(handler func(cfg *Config))
	ReloadAndDistributeChange()
	PostHandler(ctx *faas.HttpContext, next faas.HttpHandler) (*faas.HttpContext, error)
	PutHandler(ctx *faas.HttpContext, next faas.HttpHandler) (*faas.HttpContext, error)
	GetHandler(ctx *faas.HttpContext, next faas.HttpHandler) (*faas.HttpContext, error)
	ListHandler(ctx *faas.HttpContext, next faas.HttpHandler) (*faas.HttpContext, error)
	DeleteHandler(ctx *faas.HttpContext, next faas.HttpHandler) (*faas.HttpContext, error)
}

type store struct {
	config         *Config
	changeHandlers []func(cfg *Config)
}

var (
	teamCol documents.CollectionRef
	_       ConfigurationProvider = &store{}
)

func NewConfig() ConfigurationProvider {
	cs := &store{}
	var err error
	teamCol, err = resources.NewCollection("team", resources.CollectionWriting, resources.CollectionReading, resources.CollectionDeleting)
	if err != nil {
		panic(err)
	}

	return cs
}

func (cs *store) Config() *Config {
	return cs.config
}

func (sc *store) OnChange(handler func(cfg *Config)) {
	sc.changeHandlers = append(sc.changeHandlers, handler)
}

func (sc *store) ReloadAndDistributeChange() {
	query := teamCol.Query()
	results, err := query.Fetch()
	if err != nil {
		log.Println(err)
		return
	}

	sc.config = &Config{Teams: []TeamConfig{}}
	for _, doc := range results.Documents {
		tc := &TeamConfig{}
		err := decodeWithJsonTags(doc, tc)
		if err != nil {
			log.Default().Println(err)
			continue
		}

		sc.config.Teams = append(sc.config.Teams, *tc)
	}

	for _, handler := range sc.changeHandlers {
		handler(sc.config)
	}
}

func decodeWithJsonTags(input interface{}, output interface{}) error {
	config := &mapstructure.DecoderConfig{
		Metadata:    nil,
		ErrorUnused: true,
		Result:      output,
		TagName:     "json",
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	err = decoder.Decode(input)
	if err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func (sc *store) PostHandler(ctx *faas.HttpContext, next faas.HttpHandler) (*faas.HttpContext, error) {
	store := &TeamConfig{}
	if err := json.Unmarshal(ctx.Request.Data(), store); err != nil {
		return common.HttpResponse(ctx, "error decoding json body", 400)
	}

	// Convert the document to a map[string]interface{}
	// for storage, future iterations of the go-sdk may include direct interface{} storage as well
	storeMap := make(map[string]interface{})

	err := decodeWithJsonTags(store, &storeMap)
	if err != nil {
		return common.HttpResponse(ctx, "error decoding store document :"+err.Error(), 400)
	}

	if err := teamCol.Doc(store.Name).Set(storeMap); err != nil {
		return common.HttpResponse(ctx, "error writing store document :"+err.Error(), 400)
	}

	common.HttpResponse(ctx, fmt.Sprintf("Created store with ID: %s", store.Name), 200)

	sc.config.Teams = append(sc.config.Teams, *store)
	sc.ReloadAndDistributeChange()

	return next(ctx)
}

func (sc *store) ListHandler(ctx *faas.HttpContext, next faas.HttpHandler) (*faas.HttpContext, error) {
	query := teamCol.Query()
	results, err := query.Fetch()
	if err != nil {
		return common.HttpResponse(ctx, "error querying collection: "+err.Error(), 500)
	}

	docs := make([]map[string]interface{}, 0)
	for _, doc := range results.Documents {
		docs = append(docs, doc.Content())
	}

	b, err := json.Marshal(docs)
	if err != nil {
		return common.HttpResponse(ctx, err.Error(), 400)
	}

	ctx.Response.Body = b
	ctx.Response.Headers["Content-Type"] = []string{"application/json"}

	return next(ctx)
}

func (sc *store) GetHandler(ctx *faas.HttpContext, next faas.HttpHandler) (*faas.HttpContext, error) {
	params := ctx.Request.PathParams()
	if len(params) == 0 {
		return common.HttpResponse(ctx, "error retrieving path params", 400)
	}

	id := params["name"]

	doc, err := teamCol.Doc(id).Get()
	if err != nil {
		common.HttpResponse(ctx, "error retrieving document "+id, 404)
	} else {
		b, err := json.Marshal(doc.Content())
		if err != nil {
			return common.HttpResponse(ctx, err.Error(), 400)
		}

		ctx.Response.Headers["Content-Type"] = []string{"application/json"}
		ctx.Response.Body = b
	}

	return next(ctx)
}

func (sc *store) PutHandler(ctx *faas.HttpContext, next faas.HttpHandler) (*faas.HttpContext, error) {
	params := ctx.Request.PathParams()
	if len(params) == 0 {
		return common.HttpResponse(ctx, "error retrieving path params", 400)
	}

	id := params["name"]

	_, err := teamCol.Doc(id).Get()
	if err != nil {
		ctx.Response.Body = []byte("Error retrieving document " + id)
		ctx.Response.Status = 404
	} else {
		store := &TeamConfig{}
		if err := json.Unmarshal(ctx.Request.Data(), store); err != nil {
			return common.HttpResponse(ctx, "error decoding json body", 400)
		}

		// Convert the document to a map[string]interface{}
		// for storage, future iterations of the go-sdk may include direct interface{} storage as well
		storeMap := make(map[string]interface{})
		err := decodeWithJsonTags(store, &storeMap)
		if err != nil {
			return common.HttpResponse(ctx, "error decoding store document", 400)
		}

		if err := teamCol.Doc(id).Set(storeMap); err != nil {
			return common.HttpResponse(ctx, "error writing store document:"+err.Error(), 400)
		}

		common.HttpResponse(ctx, fmt.Sprintf("Updated store with ID: %s", id), 200)
		for i, v := range sc.config.Teams {
			if v.Name == id {
				sc.config.Teams[i] = *store
			}
		}
		sc.ReloadAndDistributeChange()
	}

	return next(ctx)
}

func (sc *store) DeleteHandler(ctx *faas.HttpContext, next faas.HttpHandler) (*faas.HttpContext, error) {
	params := ctx.Request.PathParams()
	if len(params) == 0 {
		return common.HttpResponse(ctx, "error retrieving path params", 400)
	}

	id := params["name"]

	err := teamCol.Doc(id).Delete()
	if err != nil {
		return common.HttpResponse(ctx, "error deleting document "+id, 404)
	} else {
		ctx.Response.Status = 204
	}
	sc.ReloadAndDistributeChange()

	return next(ctx)
}
