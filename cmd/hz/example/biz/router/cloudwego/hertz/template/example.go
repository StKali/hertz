// Code generated by hertz generator. DO NOT EDIT.

package Template

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	template "github.com/cloudwego/hz/client/biz/handler/cloudwego/hertz/template"
)

/*
 This file will register all the routes of the services in the master idl.
 And it will update automatically when you use the "update" command for the idl.
 So don't modify the contents of the file, or your code will be deleted when it is updated.
*/

// Register register routes based on the IDL 'api.${HTTP Method}' annotation.
func Register(r *server.Hertz) {

	root := r.Group("/", rootMw()...)
	{
		_life := root.Group("/life", _lifeMw()...)
		_life.POST("/client1", append(_bizmethod2Mw(), template.BizMethod2)...)
		_life.POST("/client2", append(_bizmethod3Mw(), template.BizMethod3)...)
		_life.POST("/client3", append(_bizmethod4Mw(), template.BizMethod4)...)
	}
}
