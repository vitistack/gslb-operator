package request

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/vitistack/gslb-operator/pkg/rest"
)

type Builder struct {
	host      string
	url       string
	urlParams url.Values
	method    string
	header    http.Header
	body      io.Reader
	ctx       context.Context
}

func NewBuilder(host string) *Builder {
	return &Builder{
		host:      host,
		body:      nil,
		urlParams: make(url.Values),
		header:    make(http.Header),
		method:    http.MethodGet, // default method
		ctx:       context.TODO(),
	}
}

func (b *Builder) Build() (*http.Request, error) {
	reqUrl := fmt.Sprintf("http://%s%s%s", b.host, b.url, b.urlParams.Encode()) // TODO: http/https?
	req, err := http.NewRequestWithContext(b.ctx, b.method, reqUrl, b.body)
	if err != nil {
		return nil, fmt.Errorf("unable to build request: %s", err.Error())
	}

	for key := range b.header {
		req.Header.Set(key, b.header.Get(key))
	}

	return req, nil
}

func (b *Builder) URL(url string) *Builder {
	b.url = url
	return b
}

func (b *Builder) WithURLParams(params any) *Builder {
	b.urlParams = UnMarshallParams(&params)
	return b
}

func (b *Builder) QueryParameter(key, val string) *Builder {
	b.urlParams.Add(key, val)
	return b
}

func (b *Builder) GET() *Builder {
	b.method = http.MethodGet
	return b
}

func (b *Builder) POST() *Builder {
	b.method = http.MethodPost
	return b
}

func (b *Builder) PUT() *Builder {
	b.method = http.MethodPut
	return b
}

func (b *Builder) DELETE() *Builder {
	b.method = http.MethodDelete
	return b
}

func (b *Builder) PATCH() *Builder {
	b.method = http.MethodPatch
	return b
}

func (b *Builder) HEAD() *Builder {
	b.method = http.MethodHead
	return b
}

func (b *Builder) OPTIONS() *Builder {
	b.method = http.MethodOptions
	return b
}

func (b *Builder) SetHeader(key, val string) *Builder {
	b.header.Set(key, val)
	return b
}

func (b *Builder) WithJSONContentType() *Builder {
	b.SetHeader("Content-Type", rest.ContentTypeJSON)
	return b
}

func (b *Builder) Body(body any) *Builder {
	//TODO: do this better, but for now json serialization works fine

	data, err := json.Marshal(body)
	if err != nil {
		b.body = nil
	}
	b.body = bytes.NewReader(data)
	b.WithJSONContentType()
	return b
}

func (b *Builder) CTX(ctx context.Context) *Builder {
	b.ctx = ctx
	return b
}
