package main

import (
	"errors"
	"fmt"
	limits "github.com/gin-contrib/size"
	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"github.com/minecrafthopper/pasteforhelp/static"
	"github.com/minecrafthopper/pasteforhelp/webtemplates"
	"github.com/spf13/cast"
	"html/template"
	"io"
	"net/http"
	"os"
)

var storageDirPath, _ = os.LookupEnv("STORAGE_DIR")
var storageSystem FileServer
var maxFileSizeEnv, _ = os.LookupEnv("MAX_FILE_SIZE")
var maxFileSize int64 = 1024 /*b*/ * 1024 /*kb*/ * 100 /*mb*/

func main() {
	if i := cast.ToInt64(maxFileSizeEnv); i > 0 {
		maxFileSize = i
	}

	var err error
	storageSystem, err = NewFileServer(storageDirPath)
	if err != nil {
		panic(err)
	}

	defer Close(storageSystem)

	r := gin.Default()
	r.Use(limits.RequestSizeLimiter(maxFileSize))

	r.StaticFS("/static", http.FS(static.AssetFiles))

	indexTemplate := template.Must(template.ParseFS(webtemplates.WebFiles, "base.html", "index.html"))
	uploadTemplate := template.Must(template.ParseFS(webtemplates.WebFiles, "base.html", "upload.html"))

	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
		err := indexTemplate.ExecuteTemplate(c.Writer, "index.html", gin.H{})
		if err != nil {
			fmt.Printf("%s\n", err)
		}
	})

	r.StaticFileFS("/favicon.ico", "favicon.ico", http.FS(static.AssetFiles))

	r.GET("/upload", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/")
	})

	r.GET("/upload/:id", func(c *gin.Context) {
		id := c.Param("id")

		file, err := storageSystem.OpenFile(id, os.O_RDONLY, 0644)
		if err != nil {
			if os.IsNotExist(err) {
				//c.AbortWithStatus(http.StatusNotFound)
				c.Redirect(http.StatusTemporaryRedirect, "/")
			} else {
				fmt.Printf("%s\n", err)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
			return
		}

		defer Close(file)

		reader := io.LimitReader(file, maxFileSize)

		data, err := io.ReadAll(reader)
		if err != nil {
			fmt.Printf("%s\n", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Status(http.StatusOK)
		err = uploadTemplate.ExecuteTemplate(c.Writer, "upload.html", map[string]interface{}{
			"id":      id,
			"content": string(data),
		})
		if err != nil {
			fmt.Printf("%s\n", err)
		}
	})

	r.POST("/upload", func(c *gin.Context) {
		if len(c.Errors) > 0 {
			return
		}

		id := RandomString()

		var input = c.PostForm("content")
		file, err := storageSystem.OpenFile(id, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			if os.IsNotExist(err) {
				c.Redirect(http.StatusTemporaryRedirect, "/")
				return
			}
			fmt.Printf("%s\n", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer Close(file)
		_, err = file.WriteString(input)
		if err != nil {
			fmt.Printf("%s\n", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Redirect(http.StatusFound, "/upload/"+id)
	})

	r.GET("/raw/:id", func(c *gin.Context) {
		id := c.Param("id")

		file, err := storageSystem.OpenFile(id, os.O_RDONLY, 0644)
		if err != nil {
			if os.IsNotExist(err) {
				c.AbortWithStatus(http.StatusNotFound)
			} else {
				fmt.Printf("%s\n", err)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
			return
		}

		defer Close(file)

		reader := io.LimitReader(file, maxFileSize)

		data, err := io.ReadAll(reader)
		if err != nil {
			fmt.Printf("%s\n", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Status(http.StatusOK)
		c.Data(200, "text/plain; charset=utf-8", data)
	})

	err = r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func Close(c io.Closer) {
	if c != nil {
		_ = c.Close()
	}
}
func RandomString() string {
	gen, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}
	return gen.String()[:8]
}
