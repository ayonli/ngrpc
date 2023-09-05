package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc"
	"github.com/ayonli/ngrpc/services"
	"github.com/ayonli/ngrpc/services/github/ayonli/ngrpc/services_proto"
	"github.com/gin-gonic/gin"
)

func main() {
	appName := ngrpc.GetAppName()
	app := goext.Ok(ngrpc.Start(appName))
	defer app.WaitForExit()

	var httpServer *http.Server
	var httpsServer *http.Server
	route := gin.Default()
	urlObj := goext.Ok(url.Parse(app.Uri))

	if urlObj.Scheme == "https" {
		port := urlObj.Port()

		if port == "" {
			port = "443"
		}

		httpsServer = &http.Server{
			Addr:    ":" + port,
			Handler: route,
		}
	} else {
		port := urlObj.Port()

		if port == "" {
			port = "80"
		}

		httpServer = &http.Server{
			Addr:    ":" + port,
			Handler: route,
		}
	}

	fmt.Println(httpServer, httpsServer)

	go func() {
		if httpsServer != nil {
			httpsServer.ListenAndServeTLS(app.Cert, app.Key)
		} else if httpServer != nil {
			httpServer.ListenAndServe()
		}
	}()

	app.OnStop(func() {
		if httpsServer != nil {
			goext.Ok(0, httpsServer.Shutdown(context.Background()))
		} else if httpServer != nil {
			goext.Ok(0, httpServer.Shutdown(context.Background()))
		}
	})

	route.GET("/user/:id", func(ctx *gin.Context) {
		userId := ctx.Param("id")
		userSrv := goext.Ok((&services.UserService{}).GetClient(userId))
		user, err := userSrv.GetUser(ctx, &services_proto.UserQuery{Id: &userId})

		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				ctx.PureJSON(200, gin.H{"code": 404, "error": err.Error()})
			} else {
				ctx.PureJSON(200, gin.H{"code": 500, "error": err.Error()})
			}
		} else {
			ctx.PureJSON(200, gin.H{"code": 0, "data": user})
		}
	}).GET("/users/gender/:gender", func(ctx *gin.Context) {
		var gender services_proto.Gender

		if ctx.Param("gender") == "unknown" {
			gender = services_proto.Gender_UNKNOWN
		} else if ctx.Param("gender") == "male" {
			gender = services_proto.Gender_MALE
		} else if ctx.Param("gender") == "female" {
			gender = services_proto.Gender_FEMALE
		} else {
			ctx.PureJSON(200, gin.H{
				"code":  400,
				"error": "unrecognized gender argument, shall be either 'male', 'female' or 'unknown'",
			})
			return
		}

		userSrv := goext.Ok((&services.UserService{}).GetClient(ctx.Param("gender")))
		result, err := userSrv.GetUsers(ctx, &services_proto.UsersQuery{Gender: &gender})

		if err != nil {
			ctx.PureJSON(200, gin.H{"code": 500, "error": err.Error()})
		} else {
			ctx.PureJSON(200, gin.H{"code": 0, "data": result.Users})
		}
	}).GET("/users/age/:min/to/:max", func(ctx *gin.Context) {
		minAge, err1 := strconv.Atoi(ctx.Param("min"))
		maxAge, err2 := strconv.Atoi(ctx.Param("max"))

		if err1 != nil || err2 != nil {
			ctx.PureJSON(200, gin.H{"code": 400, "error": "unrecognized age range"})
			return
		}

		_minAge := int32(minAge)
		_maxAge := int32(maxAge)
		userSrv := goext.Ok((&services.UserService{}).GetClient(""))
		result, err := userSrv.GetUsers(ctx, &services_proto.UsersQuery{
			MinAge: &_minAge,
			MaxAge: &_maxAge,
		})

		if err != nil {
			ctx.PureJSON(200, gin.H{"code": 500, "error": err.Error()})
		} else {
			ctx.PureJSON(200, gin.H{"code": 0, "data": result.Users})
		}
	}).GET("/user/:id/posts", func(ctx *gin.Context) {
		userId := ctx.Param("id")
		userSrv := goext.Ok((&services.UserService{}).GetClient(userId))
		result, err := userSrv.GetMyPosts(ctx, &services_proto.UserQuery{Id: &userId})

		if err != nil {
			ctx.PureJSON(200, gin.H{"code": 500, "error": err.Error()})
		} else {
			ctx.PureJSON(200, gin.H{"code": 0, "data": result.Posts})
		}
	}).GET("/post/:id", func(ctx *gin.Context) {
		idStr := ctx.Param("id")
		id, err := strconv.Atoi(idStr)

		if err != nil {
			ctx.PureJSON(200, gin.H{"code": 500, "error": err.Error()})
			return
		}

		postSrv := goext.Ok((&services.PostService{}).GetClient(idStr))
		post, err := postSrv.GetPost(ctx, &services_proto.PostQuery{Id: int32(id)})

		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				ctx.PureJSON(200, gin.H{"code": 404, "error": err.Error()})
			} else {
				ctx.PureJSON(200, gin.H{"code": 500, "error": err.Error()})
			}
		} else {
			ctx.PureJSON(200, gin.H{"code": 0, "data": post})
		}
	}).GET("/posts/search/:keyword", func(ctx *gin.Context) {
		keyword := ctx.Param("keyword")
		postSrv := goext.Ok((&services.PostService{}).GetClient(keyword))
		result, err := postSrv.SearchPosts(ctx, &services_proto.PostsQuery{
			Keyword: &keyword,
		})

		if err != nil {
			ctx.PureJSON(200, gin.H{"code": 500, "error": err.Error()})
		} else {
			ctx.PureJSON(200, gin.H{"code": 0, "data": result.Posts})
		}
	})
}
