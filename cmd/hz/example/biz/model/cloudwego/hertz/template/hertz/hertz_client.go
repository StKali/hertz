package hertz

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	hertz_client "github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/errors"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/client"
)

type use interface {
	Use(mws ...hertz_client.Middleware)
}

// Definition of global data and types.
type ResponseResultDecider func(statusCode int, rawResponse *protocol.Response) (isError bool)
type bindRequestBodyFunc func(c *cli, r *request) (contentType string, body io.Reader, err error)
type beforeRequestFunc func(*cli, *request) error
type afterResponseFunc func(*cli, *response) error

var (
	hdrContentTypeKey     = http.CanonicalHeaderKey("Content-Type")
	hdrContentEncodingKey = http.CanonicalHeaderKey("Content-Encoding")

	plainTextType   = "text/plain; charset=utf-8"
	jsonContentType = "application/json; charset=utf-8"

	jsonCheck = regexp.MustCompile(`(?i:(application|text)/(json|.*\+json|json\-.*)(; |$))`)
	xmlCheck  = regexp.MustCompile(`(?i:(application|text)/(xml|.*\+xml)(; |$))`)
)

// Configuration of client
type Option struct {
	f func(*Options)
}

type Options struct {
	hostUrl               string
	doer                  client.Doer
	header                http.Header
	requestBodyBind       bindRequestBodyFunc
	responseResultDecider ResponseResultDecider
	middlewares           []hertz_client.Middleware
	clientOption          []config.ClientOption
}

func getOptions(ops ...Option) *Options {
	opts := &Options{}
	for _, do := range ops {
		do.f(opts)
	}
	return opts
}

// 注入client选项，会使用这些选项来创建一个默认的client
func WithHertzClientOption(opt ...config.ClientOption) Option {
	return Option{func(op *Options) {
		op.clientOption = append(op.clientOption, opt...)
	}}
}

// 注入中间件
func WithHertzClientMiddleware(mws ...hertz_client.Middleware) Option {
	return Option{func(op *Options) {
		op.middlewares = append(op.middlewares, mws...)
	}}
}

// 支持 hertz client
func WithHertzClient(client client.Doer) Option {
	return Option{func(op *Options) {
		op.doer = client
	}}
}

// 添加默认的 header, 相比于在结构体中的header，这个header会默认每个请求都发，便于做一些校验
func WithHeader(header http.Header) Option {
	return Option{func(op *Options) {
		op.header = header
	}}
}

// 自定义反序列化http响应到响应结构体的方法
func WithResponseResultDecider(decider ResponseResultDecider) Option {
	return Option{func(op *Options) {
		op.responseResultDecider = decider
	}}
}

func withHostUrl(HostUrl string) Option {
	return Option{func(op *Options) {
		op.hostUrl = HostUrl
	}}
}

// underlying client
type cli struct {
	hostUrl               string
	doer                  client.Doer
	header                http.Header
	bindRequestBody       bindRequestBodyFunc
	responseResultDecider ResponseResultDecider

	beforeRequest []beforeRequestFunc
	afterResponse []afterResponseFunc
}

func (c *cli) Use(mws ...hertz_client.Middleware) error {
	u, ok := c.doer.(use)
	if !ok {
		return errors.NewPublic("doer does not support middleware, choose the right doer.")
	}
	u.Use(mws...)
	return nil
}

func newClient(opts *Options) (*cli, error) {
	if opts.requestBodyBind == nil {
		opts.requestBodyBind = defaultRequestBodyBind
	}
	if opts.responseResultDecider == nil {
		opts.responseResultDecider = defaultResponseResultDecider
	}
	if opts.doer == nil {
		cli, err := hertz_client.NewClient(opts.clientOption...)
		if err != nil {
			return nil, err
		}
		opts.doer = cli
	}

	c := &cli{
		hostUrl:               opts.hostUrl,
		doer:                  opts.doer,
		bindRequestBody:       opts.requestBodyBind,
		responseResultDecider: opts.responseResultDecider,
		beforeRequest: []beforeRequestFunc{
			parseRequestURL,
			parseRequestHeader,
			createHTTPRequest,
		},
		afterResponse: []afterResponseFunc{
			parseResponseBody,
		},
	}

	if len(opts.middlewares) != 0 {
		if err := c.Use(opts.middlewares...); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *cli) execute(req *request) (*response, error) {
	var err error
	for _, f := range c.beforeRequest {
		if err = f(c, req); err != nil {
			return nil, err
		}
	}

	if hostHeader := req.header.Get("Host"); hostHeader != "" {
		req.rawRequest.Header.SetHost(hostHeader)
	}

	resp := protocol.Response{}

	err = c.doer.Do(req.ctx, req.rawRequest, &resp)

	response := &response{
		request:     req,
		rawResponse: &resp,
	}

	if err != nil {
		return response, err
	}

	body, err := resp.BodyE()
	if err != nil {
		return nil, err
	}
	// todo: check 下 hertz 针对 gzip 解码的使用是否正确
	// GitHub #142 & #187
	if strings.EqualFold(resp.Header.Get(hdrContentEncodingKey), "gzip") && resp.Header.ContentLength() != 0 {
		body, err = resp.BodyGunzip()
		if err != nil {
			return nil, err
		}
	}

	response.bodyByte = body

	// todo: check下这里使用len()，而不是使用 content-length
	response.size = int64(len(response.bodyByte))

	// Apply Response middleware
	for _, f := range c.afterResponse {
		if err = f(c, response); err != nil {
			break
		}
	}

	return response, err
}

// get request
func (c *cli) r() *request {
	return &request{
		queryParam: url.Values{},
		header:     http.Header{},
		pathParams: map[string]string{},
		client:     c,
	}
}

type response struct {
	request     *request
	rawResponse *protocol.Response

	bodyByte []byte
	size     int64
}

// StatusCode method returns the HTTP status code for the executed request.
//	Example: 200
func (r *response) statusCode() int {
	if r.rawResponse == nil {
		return 0
	}

	return r.rawResponse.StatusCode()
}

// Body method returns HTTP response as []byte array for the executed request.
func (r *response) body() []byte {
	if r.rawResponse == nil {
		return []byte{}
	}
	return r.bodyByte
}

// Header method returns the response headers
func (r *response) header() http.Header {
	if r.rawResponse == nil {
		return http.Header{}
	}
	h := http.Header{}
	// todo: 是否写使用好性能的 s2b 写法
	r.rawResponse.Header.VisitAll(func(key, value []byte) {
		h.Add(string(key), string(value))
	})

	return h
}

type request struct {
	url            string
	method         string
	queryParam     url.Values
	header         http.Header
	requestOptions []config.RequestOption
	bodyParam      interface{}
	rawRequest     *protocol.Request
	ctx            context.Context
	pathParams     map[string]string
	client         *cli
	result         interface{}
	Error          interface{}
}

func (r *request) setContext(ctx context.Context) *request {
	r.ctx = ctx
	return r
}

func (r *request) context() context.Context {
	return r.ctx
}

func (r *request) setHeader(header, value string) *request {
	r.header.Set(header, value)
	return r
}

func (r *request) setQueryParam(param string, value interface{}) *request {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		for index := 0; index < v.Len(); index++ {
			r.queryParam.Add(param, fmt.Sprint(v.Index(index).Interface()))
		}
	default:
		r.queryParam.Set(param, fmt.Sprint(v))
	}
	return r
}

func (r *request) setResult(res interface{}) *request {
	r.result = res
	return r
}

func (r *request) setError(err interface{}) *request {
	r.Error = err
	return r
}

func (r *request) setHeaders(headers map[string]string) *request {
	for h, v := range headers {
		r.setHeader(h, v)
	}

	return r
}

func (r *request) setQueryParams(params map[string]interface{}) *request {
	for p, v := range params {
		r.setQueryParam(p, v)
	}

	return r
}

func (r *request) setPathParams(params map[string]string) *request {
	for p, v := range params {
		r.pathParams[p] = v
	}
	return r
}

func (r *request) setBodyParam(body interface{}) *request {
	r.bodyParam = body
	return r
}

func (r *request) setRequestOption(option ...config.RequestOption) *request {
	r.requestOptions = append(r.requestOptions, option...)
	return r
}

func (r *request) execute(method, url string) (*response, error) {
	r.method = method
	r.url = url
	return r.client.execute(r)
}

func parseRequestURL(c *cli, r *request) error {
	// GitHub #103 Path Params
	if len(r.pathParams) > 0 {
		for p, v := range r.pathParams {
			r.url = strings.Replace(r.url, ":"+p, url.PathEscape(v), -1)
		}
	}

	// Parsing request URL
	reqURL, err := url.Parse(r.url)
	if err != nil {
		return err
	}

	// If request.URL is relative path then added c.HostURL into
	// the request URL otherwise request.URL will be used as-is
	if !reqURL.IsAbs() {
		r.url = reqURL.String()
		if len(r.url) > 0 && r.url[0] != '/' {
			r.url = "/" + r.url
		}

		reqURL, err = url.Parse(c.hostUrl + r.url)
		if err != nil {
			return err
		}
	}

	// Adding Query Param
	query := make(url.Values)

	for k, v := range r.queryParam {
		// remove query param from client level by key
		// since overrides happens for that key in the request
		query.Del(k)
		for _, iv := range v {
			query.Add(k, iv)
		}
	}

	// GitHub #123 Preserve query string order partially.
	// Since not feasible in "SetQuery*" resty methods, because
	// standard package "url.Encode(...)" sorts the query params
	// alphabetically
	if len(query) > 0 {
		if isStringEmpty(reqURL.RawQuery) {
			reqURL.RawQuery = query.Encode()
		} else {
			reqURL.RawQuery = reqURL.RawQuery + "&" + query.Encode()
		}
	}

	r.url = reqURL.String()

	return nil
}

func isStringEmpty(str string) bool {
	return len(strings.TrimSpace(str)) == 0
}

func parseRequestHeader(c *cli, r *request) error {
	hdr := make(http.Header)
	if c.header != nil {
		for k := range c.header {
			hdr[k] = append(hdr[k], c.header[k]...)
		}
	}

	for k := range r.header {
		hdr.Del(k)
		hdr[k] = append(hdr[k], r.header[k]...)
	}

	r.header = hdr
	return nil
}

// detectContentType method is used to figure out "request.Body" content type for request header
func detectContentType(body interface{}) string {
	contentType := plainTextType
	kind := reflect.Indirect(reflect.ValueOf(body)).Kind()
	switch kind {
	case reflect.Struct, reflect.Map:
		contentType = jsonContentType
	case reflect.String:
		contentType = plainTextType
	default:
		if b, ok := body.([]byte); ok {
			contentType = http.DetectContentType(b)
		} else if kind == reflect.Slice {
			contentType = jsonContentType
		}
	}

	return contentType
}

func defaultRequestBodyBind(c *cli, r *request) (contentType string, body io.Reader, err error) {
	if !isPayloadSupported(r.method) {
		return
	}
	var bodyBytes []byte
	contentType = r.header.Get(hdrContentTypeKey)
	if isStringEmpty(contentType) {
		contentType = detectContentType(r.bodyParam)
		r.header.Set(hdrContentTypeKey, contentType)
	}
	kind := reflect.Indirect(reflect.ValueOf(r.bodyParam)).Kind()
	if isJSONType(contentType) &&
		(kind == reflect.Struct || kind == reflect.Map || kind == reflect.Slice) {
		bodyBytes, err = json.Marshal(r.bodyParam)
	} else if isXMLType(contentType) && (kind == reflect.Struct) {
		bodyBytes, err = xml.Marshal(r.bodyParam)
	}
	if err != nil {
		return
	}
	return contentType, strings.NewReader(string(bodyBytes)), nil
}

func isPayloadSupported(m string) bool {
	return !(m == http.MethodHead || m == http.MethodOptions || m == http.MethodGet || m == http.MethodDelete)
}

func createHTTPRequest(c *cli, r *request) (err error) {
	contentType, body, err := c.bindRequestBody(c, r)
	if !isStringEmpty(contentType) {
		r.header.Set(hdrContentTypeKey, contentType)
	}
	if err == nil {
		r.rawRequest = protocol.NewRequest(r.method, r.url, body)
		for key, values := range r.header {
			for _, val := range values {
				// todo: 检查下同名header多参数这么使用是否 ok
				r.rawRequest.Header.Add(key, val)
			}
		}
		r.rawRequest.SetOptions(r.requestOptions...)
	}
	return err
}

func silently(_ ...interface{}) {}

// defaultResponseResultDecider method returns true if HTTP status code >= 400 otherwise false.
func defaultResponseResultDecider(statusCode int, rawResponse *protocol.Response) bool {
	return statusCode > 399
}

// IsJSONType method is to check JSON content type or not
func isJSONType(ct string) bool {
	return jsonCheck.MatchString(ct)
}

// IsXMLType method is to check XML content type or not
func isXMLType(ct string) bool {
	return xmlCheck.MatchString(ct)
}

func parseResponseBody(c *cli, res *response) (err error) {
	if res.statusCode() == http.StatusNoContent {
		return
	}
	// Handles only JSON or XML content type
	ct := res.header().Get(hdrContentTypeKey)

	isError := c.responseResultDecider(res.statusCode(), res.rawResponse)
	if isError {
		if res.request.Error != nil {
			if isJSONType(ct) || isXMLType(ct) {
				err = unmarshalContent(ct, res.bodyByte, res.request.Error)
			}
		} else {
			jsonByte, jsonErr := json.Marshal(map[string]interface{}{
				"status_code": res.rawResponse.StatusCode,
				"body":        string(res.bodyByte),
			})
			if jsonErr != nil {
				return jsonErr
			}
			err = fmt.Errorf(string(jsonByte))
		}
		// todo: 反序列化
	} else if res.request.result != nil {
		if isJSONType(ct) || isXMLType(ct) {
			err = unmarshalContent(ct, res.bodyByte, res.request.result)
			return
		}
	}
	return
}

// unmarshalContent content into object from JSON or XML
func unmarshalContent(ct string, b []byte, d interface{}) (err error) {
	if isJSONType(ct) {
		err = json.Unmarshal(b, d)
	} else if isXMLType(ct) {
		err = xml.Unmarshal(b, d)
	}

	return
}

// todo: 优化这里，生成的时候直接生成指针指向的内容(*value)
// 目前为了快速调通接口设计
func DereferenceValue(v interface{}) interface{} {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		v = val.Elem()
	}
	return v
}
