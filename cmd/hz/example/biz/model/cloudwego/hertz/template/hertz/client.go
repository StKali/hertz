package hertz

import (
	"context"
	"fmt"

	hertz_client "github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/protocol"
	template "github.com/cloudwego/hz/client/biz/model/cloudwego/hertz/template"
)

type Client interface {
	BizMethod2(context context.Context, req *template.Req, reqOpt ...config.RequestOption) (resp *template.Resp, rawResponse *protocol.Response, err error)
	BizMethod3(context context.Context, req *template.Req, reqOpt ...config.RequestOption) (resp *template.Resp, rawResponse *protocol.Response, err error)
	BizMethod4(context context.Context, req *template.Req, reqOpt ...config.RequestOption) (resp *template.Resp, rawResponse *protocol.Response, err error)
}

type idlClient struct {
	client *cli
}

func NewClient(hostUrl string, ops ...Option) (Client, error) {
	opts := getOptions(append(ops, withHostUrl(hostUrl))...)
	cli, err := newClient(opts)
	if err != nil {
		return nil, err
	}
	return &idlClient{
		client: cli,
	}, nil
}

func (s *idlClient) BizMethod2(context context.Context, req *template.Req, reqOpt ...config.RequestOption) (resp *template.Resp, rawResponse *protocol.Response, err error) {
	httpResp := &template.Resp{}
	ret, err := s.client.r().
		setContext(context).
		setQueryParams(map[string]interface{}{
			"q1": DereferenceValue(req.QueryString),
			"q2": DereferenceValue(req.MixString),
		}).
		setPathParams(map[string]string{
			"p1": fmt.Sprint(DereferenceValue(req.QueryString)),
			"p2": fmt.Sprint(DereferenceValue(req.MixString)),
		}).
		setBodyParam(req).
		setHeaders(map[string]string{
			"h1": fmt.Sprint(DereferenceValue(req.HeaderString)),
			"h2": fmt.Sprint(DereferenceValue(req.MixString)),
		}).
		setRequestOption(reqOpt...).
		setResult(httpResp).
		execute("POST", "/life/client1")
	if err == nil {
		resp = httpResp
	}
	rawResponse = ret.rawResponse
	return resp, rawResponse, err
}

func (s *idlClient) BizMethod3(context context.Context, req *template.Req, reqOpt ...config.RequestOption) (resp *template.Resp, rawResponse *protocol.Response, err error) {
	httpResp := &template.Resp{}
	ret, err := s.client.r().
		setContext(context).
		setQueryParams(map[string]interface{}{
			"q1": req.QueryString,
			"q2": req.MixString,
		}).
		setPathParams(map[string]string{
			"p1": fmt.Sprint(req.PathString),
			"p2": fmt.Sprint(req.MixString),
		}).
		setBodyParam(req).
		setHeaders(map[string]string{
			"h1": fmt.Sprint(req.HeaderString),
			"h2": fmt.Sprint(req.MixString),
		}).
		setRequestOption(reqOpt...).
		setResult(httpResp).
		execute("POST", "/life/client2")
	if err == nil {
		resp = httpResp
	}
	rawResponse = ret.rawResponse
	return resp, rawResponse, err
}

func (s *idlClient) BizMethod4(context context.Context, req *template.Req, reqOpt ...config.RequestOption) (resp *template.Resp, rawResponse *protocol.Response, err error){
	httpResp := &template.Resp{}
	ret, err := s.client.r().
		setContext(context).
		setQueryParams(map[string]interface{}{
			"q1": DereferenceValue(req.QueryString),
			"q2": req.MixString,
		}).
		setPathParams(map[string]string{
			"p1": fmt.Sprint(req.PathString),
			"p2": fmt.Sprint(req.MixString),
		}).
		setBodyParam(req).
		setHeaders(map[string]string{
			"h1": fmt.Sprint(req.HeaderString),
			"h2": fmt.Sprint(req.MixString),
		}).
		setRequestOption(reqOpt...).
		setResult(httpResp).
		execute("POST", "/life/client3")
	if err == nil {
		resp = httpResp
	}
	rawResponse = ret.rawResponse
	return resp, rawResponse, err
}

// 这里的 domain 可以通过在idl里定义注解来填充
var dafaultClient, _ = NewClient("http://127.0.0.1:8899", WithHertzClientOption(hertz_client.WithKeepAlive(true)))

func BizMethod2(context context.Context, req *template.Req, reqOpt ...config.RequestOption) (resp *template.Resp, rawResponse *protocol.Response, err error) {
	return dafaultClient.BizMethod2(context, req, reqOpt...)
}

func BizMethod3(context context.Context, req *template.Req, reqOpt ...config.RequestOption) (resp *template.Resp, rawResponse *protocol.Response, err error) {
	return dafaultClient.BizMethod3(context, req, reqOpt...)
}

func BizMethod4(context context.Context, req *template.Req, reqOpt ...config.RequestOption) (resp *template.Resp, rawResponse *protocol.Response, err error) {
	return dafaultClient.BizMethod4(context, req, reqOpt...)
}

